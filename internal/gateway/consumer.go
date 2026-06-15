package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

const usageDumpInterval = 30 * time.Second

const defaultUsagePath = "/var/lib/hivemind/usage.json"

// ConsumerStats tracks usage metrics for a single consumer.
type ConsumerStats struct {
	RequestCount     int64     `json:"request_count"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	LastRequest      time.Time `json:"last_request"`
}

// ConsumerTracker identifies consumers and accumulates per-consumer usage.
type ConsumerTracker struct {
	mu       sync.Mutex
	stats    map[string]*ConsumerStats
	apiKeys  map[string]string // api key → consumer name
	savePath string
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewConsumerTracker creates a tracker backed by the given config.
func NewConsumerTracker(cfg *config.Config, savePath string) *ConsumerTracker {
	return &ConsumerTracker{
		stats:    make(map[string]*ConsumerStats),
		apiKeys:  cfg.Consumers.APIKeys,
		savePath: savePath,
		stopCh:   make(chan struct{}),
	}
}

// Identify extracts the consumer name from the request.
// Priority: X-HiveMind-Consumer header → API key lookup (Authorization Bearer) → "__default__".
func (ct *ConsumerTracker) Identify(r *http.Request) string {
	if v := r.Header.Get("X-HiveMind-Consumer"); v != "" {
		return v
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		key := strings.TrimPrefix(auth, "Bearer ")
		if name, ok := ct.apiKeys[key]; ok {
			return name
		}
	}
	return "__default__"
}

// Record increments usage stats for the given consumer.
func (ct *ConsumerTracker) Record(consumer string, prompt, completion int64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	s, ok := ct.stats[consumer]
	if !ok {
		s = &ConsumerStats{}
		ct.stats[consumer] = s
	}
	s.RequestCount++
	s.PromptTokens += prompt
	s.CompletionTokens += completion
	s.LastRequest = time.Now()
}

// Middleware wraps next, identifies the consumer, and records usage after each response.
func (ct *ConsumerTracker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		consumer := ct.Identify(r)
		crw := &capturingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(crw, r)
		prompt, completion := parseTokenUsage(crw.buf.Bytes())
		ct.Record(consumer, prompt, completion)
	})
}

// HandleUsage serves GET /admin/usage — returns a JSON snapshot of all consumer stats.
func (ct *ConsumerTracker) HandleUsage(w http.ResponseWriter, r *http.Request) {
	ct.mu.Lock()
	snapshot := make(map[string]ConsumerStats, len(ct.stats))
	for k, v := range ct.stats {
		snapshot[k] = *v
	}
	ct.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot) //nolint:errcheck
}

// Start launches the background persistence goroutine.
func (ct *ConsumerTracker) Start() {
	ct.wg.Add(1)
	go func() {
		defer ct.wg.Done()
		ticker := time.NewTicker(usageDumpInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ct.persist()
			case <-ct.stopCh:
				ct.persist() // final flush before exit
				return
			}
		}
	}()
}

// Stop signals the persistence goroutine to flush and exit.
func (ct *ConsumerTracker) Stop() {
	close(ct.stopCh)
	ct.wg.Wait()
}

func (ct *ConsumerTracker) persist() {
	ct.mu.Lock()
	snapshot := make(map[string]ConsumerStats, len(ct.stats))
	for k, v := range ct.stats {
		snapshot[k] = *v
	}
	ct.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(ct.savePath), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	// Atomic write via temp file + rename.
	tmp := ct.savePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, ct.savePath) //nolint:errcheck
}

// openAIUsage mirrors the usage block in an OpenAI-compatible response.
type openAIUsage struct {
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func parseTokenUsage(body []byte) (prompt, completion int64) {
	if len(body) == 0 {
		return 0, 0
	}
	var resp openAIUsage
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}

// capturingResponseWriter buffers the response body for non-streaming responses
// so that token counts can be extracted from the OpenAI usage field.
type capturingResponseWriter struct {
	http.ResponseWriter
	buf         bytes.Buffer
	isStreaming bool
}

func (c *capturingResponseWriter) Write(b []byte) (int, error) {
	if !c.isStreaming {
		if strings.Contains(c.ResponseWriter.Header().Get("Content-Type"), "text/event-stream") {
			c.isStreaming = true
		} else {
			c.buf.Write(b)
		}
	}
	return c.ResponseWriter.Write(b)
}
