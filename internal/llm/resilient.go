package llm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/felixgeelhaar/fortify/bulkhead"
	"github.com/felixgeelhaar/fortify/circuitbreaker"
	"github.com/felixgeelhaar/fortify/ratelimit"
	"github.com/felixgeelhaar/fortify/retry"
)

// ResilientProvider wraps an LLM provider with resilience patterns from fortify
type ResilientProvider struct {
	provider       Provider
	circuitBreaker circuitbreaker.CircuitBreaker[*Response]
	retrier        retry.Retry[*Response]
	bulkhead       bulkhead.Bulkhead[*Response]
	rateLimit      ratelimit.RateLimiter
	logger         *slog.Logger
	name           string
}

// ResilientConfig holds configuration for resilient provider wrapper
type ResilientConfig struct {
	// EnableCircuitBreaker enables circuit breaker pattern
	EnableCircuitBreaker bool

	// EnableRetry enables retry with backoff
	EnableRetry bool

	// EnableBulkhead enables concurrency limiting
	EnableBulkhead bool

	// EnableRateLimit enables rate limiting
	EnableRateLimit bool

	// MaxConcurrent for bulkhead (default: 5)
	MaxConcurrent int

	// RatePerSecond for rate limiting (default: 2)
	RatePerSecond int

	// Logger for resilience events
	Logger *slog.Logger
}

// DefaultResilientConfig returns sensible defaults for LLM resilience
func DefaultResilientConfig() ResilientConfig {
	return ResilientConfig{
		EnableCircuitBreaker: true,
		EnableRetry:          true,
		EnableBulkhead:       true,
		EnableRateLimit:      true,
		MaxConcurrent:        5,
		RatePerSecond:        2,
	}
}

// NewResilientProvider wraps a provider with resilience patterns using fortify
func NewResilientProvider(provider Provider, cfg ResilientConfig) *ResilientProvider {
	rp := &ResilientProvider{
		provider: provider,
		logger:   cfg.Logger,
		name:     provider.Name(),
	}

	// Configure circuit breaker
	if cfg.EnableCircuitBreaker {
		rp.circuitBreaker = circuitbreaker.New[*Response](circuitbreaker.Config{
			MaxRequests: 2,
			Interval:    10 * time.Second,
			Timeout:     60 * time.Second,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
			OnStateChange: func(from, to circuitbreaker.State) {
				if rp.logger != nil {
					rp.logger.Warn("circuit breaker state change",
						"provider", provider.Name(),
						"from", from.String(),
						"to", to.String())
				}
			},
		})
	}

	// Configure retry
	if cfg.EnableRetry {
		rp.retrier = retry.New[*Response](retry.Config{
			MaxAttempts:   3,
			InitialDelay:  2 * time.Second,
			MaxDelay:      60 * time.Second,
			Multiplier:    2.0,
			BackoffPolicy: retry.BackoffExponential,
			Jitter:        true,
			IsRetryable: func(err error) bool {
				return isRetryableHTTPError(err)
			},
		})
	}

	// Configure bulkhead
	if cfg.EnableBulkhead {
		maxConcurrent := cfg.MaxConcurrent
		if maxConcurrent <= 0 {
			maxConcurrent = 5
		}
		rp.bulkhead = bulkhead.New[*Response](bulkhead.Config{
			MaxConcurrent: maxConcurrent,
			MaxQueue:      maxConcurrent * 2,
			QueueTimeout:  30 * time.Second,
		})
	}

	// Configure rate limiter
	if cfg.EnableRateLimit {
		rate := cfg.RatePerSecond
		if rate <= 0 {
			rate = 2
		}
		rp.rateLimit = ratelimit.New(&ratelimit.Config{
			Rate:     rate,
			Burst:    rate * 3,
			Interval: time.Second,
		})
	}

	return rp
}

func (p *ResilientProvider) Name() string {
	return p.provider.Name()
}

func (p *ResilientProvider) SupportsStreaming() bool {
	return p.provider.SupportsStreaming()
}

func (p *ResilientProvider) Generate(ctx context.Context, req *Request) (*Response, error) {
	// Apply rate limiting
	if p.rateLimit != nil {
		if !p.rateLimit.Allow(ctx, p.name) {
			return nil, fmt.Errorf("rate limit exceeded for provider %s", p.name)
		}
	}

	// Define the core operation
	operation := func(ctx context.Context) (*Response, error) {
		return p.provider.Generate(ctx, req)
	}

	// Apply bulkhead if enabled
	if p.bulkhead != nil {
		operation = func(ctx context.Context) (*Response, error) {
			return p.bulkhead.Execute(ctx, func(ctx context.Context) (*Response, error) {
				return p.provider.Generate(ctx, req)
			})
		}
	}

	// Apply circuit breaker + retry
	if p.circuitBreaker != nil && p.retrier != nil {
		return p.circuitBreaker.Execute(ctx, func(ctx context.Context) (*Response, error) {
			return p.retrier.Do(ctx, operation)
		})
	}

	if p.circuitBreaker != nil {
		return p.circuitBreaker.Execute(ctx, operation)
	}

	if p.retrier != nil {
		return p.retrier.Do(ctx, operation)
	}

	return operation(ctx)
}

func (p *ResilientProvider) GenerateStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	// Apply rate limiting
	if p.rateLimit != nil {
		if !p.rateLimit.Allow(ctx, p.name) {
			return nil, fmt.Errorf("rate limit exceeded for provider %s", p.name)
		}
	}

	// Note: Bulkhead not applied to streams as they're long-running
	// and would hold semaphore for extended periods

	// Execute stream (no retry for streams - they're stateful)
	ch, err := p.provider.GenerateStream(ctx, req)
	if err != nil {
		return nil, err
	}

	return ch, nil
}

// Close releases resources held by the resilient provider
func (p *ResilientProvider) Close() error {
	if p.rateLimit != nil {
		return p.rateLimit.Close()
	}
	return nil
}

// isRetryableHTTPError checks if an error is retryable based on HTTP semantics
func isRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}

	code := extractStatusCode(err)
	retryableCodes := []int{
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}

	for _, rc := range retryableCodes {
		if code == rc {
			return true
		}
	}

	return false
}

// extractStatusCode tries to extract HTTP status code from error message
func extractStatusCode(err error) int {
	if err == nil {
		return 0
	}

	errStr := err.Error()

	// Look for common patterns like "status 429" or "(status 500)"
	statusCodes := map[string]int{
		"status 429": http.StatusTooManyRequests,
		"status 500": http.StatusInternalServerError,
		"status 502": http.StatusBadGateway,
		"status 503": http.StatusServiceUnavailable,
		"status 504": http.StatusGatewayTimeout,
	}

	for pattern, code := range statusCodes {
		if containsString(errStr, pattern) {
			return code
		}
	}

	return 0
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
