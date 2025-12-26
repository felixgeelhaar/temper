package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	burst    int           // max tokens (bucket size)
	cleanup  time.Duration // cleanup interval for stale buckets
}

type bucket struct {
	tokens    int
	lastCheck time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: number of requests allowed per interval
// interval: time period for rate (e.g., time.Minute)
// burst: maximum burst size (bucket capacity)
func NewRateLimiter(rate int, interval time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		burst:    burst,
		cleanup:  5 * time.Minute,
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given key should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]

	if !exists {
		// New bucket starts full
		rl.buckets[key] = &bucket{
			tokens:    rl.burst - 1, // consume one token
			lastCheck: now,
		}
		return true
	}

	// Calculate tokens to add based on elapsed time
	elapsed := now.Sub(b.lastCheck)
	tokensToAdd := int(elapsed / rl.interval) * rl.rate

	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, rl.burst)
		b.lastCheck = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// Remaining returns the number of remaining tokens for a key
func (rl *RateLimiter) Remaining(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if b, exists := rl.buckets[key]; exists {
		return b.tokens
	}
	return rl.burst
}

// cleanupLoop periodically removes stale buckets
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for key, b := range rl.buckets {
			if b.lastCheck.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitConfig configures the rate limiting middleware
type RateLimitConfig struct {
	// Requests per minute for general API endpoints
	RequestsPerMinute int
	// Requests per minute for expensive endpoints (runs, pairing)
	ExpensiveRequestsPerMinute int
	// Burst size multiplier (burst = rate * multiplier)
	BurstMultiplier int
}

// DefaultRateLimitConfig returns sensible defaults
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute:          60,  // 1 per second average
		ExpensiveRequestsPerMinute: 10,  // rate limit code execution
		BurstMultiplier:            3,   // allow bursts of 3x rate
	}
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	generalLimiter := NewRateLimiter(
		config.RequestsPerMinute,
		time.Minute,
		config.RequestsPerMinute*config.BurstMultiplier,
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use IP address as key (in production, use user ID for authenticated requests)
			key := getClientIP(r)

			if !generalLimiter.Allow(key) {
				slog.Warn("rate limit exceeded",
					"ip", key,
					"path", r.URL.Path,
					"request_id", GetRequestID(r.Context()),
				)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"too many requests, please try again later"}}`))
				return
			}

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Remaining", string(rune('0'+generalLimiter.Remaining(key))))

			next.ServeHTTP(w, r)
		})
	}
}

// ExpensiveRateLimitMiddleware creates stricter rate limiting for expensive operations
func ExpensiveRateLimitMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	expensiveLimiter := NewRateLimiter(
		config.ExpensiveRequestsPerMinute,
		time.Minute,
		config.ExpensiveRequestsPerMinute*config.BurstMultiplier,
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := getClientIP(r)

			if !expensiveLimiter.Allow(key) {
				slog.Warn("expensive rate limit exceeded",
					"ip", key,
					"path", r.URL.Path,
					"request_id", GetRequestID(r.Context()),
				)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"too many code execution requests, please wait before trying again"}}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take first IP in chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
