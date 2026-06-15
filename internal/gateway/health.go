package gateway

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pook/hivemind/internal/config"
)

// BackendState represents the health state of a backend.
type BackendState int

const (
	StateUnknown   BackendState = iota
	StateHealthy
	StateUnhealthy
)

func (s BackendState) String() string {
	switch s {
	case StateHealthy:
		return "healthy"
	case StateUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

type backendHealth struct {
	state     BackendState
	failures  int
	lastCheck time.Time
}

// VRAMInfo holds GPU memory usage reported by nvidia-smi.
type VRAMInfo struct {
	UsedMiB  int `json:"used_mib"`
	TotalMiB int `json:"total_mib"`
}

// BackendStatus is the JSON shape for a single backend in /health responses.
type BackendStatus struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	LastCheck string `json:"last_check,omitempty"`
}

// HealthResponse is the JSON body returned by GET /health on the admin port.
type HealthResponse struct {
	Status   string          `json:"status"`
	Backends []BackendStatus `json:"backends"`
	VRAM     *VRAMInfo       `json:"vram,omitempty"`
}

// HealthChecker runs periodic HTTP health checks against each configured backend.
type HealthChecker struct {
	cfg       *config.Config
	mu        sync.RWMutex
	states    map[string]*backendHealth
	interval  time.Duration
	threshold int
	client    *http.Client
	metrics   *Metrics
}

// SetMetrics wires the Prometheus metrics so health transitions update gauges.
func (hc *HealthChecker) SetMetrics(m *Metrics) { hc.metrics = m }

// NewHealthChecker creates a HealthChecker.  Defaults: 30 s interval, 3 failures → unhealthy.
func NewHealthChecker(cfg *config.Config) *HealthChecker {
	interval := time.Duration(cfg.Gateway.HealthIntervalSecs) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}
	threshold := cfg.Gateway.HealthFailureThreshold
	if threshold == 0 {
		threshold = 3
	}

	states := make(map[string]*backendHealth, len(cfg.Backends))
	for _, b := range cfg.Backends {
		states[b.Name] = &backendHealth{state: StateUnknown}
	}

	return &HealthChecker{
		cfg:       cfg,
		states:    states,
		interval:  interval,
		threshold: threshold,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

// UpdateConfig swaps in a new config on reload: it points checks at the new
// backend set, adds StateUnknown entries for newly-added backends, and prunes
// states for removed ones. The check interval and failure threshold are fixed
// at construction and are not changed here.
func (hc *HealthChecker) UpdateConfig(cfg *config.Config) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.cfg = cfg
	seen := make(map[string]bool, len(cfg.Backends))
	for _, b := range cfg.Backends {
		seen[b.Name] = true
		if _, ok := hc.states[b.Name]; !ok {
			hc.states[b.Name] = &backendHealth{state: StateUnknown}
		}
	}
	for name := range hc.states {
		if !seen[name] {
			delete(hc.states, name)
		}
	}
}

// Start launches the background health-check loop.
func (hc *HealthChecker) Start() {
	go hc.loop()
}

func (hc *HealthChecker) loop() {
	hc.checkAll() // immediate first pass
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	for range ticker.C {
		hc.checkAll()
	}
}

func (hc *HealthChecker) checkAll() {
	hc.mu.RLock()
	backends := hc.cfg.Backends // snapshot slice header; reload swaps the whole cfg
	hc.mu.RUnlock()
	for _, b := range backends {
		go hc.checkOne(b)
	}
}

func (hc *HealthChecker) checkOne(b config.Backend) {
	endpoint := b.HealthEndpoint
	if endpoint == "" {
		endpoint = "/health"
	}
	// If endpoint is a relative path, prepend the backend URL
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = strings.TrimRight(b.URL, "/") + endpoint
	}

	resp, err := hc.client.Get(endpoint)
	healthy := err == nil && resp.StatusCode < 500
	if resp != nil {
		resp.Body.Close()
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()
	s := hc.states[b.Name]
	if s == nil {
		return // backend removed by a concurrent reload
	}
	s.lastCheck = time.Now()
	if healthy {
		if s.state != StateHealthy {
			log.Printf("[health] backend %q recovered (was %s, failures=%d)", b.Name, s.state, s.failures)
		}
		s.failures = 0
		s.state = StateHealthy
		if hc.metrics != nil {
			hc.metrics.SetBackendHealth(b.Name, true)
		}
	} else {
		s.failures++
		if err != nil {
			log.Printf("[health] backend %q probe failed: %v (failures=%d/%d)", b.Name, err, s.failures, hc.threshold)
		} else {
			log.Printf("[health] backend %q probe status=%d (failures=%d/%d)", b.Name, resp.StatusCode, s.failures, hc.threshold)
		}
		if s.failures >= hc.threshold {
			s.state = StateUnhealthy
			if hc.metrics != nil {
				hc.metrics.SetBackendHealth(b.Name, false)
			}
		}
	}
}

// IsHealthy returns false only for backends that have been confirmed unhealthy.
// Unknown (not yet checked) backends are treated as available.
func (hc *HealthChecker) IsHealthy(name string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	s, ok := hc.states[name]
	if !ok {
		return false
	}
	return s.state != StateUnhealthy
}

// ServeHTTP handles GET /health on the admin port.
func (hc *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hc.mu.RLock()
	statuses := make([]BackendStatus, 0, len(hc.cfg.Backends))
	allHealthy := true
	for _, b := range hc.cfg.Backends {
		s := hc.states[b.Name]
		last := ""
		if !s.lastCheck.IsZero() {
			last = s.lastCheck.Format(time.RFC3339)
		}
		if s.state == StateUnhealthy && !b.ExpectedDown {
			allHealthy = false
		}
		statuses = append(statuses, BackendStatus{
			Name:      b.Name,
			State:     s.state.String(),
			LastCheck: last,
		})
	}
	hc.mu.RUnlock()

	status := "ok"
	if !allHealthy {
		status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:   status,
		Backends: statuses,
		VRAM:     queryVRAM(),
	})
}

// queryVRAM reads GPU memory via nvidia-smi; returns nil if unavailable.
func queryVRAM() *VRAMInfo {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=memory.used,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	line := strings.TrimSpace(string(out))
	line = strings.SplitN(line, "\n", 2)[0] // first GPU only
	parts := strings.Split(line, ", ")
	if len(parts) != 2 {
		return nil
	}
	used, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	total, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return nil
	}
	return &VRAMInfo{UsedMiB: used, TotalMiB: total}
}
