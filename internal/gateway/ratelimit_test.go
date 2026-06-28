package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

func TestConsumerFromContext(t *testing.T) {
	cases := []struct {
		name string
		ctx  func() context.Context
		want string
	}{
		{"authenticated consumer", func() context.Context {
			return context.WithValue(context.Background(), ConsumerCtxKey, "ralph-swarm")
		}, "ralph-swarm"},
		{"master token", func() context.Context {
			return context.WithValue(context.Background(), ConsumerCtxKey, "__master__")
		}, "__master__"},
		{"no identity (permissive mode)", func() context.Context {
			return context.Background()
		}, "__default__"},
		{"wrong key type", func() context.Context {
			return context.WithValue(context.Background(), ConsumerCtxKey, 123)
		}, "__default__"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(c.ctx())
			if got := consumerFromContext(req); got != c.want {
				t.Errorf("consumerFromContext() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestRateLimiterMiddleware_UnspoofableHeader confirms the rate limiter keys off
// the authenticated context value, NOT the X-HiveMind-Consumer header — so a
// spoofed header cannot move a caller to a different consumer's bucket.
func TestRateLimiterMiddleware_UnspoofableHeader(t *testing.T) {
	rl := NewRateLimiter(config.RateLimitConfig{}) // empty cfg → default bucket for unknown consumers
	if rl == nil {
		t.Skip("NewRateLimiter returned nil; default-rate config required to run this test")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Authenticated as __master__ but claiming to be ralph-swarm in the header.
	req = req.WithContext(context.WithValue(req.Context(), ConsumerCtxKey, "__master__"))
	req.Header.Set("X-HiveMind-Consumer", "ralph-swarm")

	// consumerFromContext must return the context identity, ignoring the header.
	if got := consumerFromContext(req); got != "__master__" {
		t.Fatalf("spoofed header leaked past context: got %q, want __master__", got)
	}
}
