package daemon

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/correlation"
)

// ContextKey is the type for context keys used in this package.
// Kept exported for external test imports; the canonical source for the
// correlation ID is the internal/correlation package.
type ContextKey string

const (
	// CorrelationIDKey is preserved for backwards compatibility with code
	// that referenced the old key. New code should use correlation.FromContext.
	CorrelationIDKey ContextKey = "correlation_id"
	// CorrelationIDHeader is re-exported for tests.
	CorrelationIDHeader = correlation.HeaderName
)

// GetCorrelationID extracts the correlation ID from a context
func GetCorrelationID(ctx context.Context) string {
	return correlation.FromContext(ctx)
}

// correlationIDMiddleware adds or propagates a correlation ID for request tracing
func correlationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get(correlation.HeaderName)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		w.Header().Set(correlation.HeaderName, correlationID)

		ctx := correlation.WithContext(r.Context(), correlationID)

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

// corsMiddleware adds CORS headers for the web dashboard. Only echoes an
// allowlisted origin; previously echoed any Origin verbatim, which paired
// with credentialed requests creates a CSRF surface from any attacker page.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Authorization")
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// authMiddleware enforces a Bearer token on every request except /v1/health
// (which must be reachable for liveness probes). Token comparison uses
// constant-time equality to defeat timing oracles.
func authMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/health" || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				slog.Warn("auth: missing or malformed Authorization header",
					"correlation_id", GetCorrelationID(r.Context()),
					"path", r.URL.Path,
				)
				http.Error(w, `{"error_code":"UNAUTHORIZED","message":"missing bearer token"}`, http.StatusUnauthorized)
				w.Header().Set("Content-Type", "application/json")
				return
			}

			provided := strings.TrimPrefix(header, prefix)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				slog.Warn("auth: invalid bearer token",
					"correlation_id", GetCorrelationID(r.Context()),
					"path", r.URL.Path,
				)
				http.Error(w, `{"error_code":"UNAUTHORIZED","message":"invalid bearer token"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hostGuardMiddleware rejects requests whose Host header is not in the
// allowlist. Defends against DNS-rebinding attacks where a malicious page in
// the user's browser resolves an attacker-controlled domain to 127.0.0.1 and
// then issues credentialed requests to localhost. Browsers send the original
// hostname in the Host header, which we reject.
func hostGuardMiddleware(allowedHosts []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			host := r.Host
			if colon := strings.LastIndex(host, ":"); colon != -1 {
				host = host[:colon]
			}

			for _, allowed := range allowedHosts {
				if host == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			slog.Warn("host guard: rejected request",
				"correlation_id", GetCorrelationID(r.Context()),
				"host", r.Host,
				"path", r.URL.Path,
			)
			http.Error(w, `{"error_code":"FORBIDDEN_HOST","message":"host not allowed"}`, http.StatusForbidden)
		})
	}
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
