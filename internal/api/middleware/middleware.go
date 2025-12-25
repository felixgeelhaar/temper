package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

// Context keys
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

// RequestID adds a unique request ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// Logger logs HTTP requests with structured logging
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
			"request_id", GetRequestID(r.Context()),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Recovery recovers from panics and returns 500 error
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"request_id", GetRequestID(r.Context()),
					"path", r.URL.Path,
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "internal server error"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// CORS adds CORS headers for development
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get origin from request
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "http://localhost:3000"
		}

		// Development-friendly CORS settings
		// Use specific origin instead of * to allow credentials
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimit implements a simple rate limiter (placeholder for now)
func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	// TODO: Implement proper rate limiting with Redis or in-memory store
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth ensures the request has a valid session
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie or Authorization header
		cookie, err := r.Cookie("session")
		if err != nil {
			// Try Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "authentication required"}`))
				return
			}
			// TODO: Validate bearer token
		} else {
			// TODO: Validate session cookie
			_ = cookie
		}

		// TODO: Add user to context
		// ctx := context.WithValue(r.Context(), UserIDKey, userID)
		// next.ServeHTTP(w, r.WithContext(ctx))

		next.ServeHTTP(w, r)
	})
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	if id, ok := ctx.Value(UserIDKey).(uuid.UUID); ok {
		return id, true
	}
	return uuid.Nil, false
}
