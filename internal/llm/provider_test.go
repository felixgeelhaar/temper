package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockProvider is a test implementation of Provider
type mockProvider struct {
	name       string
	streaming  bool
	response   *Response
	streamResp []StreamChunk
	err        error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Generate(ctx context.Context, req *Request) (*Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockProvider) GenerateStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		for _, chunk := range m.streamResp {
			ch <- chunk
		}
	}()
	return ch, nil
}

func (m *mockProvider) SupportsStreaming() bool {
	return m.streaming
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.providers == nil {
		t.Error("providers map should not be nil")
	}
	if r.defaultP != "" {
		t.Error("default provider should be empty initially")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "test"}

	r.Register("test", p)

	got, err := r.Get("test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != p {
		t.Error("Get() returned different provider")
	}
}

func TestRegistry_SetDefault(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "test"}

	// Set default before registering should fail
	err := r.SetDefault("test")
	if err == nil {
		t.Error("SetDefault() should fail for non-existent provider")
	}

	// Register and set default
	r.Register("test", p)
	err = r.SetDefault("test")
	if err != nil {
		t.Errorf("SetDefault() error = %v", err)
	}

	// Verify default
	got, err := r.Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	if got != p {
		t.Error("Default() returned wrong provider")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "test"}
	r.Register("test", p)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"existing provider", "test", false},
		{"non-existing provider", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Get(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_Default(t *testing.T) {
	r := NewRegistry()

	// No default set
	_, err := r.Default()
	if err != ErrNoDefaultProvider {
		t.Errorf("Default() error = %v, want ErrNoDefaultProvider", err)
	}

	// Set and get default
	p := &mockProvider{name: "test"}
	r.Register("test", p)
	r.SetDefault("test")

	got, err := r.Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("Default().Name() = %v, want test", got.Name())
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	// Empty registry
	if len(r.List()) != 0 {
		t.Error("List() should return empty for new registry")
	}

	// Add providers
	r.Register("a", &mockProvider{name: "a"})
	r.Register("b", &mockProvider{name: "b"})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d items, want 2", len(list))
	}

	// Check both are present (order not guaranteed)
	found := make(map[string]bool)
	for _, name := range list {
		found[name] = true
	}
	if !found["a"] || !found["b"] {
		t.Error("List() missing expected providers")
	}
}

func TestRegistry_DefaultName(t *testing.T) {
	r := NewRegistry()

	// No default
	if r.DefaultName() != "" {
		t.Error("DefaultName() should be empty initially")
	}

	// Set default
	r.Register("test", &mockProvider{name: "test"})
	r.SetDefault("test")

	if r.DefaultName() != "test" {
		t.Errorf("DefaultName() = %v, want test", r.DefaultName())
	}
}

func TestRequest_Fields(t *testing.T) {
	req := Request{
		Model:       "gpt-4",
		MaxTokens:   1000,
		Temperature: 0.7,
		StopSeqs:    []string{"END"},
		System:      "You are helpful",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
	}

	if req.Model != "gpt-4" {
		t.Errorf("Model = %v, want gpt-4", req.Model)
	}
	if req.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %v, want 1000", req.MaxTokens)
	}
	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", req.Temperature)
	}
	if len(req.Messages) != 1 {
		t.Errorf("Messages len = %v, want 1", len(req.Messages))
	}
}

func TestResponse_Fields(t *testing.T) {
	resp := Response{
		Content:      "Hello, world!",
		FinishReason: "stop",
		Usage: Usage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	if resp.Content != "Hello, world!" {
		t.Errorf("Content = %v, want Hello, world!", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %v, want stop", resp.FinishReason)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("Usage.InputTokens = %v, want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 20 {
		t.Errorf("Usage.OutputTokens = %v, want 20", resp.Usage.OutputTokens)
	}
}

func TestRole_Constants(t *testing.T) {
	if RoleSystem != "system" {
		t.Errorf("RoleSystem = %v, want system", RoleSystem)
	}
	if RoleUser != "user" {
		t.Errorf("RoleUser = %v, want user", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %v, want assistant", RoleAssistant)
	}
}

func TestStreamChunk_Fields(t *testing.T) {
	chunk := StreamChunk{
		Content: "partial",
		Done:    false,
		Error:   nil,
	}

	if chunk.Content != "partial" {
		t.Errorf("Content = %v, want partial", chunk.Content)
	}
	if chunk.Done {
		t.Error("Done should be false")
	}
	if chunk.Error != nil {
		t.Error("Error should be nil")
	}
}

func TestMockProvider_Generate(t *testing.T) {
	p := &mockProvider{
		name: "test",
		response: &Response{
			Content:      "Hello!",
			FinishReason: "stop",
		},
	}

	resp, err := p.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("Generate().Content = %v, want Hello!", resp.Content)
	}
}

func TestMockProvider_GenerateStream(t *testing.T) {
	p := &mockProvider{
		name:      "test",
		streaming: true,
		streamResp: []StreamChunk{
			{Content: "Hello"},
			{Content: " World"},
			{Done: true},
		},
	}

	ch, err := p.GenerateStream(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("GenerateStream() error = %v", err)
	}

	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 3 {
		t.Errorf("received %d chunks, want 3", len(chunks))
	}
}

func TestRegistry_Concurrency(t *testing.T) {
	r := NewRegistry()
	done := make(chan bool)

	// Concurrent registrations and lookups
	for i := 0; i < 10; i++ {
		go func(n int) {
			name := "provider-" + string(rune('0'+n))
			r.Register(name, &mockProvider{name: name})
			done <- true
		}(i)

		go func() {
			r.List()
			r.DefaultName()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestErrors(t *testing.T) {
	if ErrProviderNotFound.Error() != "provider not found" {
		t.Errorf("ErrProviderNotFound = %v, want 'provider not found'", ErrProviderNotFound)
	}
	if ErrNoDefaultProvider.Error() != "no default provider configured" {
		t.Errorf("ErrNoDefaultProvider = %v, want 'no default provider configured'", ErrNoDefaultProvider)
	}
}

// Tests for ResilientProvider

func TestDefaultResilientConfig(t *testing.T) {
	cfg := DefaultResilientConfig()

	if !cfg.EnableCircuitBreaker {
		t.Error("EnableCircuitBreaker should be true by default")
	}
	if !cfg.EnableRetry {
		t.Error("EnableRetry should be true by default")
	}
	if !cfg.EnableBulkhead {
		t.Error("EnableBulkhead should be true by default")
	}
	if !cfg.EnableRateLimit {
		t.Error("EnableRateLimit should be true by default")
	}
	if cfg.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", cfg.MaxConcurrent)
	}
	if cfg.RatePerSecond != 2 {
		t.Errorf("RatePerSecond = %d, want 2", cfg.RatePerSecond)
	}
}

func TestNewResilientProvider(t *testing.T) {
	p := &mockProvider{name: "test"}
	cfg := DefaultResilientConfig()

	rp := NewResilientProvider(p, cfg)

	if rp == nil {
		t.Fatal("NewResilientProvider returned nil")
	}
	if rp.Name() != "test" {
		t.Errorf("Name() = %v, want test", rp.Name())
	}
	if rp.circuitBreaker == nil {
		t.Error("circuitBreaker should be set")
	}
	if rp.retrier == nil {
		t.Error("retrier should be set")
	}
	if rp.bulkhead == nil {
		t.Error("bulkhead should be set")
	}
	if rp.rateLimit == nil {
		t.Error("rateLimit should be set")
	}
}

func TestNewResilientProvider_NoPatterns(t *testing.T) {
	p := &mockProvider{name: "test"}
	cfg := ResilientConfig{} // All disabled

	rp := NewResilientProvider(p, cfg)

	if rp.circuitBreaker != nil {
		t.Error("circuitBreaker should be nil when disabled")
	}
	if rp.retrier != nil {
		t.Error("retrier should be nil when disabled")
	}
	if rp.bulkhead != nil {
		t.Error("bulkhead should be nil when disabled")
	}
	if rp.rateLimit != nil {
		t.Error("rateLimit should be nil when disabled")
	}
}

func TestResilientProvider_Generate_Success(t *testing.T) {
	p := &mockProvider{
		name: "test",
		response: &Response{
			Content:      "Hello from resilient!",
			FinishReason: "stop",
		},
	}

	// Use minimal config for fast test
	cfg := ResilientConfig{
		EnableRetry:    true,
		EnableBulkhead: true,
		MaxConcurrent:  2,
		RatePerSecond:  10,
	}
	rp := NewResilientProvider(p, cfg)

	resp, err := rp.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "Hello from resilient!" {
		t.Errorf("Content = %v, want Hello from resilient!", resp.Content)
	}
}

func TestResilientProvider_Generate_NoPatterns(t *testing.T) {
	p := &mockProvider{
		name: "test",
		response: &Response{
			Content: "Direct call",
		},
	}

	cfg := ResilientConfig{} // All disabled
	rp := NewResilientProvider(p, cfg)

	resp, err := rp.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "Direct call" {
		t.Errorf("Content = %v, want Direct call", resp.Content)
	}
}

func TestResilientProvider_Generate_OnlyCircuitBreaker(t *testing.T) {
	p := &mockProvider{
		name: "test",
		response: &Response{
			Content: "With CB only",
		},
	}

	cfg := ResilientConfig{
		EnableCircuitBreaker: true,
	}
	rp := NewResilientProvider(p, cfg)

	resp, err := rp.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "With CB only" {
		t.Errorf("Content = %v, want With CB only", resp.Content)
	}
}

func TestResilientProvider_Generate_OnlyRetry(t *testing.T) {
	p := &mockProvider{
		name: "test",
		response: &Response{
			Content: "With retry only",
		},
	}

	cfg := ResilientConfig{
		EnableRetry: true,
	}
	rp := NewResilientProvider(p, cfg)

	resp, err := rp.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "With retry only" {
		t.Errorf("Content = %v, want With retry only", resp.Content)
	}
}

func TestResilientProvider_SupportsStreaming(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
	}{
		{"supports streaming", true},
		{"no streaming", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &mockProvider{name: "test", streaming: tt.streaming}
			rp := NewResilientProvider(p, ResilientConfig{})

			if rp.SupportsStreaming() != tt.streaming {
				t.Errorf("SupportsStreaming() = %v, want %v", rp.SupportsStreaming(), tt.streaming)
			}
		})
	}
}

func TestResilientProvider_GenerateStream_Success(t *testing.T) {
	p := &mockProvider{
		name:      "test",
		streaming: true,
		streamResp: []StreamChunk{
			{Content: "Hello"},
			{Content: " World"},
			{Done: true},
		},
	}

	cfg := ResilientConfig{
		EnableRateLimit: true,
		RatePerSecond:   10,
	}
	rp := NewResilientProvider(p, cfg)

	ch, err := rp.GenerateStream(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("GenerateStream() error = %v", err)
	}

	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 3 {
		t.Errorf("received %d chunks, want 3", len(chunks))
	}
}

func TestResilientProvider_Close(t *testing.T) {
	p := &mockProvider{name: "test"}
	cfg := ResilientConfig{
		EnableRateLimit: true,
		RatePerSecond:   2,
	}
	rp := NewResilientProvider(p, cfg)

	err := rp.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestResilientProvider_Close_NoRateLimit(t *testing.T) {
	p := &mockProvider{name: "test"}
	cfg := ResilientConfig{} // No rate limit
	rp := NewResilientProvider(p, cfg)

	err := rp.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestIsRetryableHTTPError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"status 429", fmt.Errorf("request failed: status 429"), true},
		{"status 500", fmt.Errorf("internal error: status 500"), true},
		{"status 502", fmt.Errorf("gateway: status 502 bad gateway"), true},
		{"status 503", fmt.Errorf("service unavailable: status 503"), true},
		{"status 504", fmt.Errorf("timeout: status 504"), true},
		{"status 400", fmt.Errorf("bad request: status 400"), false},
		{"status 401", fmt.Errorf("unauthorized: status 401"), false},
		{"generic error", fmt.Errorf("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableHTTPError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableHTTPError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, 0},
		{"status 429", fmt.Errorf("status 429"), 429},
		{"status 500", fmt.Errorf("error: status 500"), 500},
		{"status 502", fmt.Errorf("status 502"), 502},
		{"status 503", fmt.Errorf("status 503"), 503},
		{"status 504", fmt.Errorf("status 504"), 504},
		{"unknown pattern", fmt.Errorf("HTTP 429"), 0}, // pattern doesn't match
		{"no status", fmt.Errorf("connection error"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStatusCode(tt.err)
			if got != tt.want {
				t.Errorf("extractStatusCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"", "a", false},
		{"abc", "", true},
		{"status 429 error", "status 429", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"-"+tt.substr, func(t *testing.T) {
			got := containsString(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("containsString(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestNewLLMHTTPClient(t *testing.T) {
	client := newLLMHTTPClient()

	if client == nil {
		t.Fatal("newLLMHTTPClient() returned nil")
	}
	if client.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", client.Timeout)
	}
	if client.Transport == nil {
		t.Error("Transport should not be nil")
	}
}

func TestResilientProvider_Generate_BulkheadDefaults(t *testing.T) {
	p := &mockProvider{
		name:     "test",
		response: &Response{Content: "ok"},
	}

	// Test default values when MaxConcurrent is 0
	cfg := ResilientConfig{
		EnableBulkhead: true,
		MaxConcurrent:  0, // Should default to 5
	}
	rp := NewResilientProvider(p, cfg)

	if rp.bulkhead == nil {
		t.Error("bulkhead should be created with defaults")
	}

	resp, err := rp.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %v, want ok", resp.Content)
	}
}

func TestResilientProvider_RateLimitDefaults(t *testing.T) {
	p := &mockProvider{
		name:     "test",
		response: &Response{Content: "ok"},
	}

	// Test default values when RatePerSecond is 0
	cfg := ResilientConfig{
		EnableRateLimit: true,
		RatePerSecond:   0, // Should default to 2
	}
	rp := NewResilientProvider(p, cfg)

	if rp.rateLimit == nil {
		t.Error("rateLimit should be created with defaults")
	}

	resp, err := rp.Generate(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %v, want ok", resp.Content)
	}
}

// Tests for ClaudeProvider

func TestNewClaudeProvider_Defaults(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{
		APIKey: "test-key",
	})

	if p == nil {
		t.Fatal("NewClaudeProvider returned nil")
	}
	if p.apiKey != "test-key" {
		t.Errorf("apiKey = %v, want test-key", p.apiKey)
	}
	if p.baseURL != "https://api.anthropic.com" {
		t.Errorf("baseURL = %v, want https://api.anthropic.com", p.baseURL)
	}
	if p.model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v, want claude-sonnet-4-20250514", p.model)
	}
	if p.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewClaudeProvider_CustomConfig(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{
		APIKey:  "custom-key",
		BaseURL: "https://custom.api.com",
		Model:   "claude-3-opus",
	})

	if p.baseURL != "https://custom.api.com" {
		t.Errorf("baseURL = %v, want https://custom.api.com", p.baseURL)
	}
	if p.model != "claude-3-opus" {
		t.Errorf("model = %v, want claude-3-opus", p.model)
	}
}

func TestClaudeProvider_Name(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "test"})
	if p.Name() != "claude" {
		t.Errorf("Name() = %v, want claude", p.Name())
	}
}

func TestClaudeProvider_SupportsStreaming(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "test"})
	if !p.SupportsStreaming() {
		t.Error("SupportsStreaming() should return true")
	}
}

func TestClaudeProvider_BuildRequest(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{
		APIKey: "test",
		Model:  "claude-3-opus",
	})

	tests := []struct {
		name      string
		req       *Request
		stream    bool
		wantModel string
		wantMax   int
	}{
		{
			name: "defaults",
			req: &Request{
				Messages: []Message{
					{Role: RoleUser, Content: "Hello"},
				},
			},
			stream:    false,
			wantModel: "claude-3-opus",
			wantMax:   4096,
		},
		{
			name: "custom model and tokens",
			req: &Request{
				Model:     "custom-model",
				MaxTokens: 1000,
				Messages: []Message{
					{Role: RoleUser, Content: "Hello"},
				},
			},
			stream:    true,
			wantModel: "custom-model",
			wantMax:   1000,
		},
		{
			name: "with system message",
			req: &Request{
				Messages: []Message{
					{Role: RoleSystem, Content: "You are helpful"},
					{Role: RoleUser, Content: "Hello"},
				},
			},
			stream:    false,
			wantModel: "claude-3-opus",
			wantMax:   4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.buildRequest(tt.req, tt.stream)

			if got.Model != tt.wantModel {
				t.Errorf("Model = %v, want %v", got.Model, tt.wantModel)
			}
			if got.MaxTokens != tt.wantMax {
				t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, tt.wantMax)
			}
			if got.Stream != tt.stream {
				t.Errorf("Stream = %v, want %v", got.Stream, tt.stream)
			}
		})
	}
}

func TestClaudeProvider_BuildRequest_SystemExtraction(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "test"})

	req := &Request{
		System: "Default system",
		Messages: []Message{
			{Role: RoleSystem, Content: "Override system"},
			{Role: RoleUser, Content: "Hello"},
		},
	}

	got := p.buildRequest(req, false)

	// System from messages should override
	if got.System != "Override system" {
		t.Errorf("System = %v, want Override system", got.System)
	}

	// System messages should not be in messages array
	for _, m := range got.Messages {
		if m.Role == "system" {
			t.Error("system message should not be in messages array")
		}
	}
}

func TestClaudeProvider_ParseResponse(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "test"})

	resp := &claudeResponse{
		ID:   "msg-123",
		Type: "message",
		Role: "assistant",
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "World!"},
		},
		StopReason: "end_turn",
	}
	resp.Usage.InputTokens = 10
	resp.Usage.OutputTokens = 5

	got := p.parseResponse(resp)

	if got.Content != "Hello World!" {
		t.Errorf("Content = %v, want Hello World!", got.Content)
	}
	if got.FinishReason != "end_turn" {
		t.Errorf("FinishReason = %v, want end_turn", got.FinishReason)
	}
	if got.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %v, want 10", got.Usage.InputTokens)
	}
	if got.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %v, want 5", got.Usage.OutputTokens)
	}
}

func TestClaudeProvider_ParseResponse_EmptyContent(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "test"})

	resp := &claudeResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{},
	}

	got := p.parseResponse(resp)

	if got.Content != "" {
		t.Errorf("Content = %v, want empty", got.Content)
	}
}

// Tests for OllamaProvider

func TestNewOllamaProvider_Defaults(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{})

	if p == nil {
		t.Fatal("NewOllamaProvider returned nil")
	}
	if p.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %v, want http://localhost:11434", p.baseURL)
	}
	if p.model != "llama2" {
		t.Errorf("model = %v, want llama2", p.model)
	}
}

func TestNewOllamaProvider_CustomConfig(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{
		BaseURL: "http://custom:11434",
		Model:   "codellama",
	})

	if p.baseURL != "http://custom:11434" {
		t.Errorf("baseURL = %v, want http://custom:11434", p.baseURL)
	}
	if p.model != "codellama" {
		t.Errorf("model = %v, want codellama", p.model)
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{})
	if p.Name() != "ollama" {
		t.Errorf("Name() = %v, want ollama", p.Name())
	}
}

func TestOllamaProvider_SupportsStreaming(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{})
	if !p.SupportsStreaming() {
		t.Error("SupportsStreaming() should return true")
	}
}

// Tests for OpenAIProvider

func TestNewOpenAIProvider_Defaults(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{
		APIKey: "test-key",
	})

	if p == nil {
		t.Fatal("NewOpenAIProvider returned nil")
	}
	if p.apiKey != "test-key" {
		t.Errorf("apiKey = %v, want test-key", p.apiKey)
	}
	if p.baseURL != "https://api.openai.com" {
		t.Errorf("baseURL = %v, want https://api.openai.com", p.baseURL)
	}
	if p.model != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", p.model)
	}
}

func TestNewOpenAIProvider_CustomConfig(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "custom-key",
		BaseURL: "https://custom.openai.com",
		Model:   "gpt-4-turbo",
	})

	if p.baseURL != "https://custom.openai.com" {
		t.Errorf("baseURL = %v, want https://custom.openai.com", p.baseURL)
	}
	if p.model != "gpt-4-turbo" {
		t.Errorf("model = %v, want gpt-4-turbo", p.model)
	}
}

func TestOpenAIProvider_Name(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test"})
	if p.Name() != "openai" {
		t.Errorf("Name() = %v, want openai", p.Name())
	}
}

func TestOpenAIProvider_SupportsStreaming(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test"})
	if !p.SupportsStreaming() {
		t.Error("SupportsStreaming() should return true")
	}
}

// Tests for OllamaProvider.buildRequest

func TestOllamaProvider_BuildRequest(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{Model: "llama2"})

	tests := []struct {
		name   string
		req    *Request
		stream bool
	}{
		{
			name: "basic request",
			req: &Request{
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: false,
		},
		{
			name: "with system message",
			req: &Request{
				System:   "You are a helpful assistant",
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: false,
		},
		{
			name: "streaming",
			req: &Request{
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: true,
		},
		{
			name: "with options",
			req: &Request{
				Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
				Temperature: 0.7,
				MaxTokens:   100,
				StopSeqs:    []string{"END"},
			},
			stream: false,
		},
		{
			name: "custom model",
			req: &Request{
				Model:    "codellama",
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.buildRequest(tt.req, tt.stream)
			if got == nil {
				t.Fatal("buildRequest returned nil")
			}
			if got.Stream != tt.stream {
				t.Errorf("Stream = %v, want %v", got.Stream, tt.stream)
			}
			if tt.req.Model != "" && got.Model != tt.req.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.req.Model)
			}
			if tt.req.System != "" && len(got.Messages) == 0 {
				t.Error("expected system message in messages")
			}
		})
	}
}

// Tests for OpenAIProvider.buildRequest and parseResponse

func TestOpenAIProvider_BuildRequest(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test", Model: "gpt-4o"})

	tests := []struct {
		name   string
		req    *Request
		stream bool
	}{
		{
			name: "basic request",
			req: &Request{
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: false,
		},
		{
			name: "with system message",
			req: &Request{
				System:   "You are a helpful assistant",
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: false,
		},
		{
			name: "streaming",
			req: &Request{
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			stream: true,
		},
		{
			name: "with options",
			req: &Request{
				Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
				Temperature: 0.7,
				MaxTokens:   100,
				StopSeqs:    []string{"END"},
			},
			stream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.buildRequest(tt.req, tt.stream)
			if got == nil {
				t.Fatal("buildRequest returned nil")
			}
			if got.Stream != tt.stream {
				t.Errorf("Stream = %v, want %v", got.Stream, tt.stream)
			}
			if tt.req.MaxTokens > 0 && got.MaxTokens != tt.req.MaxTokens {
				t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, tt.req.MaxTokens)
			}
		})
	}
}

func TestOpenAIProvider_ParseResponse(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test"})

	resp := &openaiResponse{
		ID: "chatcmpl-test",
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "Hello!"},
				FinishReason: "stop",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		}{PromptTokens: 10, CompletionTokens: 5},
	}

	got := p.parseResponse(resp)

	if got.Content != "Hello!" {
		t.Errorf("Content = %v, want Hello!", got.Content)
	}
	if got.FinishReason != "stop" {
		t.Errorf("FinishReason = %v, want stop", got.FinishReason)
	}
	if got.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %v, want 10", got.Usage.InputTokens)
	}
	if got.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %v, want 5", got.Usage.OutputTokens)
	}
}

func TestOpenAIProvider_ParseResponse_EmptyChoices(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test"})

	resp := &openaiResponse{
		ID: "chatcmpl-test",
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{},
	}

	got := p.parseResponse(resp)

	if got.Content != "" {
		t.Errorf("Content = %v, want empty", got.Content)
	}
}

// HTTP Integration Tests for Claude Provider

func TestClaudeProvider_Generate_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %v, want POST", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Path = %v, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %v, want test-key", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("anthropic-version = %v, want 2023-06-01", r.Header.Get("anthropic-version"))
		}

		resp := claudeResponse{
			ID:   "msg_test",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hello from Claude!"},
			},
			StopReason: "end_turn",
		}
		resp.Usage.InputTokens = 10
		resp.Usage.OutputTokens = 5

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewClaudeProvider(ClaudeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	got, err := p.Generate(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.Content != "Hello from Claude!" {
		t.Errorf("Content = %v, want Hello from Claude!", got.Content)
	}
}

func TestClaudeProvider_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	p := NewClaudeProvider(ClaudeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	_, err := p.Generate(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("Generate() expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code 500, got: %v", err)
	}
}

// HTTP Integration Tests for OpenAI Provider

func TestOpenAIProvider_Generate_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %v, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Path = %v, want /v1/chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %v, want Bearer test-key", r.Header.Get("Authorization"))
		}

		resp := map[string]interface{}{
			"id": "chatcmpl-test",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello from OpenAI!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	got, err := p.Generate(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.Content != "Hello from OpenAI!" {
		t.Errorf("Content = %v, want Hello from OpenAI!", got.Content)
	}
}

func TestOpenAIProvider_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	_, err := p.Generate(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("Generate() expected error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status code 401, got: %v", err)
	}
}

// HTTP Integration Tests for Ollama Provider

func TestOllamaProvider_Generate_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %v, want POST", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("Path = %v, want /api/chat", r.URL.Path)
		}

		resp := map[string]interface{}{
			"model": "llama2",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Hello from Ollama!",
			},
			"done":              true,
			"total_duration":    1000000,
			"eval_count":        5,
			"prompt_eval_count": 10,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	got, err := p.Generate(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.Content != "Hello from Ollama!" {
		t.Errorf("Content = %v, want Hello from Ollama!", got.Content)
	}
}

func TestOllamaProvider_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "model not loaded"}`))
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	_, err := p.Generate(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("Generate() expected error for HTTP 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should contain status code 503, got: %v", err)
	}
}

// Streaming Tests

func TestClaudeProvider_GenerateStream_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		events := []string{
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world!"}}`,
			`data: {"type":"message_stop"}`,
		}

		for _, event := range events {
			fmt.Fprintln(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewClaudeProvider(ClaudeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	ch, err := p.GenerateStream(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("GenerateStream() error = %v", err)
	}

	var content strings.Builder
	for chunk := range ch {
		if chunk.Error != nil {
			t.Errorf("received error chunk: %v", chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}

	if content.String() != "Hello world!" {
		t.Errorf("Content = %v, want Hello world!", content.String())
	}
}

func TestClaudeProvider_GenerateStream_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	p := NewClaudeProvider(ClaudeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	_, err := p.GenerateStream(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("GenerateStream() expected error for HTTP 429")
	}
}

func TestOpenAIProvider_GenerateStream_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		events := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world!"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			fmt.Fprintln(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	ch, err := p.GenerateStream(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("GenerateStream() error = %v", err)
	}

	var content strings.Builder
	for chunk := range ch {
		if chunk.Error != nil {
			t.Errorf("received error chunk: %v", chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}

	if content.String() != "Hello world!" {
		t.Errorf("Content = %v, want Hello world!", content.String())
	}
}

func TestOllamaProvider_GenerateStream_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		events := []string{
			`{"model":"llama2","message":{"content":"Hello"},"done":false}`,
			`{"model":"llama2","message":{"content":" world!"},"done":false}`,
			`{"model":"llama2","message":{"content":""},"done":true}`,
		}

		for _, event := range events {
			fmt.Fprintln(w, event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	ch, err := p.GenerateStream(context.Background(), &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("GenerateStream() error = %v", err)
	}

	var content strings.Builder
	for chunk := range ch {
		if chunk.Error != nil {
			t.Errorf("received error chunk: %v", chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}

	if content.String() != "Hello world!" {
		t.Errorf("Content = %v, want Hello world!", content.String())
	}
}

// Context cancellation tests

func TestClaudeProvider_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	p := NewClaudeProvider(ClaudeConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Generate(ctx, &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("Generate() expected error for cancelled context")
	}
}

func TestOpenAIProvider_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Generate(ctx, &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("Generate() expected error for cancelled context")
	}
}

func TestOllamaProvider_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Generate(ctx, &Request{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("Generate() expected error for cancelled context")
	}
}

// Additional edge case tests

func TestClaudeProvider_SetHeaders(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "my-api-key"})

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.setHeaders(req)

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("x-api-key") != "my-api-key" {
		t.Errorf("x-api-key = %v, want my-api-key", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version = %v, want 2023-06-01", req.Header.Get("anthropic-version"))
	}
}

func TestOpenAIProvider_SetHeaders(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{APIKey: "my-api-key"})

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.setHeaders(req)

	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("Authorization") != "Bearer my-api-key" {
		t.Errorf("Authorization = %v, want Bearer my-api-key", req.Header.Get("Authorization"))
	}
}
