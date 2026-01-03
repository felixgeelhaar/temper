# ADR-002: Production Resilience Patterns

## Status
Accepted

## Context
Temper relies on external LLM providers (Claude, OpenAI, Ollama) for AI pairing. These services can:
- Rate limit requests (429 errors)
- Experience outages (5xx errors)
- Have high latency (slow responses)
- Timeout unexpectedly

Without resilience patterns, a single provider failure cascades through the system, degrading user experience.

## Decision
We integrated `felixgeelhaar/fortify` for production-grade resilience, applying patterns at the LLM provider level.

### Pattern Configuration

#### Circuit Breaker
```go
circuitbreaker.Config{
    MaxRequests: 2,           // Requests in half-open state
    Interval:    10 * time.Second,
    Timeout:     60 * time.Second,  // Time before half-open
    ReadyToTrip: func(counts) bool {
        return counts.ConsecutiveFailures >= 3
    },
}
```

#### Retry with Backoff
```go
retry.Config{
    MaxAttempts:   3,
    InitialDelay:  2 * time.Second,
    MaxDelay:      60 * time.Second,
    Multiplier:    2.0,
    BackoffPolicy: retry.BackoffExponential,
    Jitter:        true,
    IsRetryable:   isRetryableHTTPError,  // 429, 5xx
}
```

#### Rate Limiting
```go
ratelimit.Config{
    Rate:     2,              // Requests per second
    Burst:    6,              // Max burst
    Interval: time.Second,
}
```

#### Bulkhead
```go
bulkhead.Config{
    MaxConcurrent: 5,         // Max concurrent LLM calls
    MaxQueue:      10,        // Queued requests
    QueueTimeout:  30 * time.Second,
}
```

### HTTP Client Configuration
```go
http.Client{
    Timeout: 120 * time.Second,  // Total request timeout
    Transport: &http.Transport{
        DialContext:           10s timeout,
        TLSHandshakeTimeout:   10s,
        ResponseHeaderTimeout: 60s,
        MaxConnsPerHost:       10,
    },
}
```

## Consequences

### Positive
- **Fail-fast**: Circuit breaker prevents waiting on dead providers
- **Self-healing**: Automatic recovery after timeout period
- **Fair usage**: Rate limiting respects provider quotas
- **Resource protection**: Bulkhead prevents thread exhaustion
- **Graceful degradation**: Retries handle transient failures

### Negative
- Added latency from retry backoff during failures
- Memory overhead for rate limiter token buckets
- Complexity in debugging (need to check circuit state)

### Monitoring Recommendations
- Log circuit breaker state changes (implemented via OnStateChange)
- Track retry attempt counts
- Alert on sustained open circuit state
- Monitor rate limit rejections

## Alternatives Considered

### 1. Custom Implementation
Rejected: Fortify is battle-tested and maintained.

### 2. No Resilience (Fail-Through)
Rejected: Unacceptable user experience during outages.

### 3. Provider Fallback Chain
Deferred: Could add automatic failover to backup providers in future.

## References
- Nygard, Michael. "Release It! Design and Deploy Production-Ready Software"
- Fortify documentation: github.com/felixgeelhaar/fortify
