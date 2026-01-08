package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestGetCorrelationID_FromContext(t *testing.T) {
	testID := "test-correlation-id-123"
	ctx := context.WithValue(context.Background(), CorrelationIDKey, testID)

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
