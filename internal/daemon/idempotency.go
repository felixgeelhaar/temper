package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// IdempotencyHeader is the canonical request header used to deduplicate
// non-idempotent POSTs. Editor clients retrying on a network blip should
// reuse the same key; the daemon then returns the cached prior result
// rather than executing the side effect twice.
const IdempotencyHeader = "Idempotency-Key"

// IdempotencyTTL is how long a cached result remains valid. Long enough
// to cover a retry storm, short enough that disk usage stays bounded.
const IdempotencyTTL = 5 * time.Minute

// idempotencyEntry caches the response payload + status for a (session,
// key, body-hash) tuple. Includes a fingerprint of the request body so
// reusing the same key with a different payload is treated as a new
// request rather than silently returning the prior response.
type idempotencyEntry struct {
	bodyHash   string
	statusCode int
	payload    []byte
	expiresAt  time.Time
}

// IdempotencyCache is an in-memory deduplication cache. Bounded by
// IdempotencyTTL — entries past their expiry are skipped on read and
// evicted lazily.
type IdempotencyCache struct {
	mu      sync.Mutex
	entries map[string]idempotencyEntry
}

// NewIdempotencyCache returns an empty cache.
func NewIdempotencyCache() *IdempotencyCache {
	return &IdempotencyCache{
		entries: make(map[string]idempotencyEntry),
	}
}

// hashBody returns a stable fingerprint of the request body. Differing
// bodies under the same key are treated as conflicts.
func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// cacheKey scopes idempotency to a session so two different sessions
// re-using the same key don't collide.
func cacheKey(sessionID, idemKey string) string {
	return sessionID + "|" + idemKey
}

// Lookup returns the cached entry for this (session, key) pair if one
// exists and matches the current body hash. Returns ok=false on miss,
// expired entry, or body-hash mismatch (the second return is true with
// conflict=true to signal HTTP 409).
func (c *IdempotencyCache) Lookup(sessionID, idemKey string, body []byte) (entry idempotencyEntry, ok, conflict bool) {
	if idemKey == "" {
		return idempotencyEntry{}, false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	e, found := c.entries[cacheKey(sessionID, idemKey)]
	if !found {
		return idempotencyEntry{}, false, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, cacheKey(sessionID, idemKey))
		return idempotencyEntry{}, false, false
	}
	if e.bodyHash != hashBody(body) {
		return idempotencyEntry{}, false, true
	}
	return e, true, false
}

// Store records the response for later replay. The body is kept verbatim
// so the client receives the exact same bytes it would have on the
// original call.
func (c *IdempotencyCache) Store(sessionID, idemKey string, body []byte, statusCode int, payload []byte) {
	if idemKey == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey(sessionID, idemKey)] = idempotencyEntry{
		bodyHash:   hashBody(body),
		statusCode: statusCode,
		payload:    payload,
		expiresAt:  time.Now().Add(IdempotencyTTL),
	}
}

// Sweep evicts expired entries. Call periodically from a background loop
// (the daemon's main goroutine starts one in Start()) to keep the map
// bounded under sustained traffic.
func (c *IdempotencyCache) Sweep() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	removed := 0
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
			removed++
		}
	}
	return removed
}
