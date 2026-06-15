// Package tests contains integration tests for the hivemind gateway.
// These tests start real HTTP servers and exercise the full request path.
//
// PII Shield tests use a local mock server by default. TestPIIScanLive
// targets the real PII Shield at :5100 and is skipped when unavailable.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pooknast/HiveMindFusion/internal/config"
	"github.com/pooknast/HiveMindFusion/internal/gateway"
	"github.com/pooknast/HiveMindFusion/internal/pii"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// freePort returns an available TCP port on loopback.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// waitReady polls url until it returns a non-error response or the deadline passes.
func waitReady(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server not ready after 5s: %s", url)
}

// mockLLMBackend returns an httptest.Server that responds with a minimal
// OpenAI-compatible chat completion JSON.
func mockLLMBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "hello"}}},
			"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
}

// harness holds the base URLs for a running test gateway.
type harness struct {
	proxyURL   string
	adminURL   string
	metricsURL string
}

// startGW starts a Gateway with dynamic ports and the given backends.
// extra is called after the base config is built and can mutate it.
// Shutdown is registered as a t.Cleanup so callers don't need to manage it.
func startGW(t *testing.T, backends []config.Backend, extra func(*config.Config)) *harness {
	t.Helper()
	pp, ap, mp := freePort(t), freePort(t), freePort(t)

	cfg := &config.Config{
		Gateway: config.Gateway{
			Port:                   pp,
			AdminPort:              ap,
			MetricsPort:            mp,
			HealthIntervalSecs:     3600, // suppress health polling during tests
			HealthFailureThreshold: 3,
		},
		Backends: backends,
		// Qdrant is required by validation; RAG.Consumers is empty so no RAG runs.
		Qdrant: config.Qdrant{Endpoint: "http://127.0.0.1:19999"},
	}
	if extra != nil {
		extra(cfg)
	}

	g := gateway.New(cfg)
	go func() { g.Run() }() //nolint:errcheck

	h := &harness{
		proxyURL:   fmt.Sprintf("http://127.0.0.1:%d", pp),
		adminURL:   fmt.Sprintf("http://127.0.0.1:%d", ap),
		metricsURL: fmt.Sprintf("http://127.0.0.1:%d", mp),
	}
	waitReady(t, h.proxyURL+"/health")
	waitReady(t, h.adminURL+"/health")
	waitReady(t, h.metricsURL+"/metrics")

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		g.Shutdown(ctx) //nolint:errcheck
	})
	return h
}

// chatBody returns a minimal /v1/chat/completions JSON payload.
func chatBody(model string) []byte {
	b, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	return b
}

// postChat sends a POST /v1/chat/completions to proxyURL with optional header pairs.
func postChat(t *testing.T, proxyURL, model string, headers ...string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, proxyURL+"/v1/chat/completions",
		bytes.NewReader(chatBody(model)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/chat/completions: %v", err)
	}
	return resp
}

// piiAvailable returns true if the PII Shield TCP port at :5100 is open.
func piiAvailable() bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:5100", 200*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// ── gateway integration tests ─────────────────────────────────────────────────

// TestProxyToMockBackend verifies that a request is forwarded to the upstream
// backend and the X-HiveMind-Backend response header is set correctly.
func TestProxyToMockBackend(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "primary", URL: backend.URL, Model: "test-model"},
	}, nil)

	resp := postChat(t, h.proxyURL, "test-model")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("X-HiveMind-Backend"); got != "primary" {
		t.Errorf("X-HiveMind-Backend = %q, want %q", got, "primary")
	}
}

// TestHealthEndpoint checks that GET /health on the proxy port returns 200 {"status":"ok"}.
func TestHealthEndpoint(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "b1", URL: backend.URL, Model: "m1"},
	}, nil)

	resp, err := http.Get(h.proxyURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body = %q, want JSON with 'ok'", body)
	}
}

// TestMetricsEndpoint checks that GET /metrics returns Prometheus text format.
func TestMetricsEndpoint(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "b1", URL: backend.URL, Model: "m1"},
	}, nil)

	resp, err := http.Get(h.metricsURL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	// The Prometheus text exposition format is identified by its Content-Type,
	// regardless of whether any metrics have been observed yet.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") || !strings.Contains(ct, "version=0.0.4") {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Content-Type = %q, want Prometheus text/plain version=0.0.4; body: %s", ct, body)
	}
}

// TestFallbackWhenPrimaryDown verifies that when the primary backend is
// unreachable, traffic falls through to the secondary and X-HiveMind-Fallback
// is set in the response.
func TestFallbackWhenPrimaryDown(t *testing.T) {
	secondary := mockLLMBackend(t)
	defer secondary.Close()

	// Close a server immediately so its port refuses connections.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	h := startGW(t, []config.Backend{
		{Name: "dead-primary", URL: deadURL, Model: "test-model", Priority: 1},
		{Name: "live-secondary", URL: secondary.URL, Model: "test-model", Priority: 2},
	}, nil)

	resp := postChat(t, h.proxyURL, "test-model")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("X-HiveMind-Backend"); got != "live-secondary" {
		t.Errorf("X-HiveMind-Backend = %q, want 'live-secondary'", got)
	}
	if resp.Header.Get("X-HiveMind-Fallback") == "" {
		t.Error("X-HiveMind-Fallback header not set on fallback response")
	}
}

// TestConsumerTracking verifies that the API key → consumer name mapping works
// and that /admin/usage records the request count for the consumer.
func TestConsumerTracking(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "b1", URL: backend.URL, Model: "test-model"},
	}, func(cfg *config.Config) {
		cfg.Consumers.APIKeys = map[string]string{"test-key-abc": "alice"}
	})

	resp := postChat(t, h.proxyURL, "test-model", "Authorization", "Bearer test-key-abc")
	resp.Body.Close()

	usageResp, err := http.Get(h.adminURL + "/admin/usage")
	if err != nil {
		t.Fatalf("GET /admin/usage: %v", err)
	}
	defer usageResp.Body.Close()

	var usage map[string]struct {
		RequestCount int64 `json:"request_count"`
	}
	if err := json.NewDecoder(usageResp.Body).Decode(&usage); err != nil {
		t.Fatalf("decode /admin/usage: %v", err)
	}

	alice, ok := usage["alice"]
	if !ok {
		t.Fatalf("consumer 'alice' not in usage map; got: %v", usage)
	}
	if alice.RequestCount < 1 {
		t.Errorf("alice.request_count = %d, want >= 1", alice.RequestCount)
	}
}

// ── rate limiter tests ────────────────────────────────────────────────────────

// TestRateLimitTriggers429 exercises the RateLimiter middleware in isolation.
// It sets burst=1 so the second request within the same second must be rejected.
func TestRateLimitTriggers429(t *testing.T) {
	rl := gateway.NewRateLimiter(config.RateLimitConfig{
		DefaultRequestsPerMinute: 60,
		DefaultBurst:             1,
	})

	srv := httptest.NewServer(rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	defer srv.Close()

	// First request consumes the single burst token — must succeed.
	r1, err := http.Get(srv.URL + "/") //nolint:noctx
	if err != nil {
		t.Fatalf("request 1: %v", err)
	}
	r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Errorf("request 1: status = %d, want 200", r1.StatusCode)
	}

	// Second request arrives before the token refills — must be rate-limited.
	r2, err := http.Get(srv.URL + "/") //nolint:noctx
	if err != nil {
		t.Fatalf("request 2: %v", err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("request 2: status = %d, want 429", r2.StatusCode)
	}
	if r2.Header.Get("Retry-After") == "" {
		t.Error("Retry-After header missing on 429 response")
	}
}

// ── PII middleware tests ──────────────────────────────────────────────────────

// mockPIIShield returns an httptest.Server that simulates PII Shield.
// blockPattern: if the text contains this string, decision is "block".
// redactPattern: if the text contains this string (and not block), decision is "redact".
func mockPIIShield(t *testing.T, blockPattern, redactPattern string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		decision := "allow"
		sanitized := req.Text
		switch {
		case blockPattern != "" && strings.Contains(req.Text, blockPattern):
			decision = "block"
		case redactPattern != "" && strings.Contains(req.Text, redactPattern):
			decision = "redact"
			sanitized = strings.ReplaceAll(req.Text, redactPattern, "[REDACTED]")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"decision":       decision,
			"sanitized_text": sanitized,
		})
	}))
}

// TestPIIScanBlocksSensitiveRequest verifies that the PII middleware returns
// 403 when the mock PII Shield returns a "block" decision on the request body.
func TestPIIScanBlocksSensitiveRequest(t *testing.T) {
	shield := mockPIIShield(t, "123-45-6789", "")
	defer shield.Close()

	client := pii.NewClient(shield.URL, 2000, false)
	srv := httptest.NewServer(pii.Middleware(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`)) //nolint:errcheck
	})))
	defer srv.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"My SSN is 123-45-6789"}]}`
	resp, err := http.Post(srv.URL+"/", "application/json", strings.NewReader(body)) //nolint:noctx
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (blocked by PII policy)", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), "blocked") {
		t.Errorf("body = %q, want 'blocked' in response", raw)
	}
}

// TestPIIScanCleansResponse verifies that the PII middleware replaces sensitive
// content in the upstream response when the mock PII Shield returns "redact".
func TestPIIScanCleansResponse(t *testing.T) {
	shield := mockPIIShield(t, "", "secret")
	defer shield.Close()

	client := pii.NewClient(shield.URL, 2000, false)

	// Upstream returns a body containing "secret".
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":"the secret is here"}`)) //nolint:errcheck
	})

	srv := httptest.NewServer(pii.Middleware(client, upstream))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "application/json", strings.NewReader(`{"text":"hello"}`)) //nolint:noctx
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(raw), "secret") {
		t.Errorf("response still contains 'secret' after PII redaction: %s", raw)
	}
	if !strings.Contains(string(raw), "[REDACTED]") {
		t.Errorf("response missing '[REDACTED]': %s", raw)
	}
}

// TestModelsEndpoint checks GET /v1/models returns the configured model list.
func TestModelsEndpoint(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "b1", URL: backend.URL, Model: "my-model"},
	}, nil)

	resp, err := http.Get(h.proxyURL + "/v1/models")
	if err != nil {
		t.Fatalf("GET /v1/models: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var result struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode /v1/models: %v", err)
	}
	if result.Object != "list" {
		t.Errorf("object = %q, want 'list'", result.Object)
	}
	found := false
	for _, m := range result.Data {
		if m.ID == "my-model" {
			found = true
		}
	}
	if !found {
		t.Errorf("model 'my-model' not found in response: %+v", result.Data)
	}
}

// TestUnknownModelReturns404 checks that requesting an unconfigured model returns 404.
func TestUnknownModelReturns404(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "b1", URL: backend.URL, Model: "known-model"},
	}, nil)

	resp := postChat(t, h.proxyURL, "no-such-model")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "unknown model") {
		t.Errorf("body = %q, want 'unknown model'", body)
	}
}

// TestInvalidJSONBodyReturns400 checks that malformed request bodies are rejected.
func TestInvalidJSONBodyReturns400(t *testing.T) {
	backend := mockLLMBackend(t)
	defer backend.Close()

	h := startGW(t, []config.Backend{
		{Name: "b1", URL: backend.URL, Model: "m1"},
	}, nil)

	resp, err := http.Post(h.proxyURL+"/v1/chat/completions", "application/json", //nolint:noctx
		strings.NewReader("not-json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestPIIScanLive hits the real PII Shield at :5100. Skipped when unavailable.
// Verifies that the middleware round-trips without error on a benign payload.
func TestPIIScanLive(t *testing.T) {
	if !piiAvailable() {
		t.Skip("PII Shield not available at :5100 — skipping live test")
	}

	client := pii.NewClient("http://127.0.0.1:5100", 2000, true)
	called := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":"hello world"}`)) //nolint:errcheck
	})

	srv := httptest.NewServer(pii.Middleware(client, upstream))
	defer srv.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hello"}]}`
	resp, err := http.Post(srv.URL+"/", "application/json", strings.NewReader(body)) //nolint:noctx
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("PII Shield at :5100 returned 503 — is it healthy?")
	}
	if !called && resp.StatusCode != http.StatusForbidden {
		t.Error("upstream was not called and response is not a block")
	}
}
