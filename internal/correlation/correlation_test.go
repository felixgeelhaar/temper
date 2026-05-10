package correlation

import (
	"context"
	"testing"
)

func TestFromContext_Empty(t *testing.T) {
	if got := FromContext(context.Background()); got != "" {
		t.Errorf("empty context → got %q, want empty", got)
	}
	if got := FromContext(nil); got != "" {
		t.Errorf("nil context → got %q, want empty", got)
	}
}

func TestWithContext_RoundTrip(t *testing.T) {
	ctx := WithContext(context.Background(), "abc-123")
	if got := FromContext(ctx); got != "abc-123" {
		t.Errorf("got %q, want abc-123", got)
	}
}

func TestWithContext_EmptyIDIsNoop(t *testing.T) {
	parent := WithContext(context.Background(), "kept")
	child := WithContext(parent, "")
	if got := FromContext(child); got != "kept" {
		t.Errorf("empty WithContext should not overwrite, got %q", got)
	}
}

func TestHeaderName(t *testing.T) {
	if HeaderName != "X-Request-ID" {
		t.Errorf("HeaderName = %q, want X-Request-ID", HeaderName)
	}
}
