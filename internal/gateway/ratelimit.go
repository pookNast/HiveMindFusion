package gateway

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

// bucket is a token bucket for a single consumer.
type bucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newBucket(requestsPerMinute, burst int) *bucket {
	b := &bucket{
		maxTokens:  float64(burst),
		refillRate: float64(requestsPerMinute) / 60.0,
		lastRefill: time.Now(),
	}
	b.tokens = b.maxTokens
	return b
}

// take attempts to consume one token.
// Returns (true, 0) when allowed, or (false, retryAfter) when rate limited.
func (b *bucket) take() (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	// Time until the next token is available.
	needed := 1.0 - b.tokens
	wait := time.Duration(needed / b.refillRate * float64(time.Second))
	return false, wait
}

// RateLimiter enforces per-consumer token bucket rate limits.
// Consumer identity is taken from the X-HiveMind-Consumer header, falling back
// to the Authorization header value, then a synthetic "__default__" key.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	cfg     config.RateLimitConfig
}

// NewRateLimiter constructs a RateLimiter from the given config section.
func NewRateLimiter(cfg config.RateLimitConfig) *RateLimiter {
	if cfg.DefaultRequestsPerMinute == 0 {
		cfg.DefaultRequestsPerMinute = 60
	}
	if cfg.DefaultBurst == 0 {
		cfg.DefaultBurst = 10
	}
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		cfg:     cfg,
	}
}

// Update replaces the active config and resets all buckets so that the new
// limits take effect on the next request (satisfies "no restart" reload).
func (rl *RateLimiter) Update(cfg config.RateLimitConfig) {
	if cfg.DefaultRequestsPerMinute == 0 {
		cfg.DefaultRequestsPerMinute = 60
	}
	if cfg.DefaultBurst == 0 {
		cfg.DefaultBurst = 10
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.cfg = cfg
	rl.buckets = make(map[string]*bucket)
}

func (rl *RateLimiter) getBucket(consumer string) *bucket {
	rl.mu.RLock()
	b, ok := rl.buckets[consumer]
	cfg := rl.cfg
	rl.mu.RUnlock()

	if ok {
		return b
	}

	rpm := cfg.DefaultRequestsPerMinute
	burst := cfg.DefaultBurst
	if cl, found := cfg.Consumers[consumer]; found {
		rpm = cl.RequestsPerMinute
		burst = cl.Burst
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	// Re-check under write lock to avoid duplicate creation.
	if b, ok = rl.buckets[consumer]; ok {
		return b
	}
	b = newBucket(rpm, burst)
	rl.buckets[consumer] = b
	return b
}

// Middleware wraps next and enforces rate limits, returning 429 + Retry-After
// when a consumer exceeds their configured limit.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		consumer := r.Header.Get("X-HiveMind-Consumer")
		if consumer == "" {
			consumer = r.Header.Get("Authorization")
		}
		if consumer == "" {
			consumer = "__default__"
		}

		allowed, retryAfter := rl.getBucket(consumer).take()
		if !allowed {
			secs := int(retryAfter.Seconds()) + 1
			w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after":%d}`, secs)
			return
		}

		next.ServeHTTP(w, r)
	})
}
