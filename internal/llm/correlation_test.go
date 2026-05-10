package llm

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyCorrelationHeader_Set(t *testing.T) {
	httpReq, _ := http.NewRequest("POST", "https://example", nil)
	applyCorrelationHeader(httpReq, &Request{CorrelationID: "abc-123"})

	if got := httpReq.Header.Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("X-Request-ID = %q, want abc-123", got)
	}
}

func TestApplyCorrelationHeader_EmptyDoesNotSet(t *testing.T) {
	httpReq, _ := http.NewRequest("POST", "https://example", nil)
	applyCorrelationHeader(httpReq, &Request{})

	if got := httpReq.Header.Get("X-Request-ID"); got != "" {
		t.Errorf("X-Request-ID should not be set for empty CorrelationID, got %q", got)
	}
}

func TestApplyCorrelationHeader_NilRequest(t *testing.T) {
	httpReq, _ := http.NewRequest("POST", "https://example", nil)
	applyCorrelationHeader(httpReq, nil)

	if got := httpReq.Header.Get("X-Request-ID"); got != "" {
		t.Errorf("nil request should leave header empty, got %q", got)
	}
}

func TestClaudeProvider_AttachesCorrelationOnHTTP(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Get("X-Request-ID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()

	p := NewClaudeProvider(ClaudeConfig{APIKey: "k", BaseURL: srv.URL})
	_, err := p.Generate(t.Context(), &Request{
		Messages:      []Message{{Role: RoleUser, Content: "hi"}},
		CorrelationID: "trace-42",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if captured != "trace-42" {
		t.Errorf("captured X-Request-ID = %q, want trace-42", captured)
	}
}
