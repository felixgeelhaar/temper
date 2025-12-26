package middleware_test

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/api/middleware"
)

func TestRateLimiter_Allow(t *testing.T) {
	// Create a limiter: 5 requests per second, burst of 5
	rl := middleware.NewRateLimiter(5, time.Second, 5)

	key := "test-client"

	// Should allow first 5 requests (burst)
	for i := 0; i < 5; i++ {
		if !rl.Allow(key) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if rl.Allow(key) {
		t.Error("6th request should be denied")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	// Create a limiter: 10 requests per 100ms, burst of 2
	rl := middleware.NewRateLimiter(10, 100*time.Millisecond, 2)

	key := "test-client"

	// Use up the burst
	rl.Allow(key)
	rl.Allow(key)

	// Should be denied now
	if rl.Allow(key) {
		t.Error("Should be denied after burst exhausted")
	}

	// Wait for refill
	time.Sleep(110 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow(key) {
		t.Error("Should be allowed after token refill")
	}
}

func TestRateLimiter_MultipleClients(t *testing.T) {
	rl := middleware.NewRateLimiter(2, time.Second, 2)

	client1 := "client-1"
	client2 := "client-2"

	// Each client has their own bucket
	rl.Allow(client1)
	rl.Allow(client1)

	// Client 1 should be denied
	if rl.Allow(client1) {
		t.Error("Client 1 should be denied")
	}

	// Client 2 should still be allowed
	if !rl.Allow(client2) {
		t.Error("Client 2 should be allowed")
	}
}

func TestRateLimiter_Remaining(t *testing.T) {
	rl := middleware.NewRateLimiter(5, time.Second, 5)
	key := "test-client"

	// Initially should have full burst
	if remaining := rl.Remaining(key); remaining != 5 {
		t.Errorf("Remaining = %d; want 5", remaining)
	}

	// After one request
	rl.Allow(key)
	if remaining := rl.Remaining(key); remaining != 4 {
		t.Errorf("Remaining = %d; want 4", remaining)
	}

	// After exhausting
	rl.Allow(key)
	rl.Allow(key)
	rl.Allow(key)
	rl.Allow(key)

	if remaining := rl.Remaining(key); remaining != 0 {
		t.Errorf("Remaining = %d; want 0", remaining)
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := middleware.DefaultRateLimitConfig()

	if config.RequestsPerMinute <= 0 {
		t.Error("RequestsPerMinute should be positive")
	}
	if config.ExpensiveRequestsPerMinute <= 0 {
		t.Error("ExpensiveRequestsPerMinute should be positive")
	}
	if config.BurstMultiplier <= 0 {
		t.Error("BurstMultiplier should be positive")
	}
}
