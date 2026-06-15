package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/pook/hivemind/internal/config"
	"github.com/pook/hivemind/internal/rag"
)

// Proxy routes OpenAI-compatible requests to configured backends with fallback support.
type Proxy struct {
	backends     map[string][]*config.Backend // model name → ordered fallback chain
	health       *HealthChecker
	metrics      *Metrics
	transport    *http.Transport
	ragHandlers  map[string]*rag.Middleware // consumer name → RAG middleware (nil if disabled)
	consumerFunc func(*http.Request) string // injected: resolves consumer from request
	compressor   *Compressor                // headroom-srv compression sidecar (nil if disabled)
}

func newProxy(cfg *config.Config, hc *HealthChecker, metrics *Metrics, consumerFunc func(*http.Request) string) *Proxy {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		// ponytail: 180s to absorb fusion pipeline (fan-out+judge+synth can exceed 120s) — upgrade: per-backend timeout config
		ResponseHeaderTimeout: 180 * time.Second,
	}

	// Build per-consumer RAG handlers
	ragHandlers := make(map[string]*rag.Middleware)
	for name, rc := range cfg.RAG.Consumers {
		if rc.Enabled {
			ragHandlers[name] = rag.NewMiddleware(cfg.Qdrant.Endpoint, rag.ConsumerConfig{
				Enabled:       true,
				Collection:    rc.Collection,
				TopK:          rc.TopK,
				MinScore:      rc.MinScore,
				EmbedEndpoint: cfg.Embed.Endpoint,
				EmbedModel:    cfg.Embed.Model,
			})
		}
	}

	return &Proxy{
		backends:     buildFallbackChain(cfg.Backends),
		health:       hc,
		metrics:      metrics,
		transport:    transport,
		ragHandlers:  ragHandlers,
		consumerFunc: consumerFunc,
		compressor:   NewCompressor(cfg.Compression),
	}
}

type modelField struct {
	Model string `json:"model"`
}

func (p *Proxy) modelNames() []string {
	names := make([]string, 0, len(p.backends))
	for name := range p.backends {
		names = append(names, name)
	}
	return names
}

// routeRequest handles /v1/chat/completions and /v1/completions.
// It reads the model field from the JSON body, then walks the fallback chain.
func (p *Proxy) routeRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req modelField
	if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
		writeJSONError(w, http.StatusBadRequest, "model field required")
		return
	}

	chain, ok := p.backends[req.Model]
	if !ok {
		available, _ := json.Marshal(p.modelNames())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"unknown model","model":%q,"available_models":%s}`, req.Model, available)
		return
	}

	// RAG context injection (opt-in per consumer)
	if p.consumerFunc != nil {
		consumer := p.consumerFunc(r)
		if mw, ok := p.ragHandlers[consumer]; ok {
			injected, err := mw.InjectContext(body)
			if err != nil {
				log.Printf("[hivemind] RAG injection error for consumer %q: %v", consumer, err)
				// non-fatal: proceed with original body
			} else {
				body = injected
			}
		}
	}

	// Headroom compression (sidecar)
	if p.compressor != nil {
		body = p.compressor.CompressBody(body)
	}

	p.forwardChain(w, r, req.Model, chain, body)
}

// forwardChain tries each backend in the chain until one succeeds or all are exhausted.
// A backend is skipped if the health checker marks it unhealthy, or if a transport
// error occurs during forwarding (connection refused, dial timeout, etc.).
// The X-HiveMind-Fallback header is added when any non-primary backend is used.
func (p *Proxy) forwardChain(w http.ResponseWriter, r *http.Request, model string, chain []*config.Backend, body []byte) {
	var tried []string
	start := time.Now()
	consumer := ""
	if p.consumerFunc != nil {
		consumer = p.consumerFunc(r)
	}

	for _, backend := range chain {
		if !p.health.IsHealthy(backend.Name) {
			log.Printf("[hivemind] fallback: skipping unhealthy backend %q for model %q", backend.Name, model)
			tried = append(tried, backend.Name)
			continue
		}

		// Reset body so each attempt reads a fresh copy.
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		isFallback := len(tried) > 0
		if isFallback {
			logFallback(model, tried[len(tried)-1], backend.Name, "backend_unavailable")
		}

		var transportErr bool
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		p.forwardAttempt(rw, r, backend, isFallback, &transportErr)
		if !transportErr {
			if p.metrics != nil {
				p.metrics.ObserveRequest(consumer, model, backend.Name, rw.status, time.Since(start))
			}
			return // response handled (success or backend-originated error)
		}

		// Transport error — backend unreachable; try next in chain.
		log.Printf("[hivemind] fallback: transport error on backend %q, advancing chain", backend.Name)
		tried = append(tried, backend.Name)
	}

	// All backends in the chain have been exhausted.
	log.Printf("[hivemind] fallback: all backends exhausted for model %q (tried: %s)", model, chainDesc(chain))
	if p.metrics != nil {
		p.metrics.ObserveRequest(consumer, model, "none", http.StatusServiceUnavailable, time.Since(start))
	}
	writeJSONError(w, http.StatusServiceUnavailable, allUnavailableMsg(model, chain))
}

// forwardAttempt reverse-proxies r to a single backend.
// If a transport-level error occurs (before any response headers are written),
// transportErr is set to true and nothing is written to w, allowing the caller
// to retry with the next backend.
func (p *Proxy) forwardAttempt(w http.ResponseWriter, r *http.Request, backend *config.Backend, isFallback bool, transportErr *bool) {
	target, err := url.Parse(backend.URL)
	if err != nil {
		*transportErr = true
		return
	}

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			req.Header.Del("X-Forwarded-For")
			if backend.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+backend.APIKey)
			}
			if backend.PathRewrite != "" {
				if old, nw, ok := parsePathRewrite(backend.PathRewrite); ok && req.URL.Path == old {
					req.URL.Path = nw
				}
			}
		},
		Transport:     p.transport,
		FlushInterval: -1, // stream immediately (SSE pass-through)
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("X-HiveMind-Backend", backend.Name)
			if isFallback {
				resp.Header.Set("X-HiveMind-Fallback", backend.Name)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// Transport error: no response headers written to the client yet.
			// Signal the fallback loop rather than writing a response here.
			log.Printf("[hivemind] transport error on backend %q: %v", backend.Name, err)
			*transportErr = true
		},
	}

	rp.ServeHTTP(w, r)
}

// handleModels serves GET /v1/models — returns all configured models.
func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	type modelObj struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	type modelsResp struct {
		Object string     `json:"object"`
		Data   []modelObj `json:"data"`
	}

	models := make([]modelObj, 0, len(p.backends))
	for name, chain := range p.backends {
		// Report the primary (highest-priority) backend as owner.
		owner := ""
		if len(chain) > 0 {
			owner = chain[0].Name
		}
		models = append(models, modelObj{
			ID:      name,
			Object:  "model",
			OwnedBy: owner,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp{Object: "list", Data: models})
}


// parsePathRewrite parses a path rewrite rule "old=new".
func parsePathRewrite(rule string) (old, new string, ok bool) {
	parts := strings.SplitN(rule, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}
