package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/correlation"
)

func TestGetCorrelationID_FromContext(t *testing.T) {
	testID := "test-correlation-id-123"
	ctx := correlation.WithContext(context.Background(), testID)

	result := GetCorrelationID(ctx)
	if result != testID {
		t.Errorf("GetCorrelationID() = %q, want %q", result, testID)
	}
}

func TestGetCorrelationID_EmptyContext(t *testing.T) {
	ctx := context.Background()

	result := GetCorrelationID(ctx)
	if result != "" {
		t.Errorf("GetCorrelationID() = %q, want empty string", result)
	}
}

func TestGetCorrelationID_WrongType(t *testing.T) {
	// Store an int instead of string
	ctx := context.WithValue(context.Background(), CorrelationIDKey, 12345)

	result := GetCorrelationID(ctx)
	if result != "" {
		t.Errorf("GetCorrelationID() = %q, want empty string for wrong type", result)
	}
}

func TestCorrelationIDMiddleware_GeneratesID(t *testing.T) {
	var capturedID string
	handler := correlationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetCorrelationID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should have generated a UUID
	if capturedID == "" {
		t.Error("Expected correlation ID to be generated")
	}

	// Verify it's a valid UUID
	if _, err := uuid.Parse(capturedID); err != nil {
		t.Errorf("Generated ID %q is not a valid UUID: %v", capturedID, err)
	}

	// Verify it's in the response header
	responseID := rec.Header().Get(CorrelationIDHeader)
	if responseID != capturedID {
		t.Errorf("Response header ID %q != captured ID %q", responseID, capturedID)
	}
}

func TestCorrelationIDMiddleware_PropagatesExistingID(t *testing.T) {
	existingID := "existing-correlation-id"
	var capturedID string

	handler := correlationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetCorrelationID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(CorrelationIDHeader, existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should use existing ID
	if capturedID != existingID {
		t.Errorf("Captured ID %q != expected %q", capturedID, existingID)
	}

	// Verify it's echoed in the response header
	responseID := rec.Header().Get(CorrelationIDHeader)
	if responseID != existingID {
		t.Errorf("Response header ID %q != expected %q", responseID, existingID)
	}
}

func TestLoggingMiddleware_CapturesStatusCode(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		writeHeader bool
	}{
		{"ok status", http.StatusOK, true},
		{"created status", http.StatusCreated, true},
		{"bad request", http.StatusBadRequest, true},
		{"not found", http.StatusNotFound, true},
		{"internal error", http.StatusInternalServerError, true},
		{"default status (no explicit write)", http.StatusOK, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.writeHeader {
					w.WriteHeader(tt.statusCode)
				}
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.statusCode {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.statusCode)
			}
		})
	}
}

func TestLoggingMiddleware_PassesThroughBody(t *testing.T) {
	expectedBody := "test response body"

	handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Body.String() != expectedBody {
		t.Errorf("Body = %q, want %q", rec.Body.String(), expectedBody)
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Body = %q, want %q", rec.Body.String(), "success")
	}
}

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRecoveryMiddleware_CatchesNilPanic(t *testing.T) {
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	// Go 1.21+ treats panic(nil) as a real panic with error message
	// "panic called with nil argument", so we expect 500
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	rw.WriteHeader(http.StatusCreated)

	if rw.statusCode != http.StatusCreated {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusCreated)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("underlying recorder code = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestMiddlewareChain_Integration(t *testing.T) {
	var capturedCorrelationID string
	var capturedMethod string

	// Create a handler that captures the correlation ID
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCorrelationID = GetCorrelationID(r.Context())
		capturedMethod = r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Build the full middleware chain as in server.go
	handler := correlationIDMiddleware(recoveryMiddleware(loggingMiddleware(innerHandler)))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify correlation ID was generated and propagated
	if capturedCorrelationID == "" {
		t.Error("Correlation ID should have been generated")
	}

	// Verify method was passed through
	if capturedMethod != http.MethodPost {
		t.Errorf("Method = %q, want %q", capturedMethod, http.MethodPost)
	}

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify correlation ID is in response header
	responseID := rec.Header().Get(CorrelationIDHeader)
	if responseID != capturedCorrelationID {
		t.Errorf("Response header ID %q != captured ID %q", responseID, capturedCorrelationID)
	}
}

func TestMiddlewareChain_WithPanic(t *testing.T) {
	var capturedCorrelationID string

	// Create a handler that captures the correlation ID then panics
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCorrelationID = GetCorrelationID(r.Context())
		panic("simulated panic")
	})

	// Build the full middleware chain
	handler := correlationIDMiddleware(recoveryMiddleware(loggingMiddleware(innerHandler)))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(rec, req)

	// Verify correlation ID was generated (happens before panic)
	if capturedCorrelationID == "" {
		t.Error("Correlation ID should have been generated before panic")
	}

	// Verify panic was recovered and error status returned
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestCorsMiddleware_AllowsOriginInAllowlist(t *testing.T) {
	called := false
	mw := corsMiddleware([]string{"http://127.0.0.1:4321"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://127.0.0.1:4321")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called for non-OPTIONS requests")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:4321" {
		t.Error("Access-Control-Allow-Origin header should be set for allowlisted origin")
	}
}

func TestCorsMiddleware_RejectsUnknownOrigin(t *testing.T) {
	mw := corsMiddleware([]string{"http://127.0.0.1:4321"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Access-Control-Allow-Origin must not echo non-allowlisted origins")
	}
}

func TestCorsMiddleware_Options(t *testing.T) {
	called := false
	mw := corsMiddleware([]string{"http://127.0.0.1:4321"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://127.0.0.1:4321")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Error("handler should not be called for OPTIONS requests")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}
}

func TestCorrelationIDHeader_Constant(t *testing.T) {
	if CorrelationIDHeader != "X-Request-ID" {
		t.Errorf("CorrelationIDHeader = %q, want %q", CorrelationIDHeader, "X-Request-ID")
	}
}

func TestCorrelationIDKey_Constant(t *testing.T) {
	expected := ContextKey("correlation_id")
	if CorrelationIDKey != expected {
		t.Errorf("CorrelationIDKey = %v, want %v", CorrelationIDKey, expected)
	}
}

func TestAuthMiddleware_RejectsMissingHeader(t *testing.T) {
	mw := authMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when token is missing")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	mw := authMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_RejectsMalformedHeader(t *testing.T) {
	mw := authMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for malformed header")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "secret-token") // No "Bearer " prefix
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_AcceptsValidToken(t *testing.T) {
	called := false
	mw := authMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called when token matches")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_BypassesHealth(t *testing.T) {
	called := false
	mw := authMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("/v1/health must be reachable without a token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHostGuardMiddleware_AllowsAllowlisted(t *testing.T) {
	called := false
	mw := hostGuardMiddleware([]string{"127.0.0.1"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Host = "127.0.0.1:7432"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called for allowlisted host")
	}
}

func TestHostGuardMiddleware_RejectsAttackerHost(t *testing.T) {
	mw := hostGuardMiddleware([]string{"127.0.0.1"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called for non-allowlisted Host")
	}))

	// Simulates DNS-rebinding: attacker's domain resolves to 127.0.0.1 but
	// the browser sends the original hostname in Host.
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Host = "evil.example:7432"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHostGuardMiddleware_BypassesHealth(t *testing.T) {
	called := false
	mw := hostGuardMiddleware([]string{"127.0.0.1"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "anything.example"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("/v1/health must be reachable from any host (liveness probes)")
	}
}

func TestHostGuardMiddleware_RejectsLocalhostByDefault(t *testing.T) {
	// Per security review, only 127.0.0.1 is allowlisted; "localhost" is
	// rejected because resolver mapping cannot be trusted.
	mw := hostGuardMiddleware([]string{"127.0.0.1"})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called for non-allowlisted localhost")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Host = "localhost:7432"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
