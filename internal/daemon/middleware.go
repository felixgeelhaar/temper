package daemon

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ContextKey is the type for context keys used in this package
type ContextKey string

const (
	// CorrelationIDKey is the context key for the correlation ID
	CorrelationIDKey ContextKey = "correlation_id"
	// CorrelationIDHeader is the HTTP header name for correlation ID
	CorrelationIDHeader = "X-Request-ID"
)

// GetCorrelationID extracts the correlation ID from a context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// correlationIDMiddleware adds or propagates a correlation ID for request tracing
func correlationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing correlation ID in header
		correlationID := r.Header.Get(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Add correlation ID to response header
		w.Header().Set(CorrelationIDHeader, correlationID)

		// Add correlation ID to request context
		ctx := context.WithValue(r.Context(), CorrelationIDKey, correlationID)

		// Continue with enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs HTTP requests with timing and status
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Get correlation ID for logging
		correlationID := GetCorrelationID(r.Context())

		// Log based on status code
		if wrapped.statusCode >= 500 {
			slog.Error("request",
				"correlation_id", correlationID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
			)
		} else if wrapped.statusCode >= 400 {
			slog.Warn("request",
				"correlation_id", correlationID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
			)
		} else {
			slog.Debug("request",
				"correlation_id", correlationID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
			)
		}
	})
}

// recoveryMiddleware catches panics and logs them
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				correlationID := GetCorrelationID(r.Context())
				slog.Error("panic recovered",
					"correlation_id", correlationID,
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
