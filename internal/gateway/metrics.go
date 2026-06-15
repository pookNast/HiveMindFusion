package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

// statusRecorder captures the HTTP status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
