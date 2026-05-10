package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIdempotencyCache_LookupMissOnEmptyKey(t *testing.T) {
	c := NewIdempotencyCache()
	if _, ok, _ := c.Lookup("sess", "", []byte("body")); ok {
		t.Error("empty key must miss")
	}
}

func TestIdempotencyCache_StoreAndReplay(t *testing.T) {
	c := NewIdempotencyCache()
	body := []byte(`{"code":{"main.go":"package main"}}`)
	c.Store("sess-1", "key-A", body, http.StatusOK, []byte(`{"run":"r1"}`))

	entry, ok, conflict := c.Lookup("sess-1", "key-A", body)
	if !ok || conflict {
		t.Fatalf("expected hit; ok=%v conflict=%v", ok, conflict)
	}
	if string(entry.payload) != `{"run":"r1"}` {
		t.Errorf("payload = %q", entry.payload)
	}
	if entry.statusCode != http.StatusOK {
		t.Errorf("statusCode = %d", entry.statusCode)
	}
}

func TestIdempotencyCache_BodyMismatchIsConflict(t *testing.T) {
	c := NewIdempotencyCache()
	c.Store("sess-1", "key-A", []byte(`{"x":1}`), 200, []byte(`{}`))

	if _, ok, conflict := c.Lookup("sess-1", "key-A", []byte(`{"x":2}`)); ok || !conflict {
		t.Errorf("different body must conflict; ok=%v conflict=%v", ok, conflict)
	}
}

func TestIdempotencyCache_SessionScoped(t *testing.T) {
	c := NewIdempotencyCache()
	body := []byte(`{}`)
	c.Store("sess-1", "key-A", body, 200, []byte(`{}`))
	if _, ok, _ := c.Lookup("sess-2", "key-A", body); ok {
		t.Error("entry must not leak across sessions")
	}
}

func TestIdempotencyCache_Expiry(t *testing.T) {
	c := NewIdempotencyCache()
	c.entries[cacheKey("s", "k")] = idempotencyEntry{
		bodyHash:  hashBody([]byte("b")),
		expiresAt: time.Now().Add(-1 * time.Second),
		payload:   []byte("expired"),
	}
	if _, ok, _ := c.Lookup("s", "k", []byte("b")); ok {
		t.Error("expired entry must not return hit")
	}
}

func TestIdempotencyCache_Sweep(t *testing.T) {
	c := NewIdempotencyCache()
	c.entries[cacheKey("s", "fresh")] = idempotencyEntry{expiresAt: time.Now().Add(time.Minute)}
	c.entries[cacheKey("s", "stale1")] = idempotencyEntry{expiresAt: time.Now().Add(-1 * time.Second)}
	c.entries[cacheKey("s", "stale2")] = idempotencyEntry{expiresAt: time.Now().Add(-2 * time.Minute)}

	removed := c.Sweep()
	if removed != 2 {
		t.Errorf("Sweep removed %d, want 2", removed)
	}
	if len(c.entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(c.entries))
	}
}

func TestCapturingResponseWriter_RecordsBodyAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	cap := newCapturingResponseWriter(rec)

	cap.WriteHeader(http.StatusCreated)
	cap.Write([]byte(`{"hello":"world"}`))

	if cap.statusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", cap.statusCode)
	}
	if got := cap.body.String(); got != `{"hello":"world"}` {
		t.Errorf("body = %q", got)
	}
	// Underlying recorder also receives the bytes.
	if got := rec.Body.String(); got != `{"hello":"world"}` {
		t.Errorf("recorder body = %q", got)
	}
}

func TestCapturingResponseWriter_ImplicitOK(t *testing.T) {
	rec := httptest.NewRecorder()
	cap := newCapturingResponseWriter(rec)

	// Write without calling WriteHeader → defaults to 200.
	cap.Write([]byte("ok"))

	if cap.statusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", cap.statusCode)
	}
}

func TestIdempotencyHeaderConstant(t *testing.T) {
	if IdempotencyHeader != "Idempotency-Key" {
		t.Errorf("IdempotencyHeader = %q", IdempotencyHeader)
	}
}
