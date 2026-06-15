package pii

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Middleware returns an http.Handler that scans request and response bodies
// through PII Shield before forwarding to next.
//
// Consumer identity is read from X-HiveMind-Consumer; defaults to "__unknown__".
// Decision semantics:
//   - allow  → body passes through unchanged
//   - redact → body is replaced with sanitized_text
//   - block  → 403 is returned immediately
func Middleware(client *Client, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-HiveMind-Consumer")
		if agentID == "" {
			agentID = "__unknown__"
		}

		// --- scan request body ---
		if r.Body != nil {
			raw, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
				return
			}

			if len(raw) > 0 {
				result, err := client.Scan(agentID, "input", string(raw))
				if err != nil {
					http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusServiceUnavailable)
					return
				}
				switch result.Decision {
				case "block":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"error":"request blocked by PII policy"}`)
					return
				case "redact":
					raw = []byte(result.SanitizedText)
				}
			}

			r.Body = io.NopCloser(bytes.NewReader(raw))
			r.ContentLength = int64(len(raw))
		}

		// --- capture response ---
		rec := &bodyRecorder{
			ResponseWriter: w,
			buf:            &bytes.Buffer{},
			status:         http.StatusOK,
		}
		next.ServeHTTP(rec, r)

		// --- scan response body ---
		respBody := rec.buf.Bytes()
		if len(respBody) > 0 {
			result, err := client.Scan(agentID, "output", string(respBody))
			if err != nil {
				http.Error(w, `{"error":"PII shield unavailable"}`, http.StatusServiceUnavailable)
				return
			}
			switch result.Decision {
			case "block":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":"response blocked by PII policy"}`)
				return
			case "redact":
				respBody = []byte(result.SanitizedText)
			}
		}

		// bodyRecorder.Header() delegates to the underlying w, so headers set by
		// next are already on w. Just adjust Content-Length and flush.
		w.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
		w.WriteHeader(rec.status)
		w.Write(respBody) //nolint:errcheck
	})
}

// bodyRecorder captures the status code and body written by the wrapped handler
// without forwarding them to the underlying ResponseWriter until we're done.
//
// Header() is NOT overridden, so any Header().Set() calls by the wrapped handler
// write directly to the real ResponseWriter's header map — no double-copy needed.
type bodyRecorder struct {
	http.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (r *bodyRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *bodyRecorder) Write(b []byte) (int, error) {
	return r.buf.Write(b)
}
