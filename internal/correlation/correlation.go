// Package correlation provides a single source of truth for the
// correlation-ID context key. Both the daemon's HTTP middleware and the
// LLM providers must agree on the key so a request can be traced from
// editor → daemon → LLM API.
package correlation

import "context"

// Key is the context key used to carry the correlation ID. Defined as a
// distinct unexported type to avoid collisions with other context values.
type contextKey struct{}

var key = contextKey{}

// HeaderName is the HTTP header used to propagate correlation IDs across
// service boundaries.
const HeaderName = "X-Request-ID"

// WithContext returns a new context with the given correlation ID attached.
func WithContext(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, key, id)
}

// FromContext returns the correlation ID from the context, or "" if none
// is set.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}
