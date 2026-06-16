package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/pooknast/HiveMindFusion/internal/config"
	"github.com/pooknast/HiveMindFusion/internal/fusion"
	"github.com/pooknast/HiveMindFusion/internal/rag"
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
	fusionEngine *fusion.Engine             // in-process fusion orchestrator (nil if disabled)
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

	p := &Proxy{
		backends:     buildFallbackChain(cfg.Backends),
		health:       hc,
		metrics:      metrics,
		transport:    transport,
		ragHandlers:  ragHandlers,
		consumerFunc: consumerFunc,
		compressor:   NewCompressor(cfg.Compression),
	}

	// Wire fusion engine if enabled — uses backendCaller to dispatch directly
	// through the proxy's own transport (no loopback through the listener).
	if cfg.Fusion.Enabled {
		p.fusionEngine = fusion.New(cfg.Fusion, p.backendCaller)
	}

	return p
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

	// Fusion models are handled in-process — no backend routing.
	if p.fusionEngine != nil {
		tier := fusion.ExtractTier(req.Model)
		if tier != "" {
			if !p.fusionEngine.HasTier(tier) {
				writeJSONError(w, http.StatusNotFound, fmt.Sprintf("unknown fusion tier: %s (available: %v)", tier, p.fusionEngine.TierNames()))
				return
			}
			p.handleFusionRequest(w, r, body, tier)
			return
		}
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
	// Add fusion virtual models
	if p.fusionEngine != nil {
		for _, tier := range p.fusionEngine.TierNames() {
			models = append(models, modelObj{
				ID:      "hivemind/fusion-" + tier,
				Object:  "model",
				OwnedBy: "hivemind-fusion",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp{Object: "list", Data: models})
}


// backendCaller implements fusion.BackendCaller — calls a specific backend model
// directly through the proxy's HTTP transport (no gateway loopback).
func (p *Proxy) backendCaller(ctx context.Context, model string, messages []fusion.Message) (fusion.BackendResponse, error) {
	chain, ok := p.backends[model]
	if !ok {
		return fusion.BackendResponse{Model: model, Error: "unknown model"}, fmt.Errorf("unknown model: %s", model)
	}

	// Build a minimal OpenAI request body
	type chatReq struct {
		Model    string           `json:"model"`
		Messages []fusion.Message `json:"messages"`
	}
	reqBody, _ := json.Marshal(chatReq{Model: model, Messages: messages})

	start := time.Now()

	// Walk the fallback chain for a healthy backend
	for _, backend := range chain {
		if !p.health.IsHealthy(backend.Name) {
			continue
		}

		target, err := url.Parse(backend.URL)
		if err != nil {
			continue
		}

		reqURL := fmt.Sprintf("%s://%s/v1/chat/completions", target.Scheme, target.Host)
		if backend.PathRewrite != "" {
			if old, nw, ok := parsePathRewrite(backend.PathRewrite); ok && old == "/v1/chat/completions" {
				reqURL = fmt.Sprintf("%s://%s%s", target.Scheme, target.Host, nw)
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if backend.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+backend.APIKey)
		}

		resp, err := p.transport.RoundTrip(httpReq)
		if err != nil {
			log.Printf("[fusion] transport error calling %s via %s: %v", model, backend.Name, err)
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fusion.BackendResponse{Model: model, Error: "read error"}, err
		}

		latencyMs := int(time.Since(start).Milliseconds())

		// Parse OpenAI response to extract content
		var oaiResp struct {
			Choices []struct {
				Message struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				TotalTokens int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(respBody, &oaiResp); err != nil {
			return fusion.BackendResponse{Model: model, Content: string(respBody), LatencyMs: latencyMs}, nil
		}

		content := ""
		tokens := 0
		if len(oaiResp.Choices) > 0 {
			content = oaiResp.Choices[0].Message.Content
			if content == "" {
				content = oaiResp.Choices[0].Message.ReasoningContent
			}
		}
		tokens = oaiResp.Usage.TotalTokens

		return fusion.BackendResponse{
			Model:     model,
			Content:   content,
			Tokens:    tokens,
			LatencyMs: latencyMs,
		}, nil
	}

	return fusion.BackendResponse{Model: model, Error: "all backends exhausted"}, fmt.Errorf("all backends exhausted for model: %s", model)
}

// handleFusionRequest handles fusion model requests in-process.
func (p *Proxy) handleFusionRequest(w http.ResponseWriter, r *http.Request, body []byte, tier string) {
	var req struct {
		Messages []fusion.Message `json:"messages"`
		Stream   bool             `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Stream {
		p.handleFusionStream(w, r, tier, req.Messages)
		return
	}

	result, err := p.fusionEngine.RunFusion(r.Context(), tier, req.Messages)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check for quorum error in the result
	if errMsg, ok := result["error"].(string); ok {
		writeJSONError(w, http.StatusBadGateway, errMsg)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleFusionStream streams fusion results as SSE.
func (p *Proxy) handleFusionStream(w http.ResponseWriter, r *http.Request, tier string, messages []fusion.Message) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ch := make(chan fusion.StreamChunk, 64)
	completionID := fmt.Sprintf("fusion-%d", time.Now().UnixNano()%1e12)
	created := time.Now().Unix()

	go func() {
		if err := p.fusionEngine.RunFusionStream(r.Context(), tier, messages, ch); err != nil {
			log.Printf("[fusion] stream error: %v", err)
		}
	}()

	for chunk := range ch {
		var sseData map[string]any
		if chunk.FinishReason == "stop" {
			sseData = map[string]any{
				"id": completionID, "object": "chat.completion.chunk",
				"created": created, "model": "hivemind/fusion-" + tier,
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
			}
		} else {
			sseData = map[string]any{
				"id": completionID, "object": "chat.completion.chunk",
				"created": created, "model": "hivemind/fusion-" + tier,
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": chunk.Content}, "finish_reason": nil}},
			}
		}
		data, _ := json.Marshal(sseData)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
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
