package gateway

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus instruments for the hivemind gateway.
type Metrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	tokensTotal     *prometheus.CounterVec
	vramUsageBytes  *prometheus.GaugeVec
	backendHealth   *prometheus.GaugeVec
}

// NewMetrics registers all instruments with the given registerer.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hivemind_requests_total",
				Help: "Total number of requests processed by the hivemind gateway.",
			},
			[]string{"consumer", "model", "backend", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hivemind_request_duration_seconds",
				Help:    "Histogram of request latencies in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"consumer", "model", "backend"},
		),
		tokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hivemind_tokens_total",
				Help: "Total number of tokens processed (prompt and completion).",
			},
			[]string{"consumer", "model", "backend", "type"},
		),
		vramUsageBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hivemind_vram_usage_bytes",
				Help: "Current VRAM usage in bytes per backend.",
			},
			[]string{"backend"},
		),
		backendHealth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hivemind_backend_health",
				Help: "Backend health status (1 = healthy, 0 = unhealthy).",
			},
			[]string{"backend"},
		),
	}

	reg.MustRegister(
		m.requestsTotal,
		m.requestDuration,
		m.tokensTotal,
		m.vramUsageBytes,
		m.backendHealth,
	)

	return m
}

// ObserveRequest records duration and increments the request counter.
func (m *Metrics) ObserveRequest(consumer, model, backend string, status int, duration time.Duration) {
	labels := prometheus.Labels{
		"consumer": consumer,
		"model":    model,
		"backend":  backend,
		"status":   strconv.Itoa(status),
	}
	m.requestsTotal.With(labels).Inc()
	m.requestDuration.With(prometheus.Labels{
		"consumer": consumer,
		"model":    model,
		"backend":  backend,
	}).Observe(duration.Seconds())
}

// ObserveTokens records prompt and completion token counts.
func (m *Metrics) ObserveTokens(consumer, model, backend string, prompt, completion int) {
	base := prometheus.Labels{"consumer": consumer, "model": model, "backend": backend}

	pt := prometheus.Labels{"consumer": base["consumer"], "model": base["model"], "backend": base["backend"], "type": "prompt"}
	m.tokensTotal.With(pt).Add(float64(prompt))

	ct := prometheus.Labels{"consumer": base["consumer"], "model": base["model"], "backend": base["backend"], "type": "completion"}
	m.tokensTotal.With(ct).Add(float64(completion))
}

// SetVRAMUsage updates the VRAM gauge for a backend.
func (m *Metrics) SetVRAMUsage(backend string, bytes float64) {
	m.vramUsageBytes.With(prometheus.Labels{"backend": backend}).Set(bytes)
}

// SetBackendHealth updates the health gauge for a backend (1=up, 0=down).
func (m *Metrics) SetBackendHealth(backend string, healthy bool) {
	v := 0.0
	if healthy {
		v = 1.0
	}
	m.backendHealth.With(prometheus.Labels{"backend": backend}).Set(v)
}

// Middleware wraps an http.Handler and records request metrics.
// consumer and backend are extracted from request context or defaulted.
func (m *Metrics) Middleware(next http.Handler, consumer, model, backend string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		m.ObserveRequest(consumer, model, backend, rw.status, time.Since(start))
	})
}

// statusRecorder captures the HTTP status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// ServeMetrics starts an HTTP server on metricsPort exposing /metrics.
func (m *Metrics) ServeMetrics(reg prometheus.Gatherer, metricsPort int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), mux)
}
