package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/pooknast/HiveMindFusion/internal/config"
	"github.com/pooknast/HiveMindFusion/internal/pii"
	"github.com/pooknast/HiveMindFusion/internal/rag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Gateway struct {
	mu       sync.Mutex
	cfg      *config.Config
	metrics  *Metrics
	reg      *prometheus.Registry
	health   *HealthChecker
	consumer *ConsumerTracker

	// liveHandler holds the current proxy request chain (http.HandlerFunc).
	// Run() seeds it; Reload() atomically swaps a freshly-built chain in so
	// config changes apply without restarting the listener.
	liveHandler atomic.Value

	// currentProxy holds the current proxy for admin endpoint access (e.g. /panels).
	currentProxy atomic.Value

	// stored for Shutdown
	proxyServer   *http.Server
	adminServer   *http.Server
	metricsServer *http.Server
}

func New(cfg *config.Config) *Gateway {
	reg := prometheus.NewRegistry()
	return &Gateway{
		cfg:      cfg,
		metrics:  NewMetrics(reg),
		reg:      reg,
		health:   NewHealthChecker(cfg),
		consumer: NewConsumerTracker(cfg, defaultUsagePath),
	}
}

// buildProxyHandler constructs the full request chain (PII → consumer → routes →
// proxy) from cfg. Called once by Run() and again by every Reload() so backends,
// the /v1/models list, RAG handlers, compression, and PII settings all reflect the
// current config. Returned as http.HandlerFunc so atomic.Value always stores one
// concrete type (interface dynamic-type changes — e.g. PII toggling — would panic
// the atomic store otherwise).
func (g *Gateway) buildProxyHandler(cfg *config.Config) http.HandlerFunc {
	p := newProxy(cfg, g.health, g.metrics, g.consumer.Identify)
	g.currentProxy.Store(p)

	proxyMux := http.NewServeMux()
	g.setupRoutes(proxyMux, p)

	var handler http.Handler = g.consumer.Middleware(proxyMux)

	// Rate limiter: bind to authenticated identity (AuthMiddleware sets the
	// context value the limiter reads). Sits inside auth so unauthenticated
	// requests are rejected before bucket accounting touches them.
	rl := NewRateLimiter(cfg.RateLimit)
	handler = rl.Middleware(handler)

	if cfg.PII.Enabled {
		piiClient := pii.NewClient(cfg.PII.Endpoint, cfg.PII.TimeoutMs, cfg.PII.BypassOnFailure)
		handler = pii.Middleware(piiClient, handler)
	}

	// Auth gate: outermost wrapper so unauthenticated requests are rejected
	// BEFORE PII scanning or consumer tracking touch the request body.
	apiKeys, knownConsumers := g.buildKnownConsumers(cfg)
	handler = AuthMiddleware(apiKeys, knownConsumers, handler)

	return handler.ServeHTTP
}

// buildKnownConsumers returns the set of valid consumer names for auth checks.
// Includes consumers from api_keys values and rate_limit consumer entries.
func (g *Gateway) buildKnownConsumers(cfg *config.Config) (map[string]string, map[string]struct{}) {
	consumers := make(map[string]struct{})
	for _, name := range cfg.Consumers.APIKeys {
		consumers[name] = struct{}{}
	}
	for name := range cfg.RateLimit.Consumers {
		consumers[name] = struct{}{}
	}
	consumers["__default__"] = struct{}{}
	return cfg.Consumers.APIKeys, consumers
}

// Reload applies a new config to the live gateway without a restart: it rebuilds
// the proxy request chain (backends, /v1/models, RAG, compression, PII) and swaps
// it in atomically, and refreshes the health checker's backend set.
//
// Not yet hot-reloaded (still need a restart): listener ports, consumer rate-limit
// tables, and the health-check interval/threshold — these are bound at startup.
func (g *Gateway) Reload(cfg *config.Config) {
	g.mu.Lock()
	g.cfg = cfg
	g.mu.Unlock()

	g.health.UpdateConfig(cfg)
	g.liveHandler.Store(g.buildProxyHandler(cfg))
	fusionTiers := 0
	if cfg.Fusion.Enabled {
		fusionTiers = len(cfg.Fusion.Panels)
	}
	log.Printf("[hivemind] reload applied: %d backends, PII=%v, fusion=%v (%d tiers)",
		len(cfg.Backends), cfg.PII.Enabled, cfg.Fusion.Enabled, fusionTiers)
}

// Run starts all three servers concurrently. Returns the first non-close error.
func (g *Gateway) Run() error {
	g.mu.Lock()
	cfg := g.cfg
	g.mu.Unlock()

	g.health.SetMetrics(g.metrics)
	g.health.Start()
	g.consumer.Start()

	adminMux := http.NewServeMux()
	adminMux.Handle("/health", g.health)
	adminMux.Handle("/admin/ingest", rag.NewIngester(cfg))
	adminMux.HandleFunc("/admin/usage", g.consumer.HandleUsage)
	adminMux.HandleFunc("/panels", g.handlePanels)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(g.reg, promhttp.HandlerOpts{}))

	// Seed the swappable request chain; Reload() replaces it in place.
	g.liveHandler.Store(g.buildProxyHandler(cfg))
	// Stable indirection: each request dispatches to whatever chain is current,
	// so SIGHUP reloads take effect without rebinding the listener.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.liveHandler.Load().(http.HandlerFunc)(w, r)
	})

	g.mu.Lock()
	g.proxyServer = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", cfg.Gateway.Port), Handler: handler}
	g.adminServer = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", cfg.Gateway.AdminPort), Handler: adminMux}
	g.metricsServer = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", cfg.Gateway.MetricsPort), Handler: metricsMux}
	proxy, admin, metrics := g.proxyServer, g.adminServer, g.metricsServer
	g.mu.Unlock()

	errCh := make(chan error, 3)
	for _, srv := range []*http.Server{admin, metrics} {
		s := srv
		go func() {
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("server %s: %w", s.Addr, err)
			}
		}()
	}
	go func() {
		if err := proxy.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server %s: %w", proxy.Addr, err)
		}
	}()

	return <-errCh
}

// Shutdown gracefully stops all servers within the deadline.
func (g *Gateway) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	proxy, admin, metrics := g.proxyServer, g.adminServer, g.metricsServer
	g.mu.Unlock()

	var wg sync.WaitGroup
	errCh := make(chan error, 3)
	for _, srv := range []*http.Server{proxy, admin, metrics} {
		if srv == nil {
			continue
		}
		s := srv
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Shutdown(ctx); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}
	g.consumer.Stop()
	return nil
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
}

// handlePanels serves GET /panels — returns fusion tier configurations.
func (g *Gateway) handlePanels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	proxy, ok := g.currentProxy.Load().(*Proxy)
	if !ok || proxy.fusionEngine == nil {
		json.NewEncoder(w).Encode(map[string]any{"panels": map[string]any{}, "enabled": false})
		return
	}
	panels := proxy.fusionEngine.Panels()
	json.NewEncoder(w).Encode(map[string]any{"panels": panels, "enabled": true})
}
