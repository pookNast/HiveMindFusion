package pii

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakePIIShield returns an httptest.Server that responds with the given decision.
func fakePIIShield(t *testing.T, decision, sanitized string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scan" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ScanResponse{
			Decision:      decision,
			SanitizedText: sanitized,
		})
	}))
}

// echoHandler returns the request body as the response body.
var echoHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(r.Body)
	w.Write(body)
})

func newTestClient(endpoint string, bypassOnFailure bool) *Client {
	return NewClient(endpoint, 2000, bypassOnFailure)
}

// --- request scanning ---

func TestMiddleware_RequestAllow(t *testing.T) {
	shield := fakePIIShield(t, "allow", "")
	defer shield.Close()

	client := newTestClient(shield.URL, false)
	handler := Middleware(client, echoHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"msg":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HiveMind-Consumer", "agent-1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("body not forwarded: %q", w.Body.String())
	}
}

func TestMiddleware_RequestRedact(t *testing.T) {
	shield := fakePIIShield(t, "redact", `{"msg":"[REDACTED]"}`)
	defer shield.Close()

	var gotBody string
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write(b)
	})

	client := newTestClient(shield.URL, false)
	handler := Middleware(client, capture)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"msg":"secret@email.com"}`))
	req.Header.Set("X-HiveMind-Consumer", "agent-1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(gotBody, "[REDACTED]") {
		t.Fatalf("expected sanitized body to reach backend, got %q", gotBody)
	}
}

func TestMiddleware_RequestBlock(t *testing.T) {
	shield := fakePIIShield(t, "block", "")
	defer shield.Close()

	client := newTestClient(shield.URL, false)
	handler := Middleware(client, echoHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ssn":"123-45-6789"}`))
	req.Header.Set("X-HiveMind-Consumer", "agent-1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "blocked") {
		t.Fatalf("expected block message in body, got %q", w.Body.String())
	}
}

// --- response scanning ---

func TestMiddleware_ResponseRedact(t *testing.T) {
	// Shield: allow input, redact output.
	callCount := 0
	shield := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req ScanRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := ScanResponse{Decision: "allow", SanitizedText: req.Text}
		if req.Direction == "output" {
			resp.Decision = "redact"
			resp.SanitizedText = `{"msg":"[REDACTED]"}`
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer shield.Close()

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"msg":"secret phone 555-1234"}`))
	})

	client := newTestClient(shield.URL, false)
	handler := Middleware(client, backend)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"q":"tell me"}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "[REDACTED]") {
		t.Fatalf("expected sanitized response body, got %q", w.Body.String())
	}
	if callCount != 2 {
		t.Fatalf("expected 2 shield calls (input+output), got %d", callCount)
	}
}

func TestMiddleware_ResponseBlock(t *testing.T) {
	shield := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ScanRequest
		json.NewDecoder(r.Body).Decode(&req)
		decision := "allow"
		if req.Direction == "output" {
			decision = "block"
		}
		json.NewEncoder(w).Encode(ScanResponse{Decision: decision})
	}))
	defer shield.Close()

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":"highly sensitive"}`))
	})

	client := newTestClient(shield.URL, false)
	handler := Middleware(client, backend)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// --- circuit breaker ---

func TestMiddleware_CircuitBreaker_BypassTrue(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", true) // unreachable
	// Simulate circuit already open by backdating firstFailure.
	client.mu.Lock()
	client.firstFailure = time.Now().Add(-60 * time.Second)
	client.mu.Unlock()

	var gotBody string
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write(b)
	})

	handler := Middleware(client, backend)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"original":true}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("bypass_on_failure=true: expected 200, got %d", w.Code)
	}
	if !strings.Contains(gotBody, `"original":true`) {
		t.Fatalf("expected original body to pass through, got %q", gotBody)
	}
}

func TestMiddleware_CircuitBreaker_BypassFalse(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", false) // unreachable
	// Simulate circuit already open.
	client.mu.Lock()
	client.firstFailure = time.Now().Add(-60 * time.Second)
	client.mu.Unlock()

	handler := Middleware(client, echoHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"original":true}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("bypass_on_failure=false: expected 503, got %d", w.Code)
	}
}

func TestMiddleware_CircuitBreaker_Opens_After_Timeout(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", false) // unreachable
	client.cbTimeout = 50 * time.Millisecond

	// First request: shield unreachable → records firstFailure, returns 503.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	Middleware(client, echoHandler).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on first failure, got %d", w.Code)
	}

	// Circuit not yet open (< 50ms elapsed).
	if client.isOpen() {
		t.Fatal("circuit should not be open yet")
	}

	// Wait for circuit to open.
	time.Sleep(60 * time.Millisecond)

	if !client.isOpen() {
		t.Fatal("circuit should be open after timeout")
	}
}

func TestMiddleware_NoConsumerHeader_DefaultsToUnknown(t *testing.T) {
	var gotAgentID string
	shield := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ScanRequest
		json.NewDecoder(r.Body).Decode(&req)
		gotAgentID = req.AgentID
		json.NewEncoder(w).Encode(ScanResponse{Decision: "allow"})
	}))
	defer shield.Close()

	client := newTestClient(shield.URL, false)
	handler := Middleware(client, echoHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	// No X-HiveMind-Consumer header.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotAgentID != "__unknown__" {
		t.Fatalf("expected agent_id __unknown__, got %q", gotAgentID)
	}
}
