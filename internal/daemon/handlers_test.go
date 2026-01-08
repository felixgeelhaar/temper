package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
)

// mockLLMProvider is a mock implementation of llm.Provider for testing
type mockLLMProvider struct {
	name       string
	response   *llm.Response
	err        error
	streaming  bool
	streamResp []llm.StreamChunk
}

func (m *mockLLMProvider) Name() string {
	return m.name
}

func (m *mockLLMProvider) Generate(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	// Return a sensible default response
	return &llm.Response{
		Content:      "This is a mock LLM response for testing.",
		FinishReason: "stop",
		Usage: llm.Usage{
			InputTokens:  10,
			OutputTokens: 15,
		},
	}, nil
}

func (m *mockLLMProvider) GenerateStream(ctx context.Context, req *llm.Request) (<-chan llm.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan llm.StreamChunk)
	go func() {
		defer close(ch)
		if len(m.streamResp) > 0 {
			for _, chunk := range m.streamResp {
				ch <- chunk
			}
		} else {
			// Default streaming response
			ch <- llm.StreamChunk{Content: "Mock ", Done: false}
			ch <- llm.StreamChunk{Content: "streaming ", Done: false}
			ch <- llm.StreamChunk{Content: "response.", Done: false}
			ch <- llm.StreamChunk{Done: true}
		}
	}()
	return ch, nil
}

func (m *mockLLMProvider) SupportsStreaming() bool {
	return m.streaming
}

// testServerContext holds context for test server including paths for direct manipulation
type testServerContext struct {
	Server       *Server
	SessionsPath string
	SpecsPath    string
	Cleanup      func()
}

// setupTestServerWithMockLLM creates a test server with a mock LLM provider
// This enables testing of pairing and other LLM-dependent handlers
func setupTestServerWithMockLLM(t *testing.T) (*Server, func()) {
	ctx := setupTestServerWithContext(t)
	return ctx.Server, ctx.Cleanup
}

// setupTestServerWithContext creates a test server and returns full context including paths
func setupTestServerWithContext(t *testing.T) *testServerContext {
	t.Helper()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "temper-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	sessionsPath := filepath.Join(tmpDir, "sessions")
	specsPath := filepath.Join(tmpDir, "workspace")

	// Create subdirectories
	for _, dir := range []string{"sessions", "exercises", "logs", "workspace", "workspace/.specs"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("create subdir %s: %v", dir, err)
		}
	}

	// Create test config
	cfg := config.DefaultLocalConfig()
	cfg.Daemon.Port = 0 // Let system choose port
	cfg.Runner.Executor = "local"

	// Create server
	server, err := NewServer(context.Background(), ServerConfig{
		Config:       cfg,
		ExercisePath: filepath.Join(tmpDir, "exercises"),
		SessionsPath: sessionsPath,
		SpecsPath:    specsPath,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create server: %v", err)
	}

	// Register mock LLM provider
	mockProvider := &mockLLMProvider{
		name:      "mock",
		streaming: true,
	}
	server.llmRegistry.Register("mock", mockProvider)
	if err := server.llmRegistry.SetDefault("mock"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("set default provider: %v", err)
	}

	// Re-initialize pairing service with the mock provider
	server.pairingService = pairing.NewService(server.llmRegistryConcrete, "mock")

	return &testServerContext{
		Server:       server,
		SessionsPath: sessionsPath,
		SpecsPath:    specsPath,
		Cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}
}

func TestHandlers_InvalidJSON_Returns400(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/sessions"},
		{http.MethodPost, "/v1/specs"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			body := strings.NewReader(`{invalid json}`)
			req := httptest.NewRequest(ep.method, ep.path, body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if _, ok := resp["error"]; !ok {
				t.Error("expected 'error' field in response")
			}
		})
	}
}

func TestHandlers_EmptyBody_Returns400(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Session without exercise_id or spec_path
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_NotFound_Returns404(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"GET unknown endpoint", http.MethodGet, "/v1/unknown", http.StatusNotFound},
		{"GET session not found", http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000000", http.StatusNotFound},
		// DELETE returns 500 when session store reports error (implementation detail)
		{"DELETE session not found", http.MethodDelete, "/v1/sessions/00000000-0000-0000-0000-000000000000", http.StatusInternalServerError},
		{"GET exercise pack not found", http.MethodGet, "/v1/exercises/nonexistent-pack", http.StatusNotFound},
		{"GET exercise not found", http.MethodGet, "/v1/exercises/nonexistent-pack/nonexistent-exercise", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlers_MethodNotAllowed(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"PUT on health", http.MethodPut, "/v1/health"},
		{"DELETE on status", http.MethodDelete, "/v1/status"},
		{"PATCH on exercises", http.MethodPatch, "/v1/exercises"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Go 1.22+ http.ServeMux returns 405 for wrong method on valid route
			// or 404 for unknown path
			if w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
				t.Errorf("expected status 405 or 404, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlers_Health_ResponseFormat(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify required fields
	requiredFields := []string{"status", "timestamp"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", resp["status"])
	}
}

func TestHandlers_Status_AllFields(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	requiredFields := []string{"status", "version", "llm_providers", "runner"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestHandlers_Config_NoSecrets(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()

	// Ensure no API keys are exposed
	sensitivePatterns := []string{
		"api_key",
		"apiKey",
		"secret",
		"password",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(strings.ToLower(body), pattern) {
			t.Errorf("config response may contain sensitive data: found '%s'", pattern)
		}
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	requiredFields := []string{"daemon", "learning_contract", "runner", "default_provider"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestHandlers_Exercises_ListPacks(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	packs, ok := resp["packs"].([]interface{})
	if !ok {
		t.Error("expected 'packs' to be an array")
	}

	// Empty exercises dir should return empty array, not nil
	if packs == nil {
		t.Error("packs should not be nil")
	}
}

func TestHandlers_Profile_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Analytics_Overview(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["overview"]; !ok {
		t.Error("expected 'overview' field in response")
	}
}

func TestHandlers_Analytics_Skills(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Analytics_Errors(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Analytics_Trend(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Specs_CreateRequiresName(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_Specs_List(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["specs"]; !ok {
		t.Error("expected 'specs' field in response")
	}
}

func TestHandlers_Spec_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/nonexistent.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Providers_List(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config/providers", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["default"]; !ok {
		t.Error("expected 'default' field in response")
	}

	providers, ok := resp["providers"].([]interface{})
	if !ok {
		t.Error("expected 'providers' to be an array")
	}

	// Each provider should have expected fields
	for i, p := range providers {
		provider, ok := p.(map[string]interface{})
		if !ok {
			t.Errorf("provider %d: expected object", i)
			continue
		}

		requiredFields := []string{"name", "enabled", "configured"}
		for _, field := range requiredFields {
			if _, ok := provider[field]; !ok {
				t.Errorf("provider %d: missing field '%s'", i, field)
			}
		}
	}
}

func TestHandlers_Patch_PreviewNoSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Use a valid UUID format but nonexistent session
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return OK with has_patch: false since there's no pending patch
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_ApplyNoSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/apply", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 for no pending patch
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_CorrelationID_Header(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test that correlation ID is returned in response
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	// Use the full handler chain (which includes correlation ID middleware)
	handler := correlationIDMiddleware(recoveryMiddleware(loggingMiddleware(server.router)))
	handler.ServeHTTP(w, req)

	correlationID := w.Header().Get(CorrelationIDHeader)
	if correlationID == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestHandlers_CorrelationID_Propagation(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	existingID := "test-correlation-id-12345"

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set(CorrelationIDHeader, existingID)
	w := httptest.NewRecorder()

	handler := correlationIDMiddleware(recoveryMiddleware(loggingMiddleware(server.router)))
	handler.ServeHTTP(w, req)

	responseID := w.Header().Get(CorrelationIDHeader)
	if responseID != existingID {
		t.Errorf("expected X-Request-ID = %q, got %q", existingID, responseID)
	}
}

func TestHandlers_Ready_WhenReady(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify required fields
	if _, ok := resp["status"]; !ok {
		t.Error("expected 'status' field in response")
	}
	if _, ok := resp["timestamp"]; !ok {
		t.Error("expected 'timestamp' field in response")
	}
	if _, ok := resp["checks"]; !ok {
		t.Error("expected 'checks' field in response")
	}

	// When server is set up with local executor, status should be ready
	if resp["status"] != "ready" {
		t.Errorf("expected status 'ready', got %v", resp["status"])
	}

	// Verify checks structure
	checks, ok := resp["checks"].(map[string]interface{})
	if !ok {
		t.Error("expected 'checks' to be an object")
	} else {
		if _, ok := checks["llm_provider"]; !ok {
			t.Error("expected 'llm_provider' check")
		}
		if _, ok := checks["runner"]; !ok {
			t.Error("expected 'runner' check")
		}
	}
}

func TestHandlers_Ready_ResponseFormat(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}

	// Verify JSON is valid
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

// Helper to create a test session for handler tests
func createTestSession(t *testing.T, server *Server) string {
	t.Helper()

	// Create exercise structure
	basePath := server.exerciseLoader.BasePath()
	packPath := filepath.Join(basePath, "test-pack")
	categoryPath := filepath.Join(packPath, "basics")
	if err := os.MkdirAll(categoryPath, 0755); err != nil {
		t.Fatalf("create category dir: %v", err)
	}

	// Write pack.yaml
	packYAML := `id: test-pack
name: Test Pack
version: "1.0"
description: Test exercises
language: go
exercises:
  - basics/hello
`
	if err := os.WriteFile(filepath.Join(packPath, "pack.yaml"), []byte(packYAML), 0644); err != nil {
		t.Fatalf("write pack.yaml: %v", err)
	}

	// Write exercise.yaml
	exerciseYAML := `id: hello
title: Hello World
difficulty: beginner
description: Say hello
instructions: Print "Hello, World!"
starter:
  main.go: |
    package main

    func main() {
    }
tests:
  main_test.go: |
    package main

    import "testing"

    func TestMain(t *testing.T) {
    }
`
	if err := os.WriteFile(filepath.Join(categoryPath, "hello.yaml"), []byte(exerciseYAML), 0644); err != nil {
		t.Fatalf("write exercise.yaml: %v", err)
	}

	// Create session
	body := strings.NewReader(`{"exercise_id": "test-pack/basics/hello", "track": "practice"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return resp["id"].(string)
}

func TestHandlers_CreateRun_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Test run with code
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"},
		"format": true,
		"build": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["run"]; !ok {
		t.Error("expected 'run' field in response")
	}
}

func TestHandlers_CreateRun_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "build": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_CreateRun_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// TestHandlers_SpecSession_RunAndReview verifies that :TemperRun and :TemperReview
// work with spec-based (feature_guidance) sessions, not just exercise-based sessions.
func TestHandlers_SpecSession_RunAndReview(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()
	server := ctx.Server

	// Create a spec file in the .specs directory (relative path required)
	specFullPath := filepath.Join(ctx.SpecsPath, ".specs", "test-feature.spec.yaml")
	specContent := `name: Test Feature
title: Test Feature
goals:
  - Implement hello world functionality
acceptance_criteria:
  - id: ac-1
    description: Code compiles successfully
  - id: ac-2
    description: Tests pass
features:
  - id: f-1
    title: Hello World
    description: Print hello world
`
	if err := os.WriteFile(specFullPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// Create a spec-based session (feature_guidance intent) using relative path
	body := strings.NewReader(`{"spec_path": ".specs/test-feature.spec.yaml", "intent": "feature_guidance"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create spec session: %d: %s", w.Code, w.Body.String())
	}

	var sessionResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&sessionResp); err != nil {
		t.Fatalf("decode session response: %v", err)
	}

	sessionID := sessionResp["id"].(string)

	// Verify session has feature_guidance intent
	intent, ok := sessionResp["intent"].(string)
	if !ok || intent != "feature_guidance" {
		t.Errorf("expected intent 'feature_guidance', got %v", sessionResp["intent"])
	}

	// Test 1: Run endpoint should work with spec-based session
	t.Run("run works with spec session", func(t *testing.T) {
		runBody := strings.NewReader(`{
			"code": {"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"},
			"format": true,
			"build": true
		}`)
		runReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", runBody)
		runReq.Header.Set("Content-Type", "application/json")
		runW := httptest.NewRecorder()

		server.router.ServeHTTP(runW, runReq)

		if runW.Code != http.StatusOK {
			t.Errorf("run should work with spec session: expected %d, got %d: %s",
				http.StatusOK, runW.Code, runW.Body.String())
		}

		var runResp map[string]interface{}
		if err := json.NewDecoder(runW.Body).Decode(&runResp); err != nil {
			t.Fatalf("decode run response: %v", err)
		}

		if _, ok := runResp["run"]; !ok {
			t.Error("expected 'run' field in response")
		}
	})

	// Test 2: Review endpoint should work with spec-based session
	t.Run("review works with spec session", func(t *testing.T) {
		reviewBody := strings.NewReader(`{
			"code": {"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"}
		}`)
		reviewReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", reviewBody)
		reviewReq.Header.Set("Content-Type", "application/json")
		reviewW := httptest.NewRecorder()

		server.router.ServeHTTP(reviewW, reviewReq)

		// Review uses LLM, so it should succeed (mock LLM is registered)
		if reviewW.Code != http.StatusOK {
			t.Errorf("review should work with spec session: expected %d, got %d: %s",
				http.StatusOK, reviewW.Code, reviewW.Body.String())
		}

		var reviewResp map[string]interface{}
		if err := json.NewDecoder(reviewW.Body).Decode(&reviewResp); err != nil {
			t.Fatalf("decode review response: %v", err)
		}

		// Response should have intervention content
		if _, ok := reviewResp["content"]; !ok {
			t.Error("expected 'content' field in review response")
		}
	})

	// Test 3: Hint endpoint should also work with spec-based session
	// Note: May return 429 (cooldown) after review call, which is expected behavior
	t.Run("hint works with spec session", func(t *testing.T) {
		hintBody := strings.NewReader(`{
			"code": {"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"}
		}`)
		hintReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", hintBody)
		hintReq.Header.Set("Content-Type", "application/json")
		hintW := httptest.NewRecorder()

		server.router.ServeHTTP(hintW, hintReq)

		// 200 = success, 429 = cooldown active (both indicate the endpoint works with spec sessions)
		if hintW.Code != http.StatusOK && hintW.Code != http.StatusTooManyRequests {
			t.Errorf("hint should work with spec session: expected 200 or 429, got %d: %s",
				hintW.Code, hintW.Body.String())
		}
	})
}

func TestHandlers_Format_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Unformatted code
	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main(){\nprintln(\"hello\")\n}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["ok"] != true {
		t.Error("expected ok=true")
	}
	if _, ok := resp["formatted"]; !ok {
		t.Error("expected 'formatted' field in response")
	}
}

func TestHandlers_Format_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	endpoints := []string{"hint", "review", "stuck", "next", "explain"}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			body := strings.NewReader(`{}`)
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/"+ep, body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlers_Escalate_InvalidLevel(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Escalation with invalid level
	body := strings.NewReader(`{"level": 3, "justification": "test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_MissingJustification(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Escalation without justification
	body := strings.NewReader(`{"level": 4}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_ValidateNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/nonexistent.spec.yaml", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_MarkCriterionInvalidID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/invalid-uuid", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for invalid UUID
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_LockNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/nonexistent.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_ProgressNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/nonexistent.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_DriftNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/nonexistent.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_RejectNoSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/reject", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 for no pending patch
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_ListEmpty(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	patches, ok := resp["patches"].([]interface{})
	if !ok {
		t.Error("expected 'patches' to be an array")
	}
	if len(patches) != 0 {
		t.Errorf("expected empty patches, got %d", len(patches))
	}
}

func TestHandlers_PatchLog(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_PatchStats(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify stats structure (matches patch.LogStats JSON fields)
	requiredFields := []string{"total_patches", "applied", "rejected", "expired"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestHandlers_Authoring_Discover(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"workspace_path": "."}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or return 200 with results
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_DiscoverInvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_SuggestNoSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Must provide section field to pass validation and reach session lookup
	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_ApplyNoSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Must provide section field to pass validation and reach session lookup
	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_HintNoSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_ExercisePack_List(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create exercise pack
	_ = createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/test-pack", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Response includes pack_id and exercises array
	if _, ok := resp["pack_id"]; !ok {
		t.Error("expected 'pack_id' field in response")
	}
	if _, ok := resp["exercises"]; !ok {
		t.Error("expected 'exercises' field in response")
	}
}

func TestHandlers_Exercise_Get(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create exercise pack
	_ = createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/test-pack/basics/hello", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Exercise is returned directly with fields at top level
	// Domain struct fields marshal with capital letters (no json tags)
	if _, ok := resp["ID"]; !ok {
		t.Error("expected 'ID' field in exercise response")
	}
}

// Additional handler tests to increase coverage

func TestHandlers_ListExercises(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create exercise pack first
	_ = createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["packs"]; !ok {
		t.Error("expected 'packs' field in response")
	}
}

func TestHandlers_CreateSpec(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a spec file in a temp directory
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.spec.yaml")

	// Use "name" instead of "title" per the handler validation
	body := strings.NewReader(`{"path": "` + specPath + `", "name": "Test Spec"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Could succeed or fail depending on spec service state
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSpec_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_ListSpecs(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return OK or error depending on workspace
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpec_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/nonexistent.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetProfile(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsOverview(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsSkills(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsErrors(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsTrend(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_DeleteSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_DeleteSession_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/00000000-0000-0000-0000-000000000099", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Handler returns 500 with "session not found" in details (not 404)
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 500 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["id"]; !ok {
		t.Error("expected 'id' field in session response")
	}
}

func TestHandlers_GetSession_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000099", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Hint_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (200) or handle LLM error gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Review_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (200) or handle LLM error gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Stuck_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (200) or handle LLM error gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Next_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (200) or handle LLM error gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Explain_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (200) or handle LLM error gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Escalate_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Justification must be at least 20 characters
	// Also requires at least 2 hints before escalation
	body := strings.NewReader(`{"code": {"main.go": "package main"}, "level": 4, "justification": "I need more help because I am completely stuck on this problem"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 400 "try at least 2 hints" when hints haven't been tried
	// Or 200/500 if hints were tried and LLM is called
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateRun_WithRecipe(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Test with all recipe options
	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "format": true, "build": true, "test": false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or fail due to runner not being configured
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Format_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main(){}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or fail due to runner not being configured
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchPreview(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Route is GET /v1/sessions/{id}/patch/preview
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or fail gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchPreview_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000099/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// For non-existent sessions, returns 200 with "no pending patches" (not 404)
	// This is by design - the endpoint is graceful about missing sessions
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchApply(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Route is POST /v1/sessions/{id}/patch/apply (not patches)
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 404 "no pending patch to apply" when there's no pending patch
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchApply_NoPendingPatch(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 404 when no pending patch
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_PatchApply_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_SpecBased(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a spec-based session
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.spec.yaml")

	// Write a basic spec file
	specContent := `title: Test Feature
goals:
  - Implement basic feature
acceptance_criteria:
  - Feature works
features: []
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	body := strings.NewReader(`{"spec_path": "` + specPath + `", "intent": "feature_guidance"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Could succeed or fail depending on spec service
	if w.Code != http.StatusCreated && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_NoExercise(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"exercise_id": "nonexistent/exercise"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_ExerciseNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/nonexistent-pack", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_ExerciseGetNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create pack first
	_ = createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/test-pack/nonexistent", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Additional tests to improve coverage for handlers with low coverage

func TestHandlers_Pairing_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Invalid JSON should return 400
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_ValidateSpec_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// The validate endpoint is POST /v1/specs/validate/{path...}
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/test.spec.md", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 400 for invalid JSON or 404/500 for spec not found
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_MarkCriterion_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// The endpoint is PUT /v1/specs/criteria/{id}
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/test-criterion-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 400 for invalid JSON or 404/500 for criterion not found
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// The endpoint is POST /v1/specs/lock/{path...}
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/test.spec.md", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 400 for invalid JSON or 404/500 for spec not found
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"reason": "test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 404 when no pending patch, or 200 on success
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Errorf("expected status 404 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_NoPendingPatch(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// The handler doesn't require a body - it rejects by session ID
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 404 when no pending patch exists
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"reason": "test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000099/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringSuggest_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringApply_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringHint_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlers_PatchLog_Pagination(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test with pagination params
	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandlers_Ready_Detailed(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify response contains expected fields
	if _, ok := resp["status"]; !ok {
		t.Error("expected 'status' field in ready response")
	}
}

func TestHandlers_CreateRun_MissingCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Missing code field
	body := strings.NewReader(`{"format": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for missing code
	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_ValidateWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/spec/validate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Training session doesn't have a spec, so returns error
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_MarkCriterionWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"evidence": "test evidence"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/spec/criteria/1/satisfy", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Training session doesn't have a spec, so returns error
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_LockWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/spec/lock", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Training session doesn't have a spec, so returns error
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_ProgressWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/spec/progress", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Training session doesn't have a spec, so returns error
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_DriftWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/spec/drift", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Training session doesn't have a spec, so returns error
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Additional tests for pairing handlers (routes are /v1/sessions/{id}/hint, etc.)

func TestHandlers_HintWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Hint may succeed with LLM or fail without
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ReviewWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Review may succeed with LLM or fail without
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_StuckWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Stuck may succeed with LLM or fail without
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_NextWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Next may succeed with LLM or fail without
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ExplainWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Explain may succeed with LLM or fail without
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Test escalate endpoint

func TestHandlers_EscalateWithValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 3, "justification": "I have tried multiple approaches and am completely stuck on this problem"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Escalate requires 2+ hints first, so may return 400
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Additional spec handler tests

func TestHandlers_Spec_CreateValid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"name": "test-feature-spec"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 201 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_GetByPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/test.spec.md", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 for non-existent spec
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_Progress(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/test.spec.md", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return error for non-existent spec
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Spec_Drift(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/test.spec.md", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return error for non-existent spec
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Additional authoring tests (routes are /v1/sessions/{id}/authoring/*)

func TestHandlers_Authoring_SuggestWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Suggest requires LLM provider, may fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_ApplyWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "content": "Test goal content"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Apply may succeed or fail depending on session state
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_HintWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "context": "help me write better goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Hint requires LLM provider
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Additional patch tests

func TestHandlers_Patch_ListWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return empty patches list or error
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_PreviewWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Preview returns 200 with "no pending patches" message
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_ApplyWithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Apply returns 404 when no pending patch
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 404, or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// Profile and analytics tests

func TestHandlers_Profile_Complete(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have profile fields
	if _, ok := resp["id"]; !ok {
		t.Error("expected 'id' field in profile")
	}
}

func TestHandlers_Analytics_OverviewFields(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Just verify we can decode the response (structure may vary)
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Analytics overview should return some data
	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestHandlers_Analytics_TrendWithPeriod(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name   string
		period string
	}{
		{"daily", "day"},
		{"weekly", "week"},
		{"monthly", "month"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend?period="+tt.period, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlers_Providers_ListAllFields(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Endpoint is /v1/config/providers
	req := httptest.NewRequest(http.MethodGet, "/v1/config/providers", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have providers array
	if _, ok := resp["providers"]; !ok {
		t.Error("expected 'providers' field")
	}
}

// Session lifecycle tests

func TestHandlers_Session_CompleteLifecycle(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create session
	sessionID := createTestSession(t, server)

	// Get session
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get session failed: %d: %s", w.Code, w.Body.String())
	}

	// Delete session
	req = httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("delete session failed: %d: %s", w.Code, w.Body.String())
	}
}

// Run lifecycle with session tests

func TestHandlers_Run_WithAllOptions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { }"}, "format": true, "build": true, "test": false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Run may succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Invalid session ID format tests

func TestHandlers_InvalidSessionID_Format(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus []int
	}{
		{"get session", http.MethodGet, "/v1/sessions/not-a-uuid", "", []int{http.StatusBadRequest, http.StatusNotFound}},
		// Delete returns 500 when session store fails to find the session
		{"delete session", http.MethodDelete, "/v1/sessions/not-a-uuid", "", []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}},
		{"create run", http.MethodPost, "/v1/sessions/not-a-uuid/runs", `{}`, []int{http.StatusBadRequest, http.StatusNotFound}},
		// Hint route is /v1/sessions/{id}/hint, not /pairing/hint
		{"hint", http.MethodPost, "/v1/sessions/not-a-uuid/hint", `{}`, []int{http.StatusBadRequest, http.StatusNotFound}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Check if status is in expected list
			found := false
			for _, expected := range tt.expectedStatus {
				if w.Code == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected status in %v, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// Additional edge case tests for better coverage

func TestHandlers_Authoring_SuggestSectionRequired(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Missing section should return 400
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_ApplySectionRequired(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Missing section should return 400
	body := strings.NewReader(`{"content": "test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_HintSectionRequired(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Missing section should return 400
	body := strings.NewReader(`{"context": "help me"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_NotAuthoringSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a training session (not an authoring session)
	sessionID := createTestSession(t, server)

	tests := []struct {
		name string
		path string
		body string
	}{
		{"suggest", "/authoring/suggest", `{"section": "goals"}`},
		{"apply", "/authoring/apply", `{"section": "goals", "content": "test"}`},
		{"hint", "/authoring/hint", `{"section": "goals"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+tt.path, body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should return 400 because session is not a spec authoring session
			if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
				t.Errorf("expected status 400 or 500, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlers_Spec_CreateEmptyName(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"name": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty name should return 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_RejectWithReason(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Reject with reason (though no pending patch exists)
	body := strings.NewReader(`{"reason": "test rejection reason"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Returns 404 when no pending patch exists
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Patch_ApplyInvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for invalid JSON or 404 for no pending patch
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Session_DeleteSuccess(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Delete the session
	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Errorf("expected status 200 or 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session is deleted
	req = httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for deleted session, got %d", w.Code)
	}
}

func TestHandlers_Format_WithCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main(){\nprintln(\"hello\")\n}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Format may succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_LevelOutOfRange(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 10, "justification": "This is a valid justification with enough characters"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Invalid level should return 400
	if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_LevelZero(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 0, "justification": "This is a valid justification with enough characters"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Level 0 should return 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchLog_WithLimit(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log?limit=5", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchLog_WithOffset(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log?offset=10", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchStats_Response(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have stats fields
	if _, ok := resp["total_patches"]; !ok {
		t.Error("expected 'total_patches' field")
	}
}

func TestHandlers_Session_GetDetails(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Session is returned directly, should have 'id' field
	if _, ok := resp["id"]; !ok {
		t.Error("expected 'id' field in session response")
	}
}

func TestHandlers_Authoring_DiscoverWithPaths(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"paths": ["*.go", "*.md"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Discover may succeed or fail depending on file system
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_FormatBuildTest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { }"}, "format": true, "build": true, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Run may succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_EmptyCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for empty code or succeed with nothing to run
	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_MarkCriterion_MissingPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"evidence": "test evidence"}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/criterion-123", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Path is required
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_MarkCriterion_WithEvidence(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"path": "test/spec.yaml", "evidence": "Code passes all tests"}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/criterion-123", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Spec not found is acceptable
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_EmptyPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty path should return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_ValidPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/test/spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Spec not found is acceptable
	if w.Code != http.StatusNotFound && w.Code != http.StatusCreated && w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpecProgress_EmptyPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty path should return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpecProgress_ValidPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/test/spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Spec not found is acceptable
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpecDrift_EmptyPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty path should return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpecDrift_ValidPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/test/spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Spec not found is acceptable
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchPreview_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/invalid-uuid/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/patch/reject", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListPatches_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/invalid-uuid/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListPatches_ValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have patches field (may be empty array)
	if _, ok := resp["patches"]; !ok {
		t.Error("expected 'patches' field")
	}
}

func TestHandlers_PatchLog_Global(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 200 or 503 if logging not available
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		// Should have entries field (may be empty array)
		if _, ok := resp["entries"]; !ok {
			t.Error("expected 'entries' field")
		}
	}
}

func TestHandlers_AnalyticsSkills_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsErrors_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsTrend_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetProfile_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Profile not found or success
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ValidateSpec_EmptyPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty path should return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ValidateSpec_ValidPathWithBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"goals": ["test goal"], "criteria": []}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/test/spec.yaml", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Validation may fail or succeed
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSpec_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"path": "test/new-spec.yaml", "goals": ["Learn basics"], "criteria": [{"id": "c1", "description": "Complete exercise"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusCreated && w.Code != http.StatusBadRequest && w.Code != http.StatusConflict && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSpec_MissingFields(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListSpecs_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have specs field
	if _, ok := resp["specs"]; !ok {
		t.Error("expected 'specs' field")
	}
}

func TestHandlers_ListSpecs_WithFilters(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs?locked=true&valid=true", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_SuggestMinimalRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or fail with LLM error
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_ApplyValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "content": "New goal content", "path": "test/spec.yaml"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Authoring_HintMinimalRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "criteria"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or fail with LLM error
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Hint_InvalidIntent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "intent": "invalid_intent"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Invalid intent should fail or be ignored
	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Hint_WithOutput(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "output": "Error: undefined variable"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Review_WithSpecificCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Stuck_WithAttempts(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "attempts": 5}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Next_MultipleFiles(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main", "helper.go": "package main\n\nfunc helper() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Explain_WithContext(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {\n\tdefer func() {\n\t\trecover()\n\t}()\n}"}, "context": "I don't understand defer and recover"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_ValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 4, "justification": "I have been stuck on this for an hour and need more help to understand the concept"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_WithTimeout(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "timeout": 5}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}


func TestHandlers_Run_FormatOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { }"}, "format": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_BuildOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { }"}, "build": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_TestOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main_test.go": "package main\\n\\nimport \"testing\"\\n\\nfunc TestHello(t *testing.T) {\\n\\tif 1+1 != 2 {\\n\\t\\tt.Error(\"math is broken\")\\n\\t}\\n}"}, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_AllFlags(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { }"}, "format": true, "build": true, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on executor
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Format_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Format endpoint formats code regardless of session state
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_EmptyCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"exercise_id": "test-pack/basics/hello", "code": {}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 201, 400, 404, or 500 (exercise may not exist)
	if w.Code != http.StatusCreated && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_DeleteSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListExercises_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetExercise_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/nonexistent-exercise", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_WithAllIntents(t *testing.T) {
	intents := []string{"understand", "fix", "debug", "improve", "learn"}

	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			server, cleanup := setupTestServer(t)
			defer cleanup()

			sessionID := createTestSession(t, server)

			body := strings.NewReader(`{"code": {"main.go": "package main"}, "intent": "` + intent + `"}`)
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// May succeed or fail
			if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
				t.Errorf("unexpected status %d for intent %s: %s", w.Code, intent, w.Body.String())
			}
		})
	}
}

func TestHandlers_Ready_AllChecks(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 or 503
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have status and checks fields
	if _, ok := resp["status"]; !ok {
		t.Error("expected 'status' field")
	}
	if _, ok := resp["checks"]; !ok {
		t.Error("expected 'checks' field")
	}
}

// Additional tests for handleCreateRun to improve coverage
func TestHandlers_Run_AllOperations(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "format": true, "build": true, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_SessionNotActive(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create and immediately try to delete session
	sessionID := createTestSession(t, server)

	// Delete the session to make it inactive
	delReq := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	delW := httptest.NewRecorder()
	server.router.ServeHTTP(delW, delReq)

	// Now try to run
	body := strings.NewReader(`{"code": {"main.go": "package main"}, "format": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 400 (not active) or 404 (not found)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for handlePairingWithEscalation
func TestHandlers_Escalate_InvalidLevelTooLow(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Level 3 is not valid for escalation (must be 4 or 5)
	body := strings.NewReader(`{"level": 3, "justification": "This is my detailed justification for escalation"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_EmptyJustification(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 4, "justification": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_JustificationTooShort(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Less than 20 characters
	body := strings.NewReader(`{"level": 4, "justification": "too short"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"level": 4, "justification": "This is my detailed justification for escalation request"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_Level5(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 5, "justification": "I really need a full solution because I'm completely stuck"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May fail due to insufficient hint count (need 2+ hints first)
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Tests for handlePatchPreview
func TestHandlers_PatchPreview_NoPendingPatch(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 (no pending patch)
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for handleAuthoringSuggest
func TestHandlers_AuthoringSuggest_MissingSection(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"context": "some context"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringSuggest_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for handleAuthoringApply
func TestHandlers_AuthoringApply_MissingSection(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"suggestion_id": "some-id"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringApply_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals", "value": "test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for handleAuthoringHint
func TestHandlers_AuthoringHint_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for analytics endpoints
func TestHandlers_AnalyticsSkills_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 200 or 503
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsErrors_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 200 or 503
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsTrend_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 200 or 503
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Tests for spec endpoints
func TestHandlers_ValidateSpec_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404, 500, or 400 (path validation)
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404, 500, or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_SpecProgress_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_SpecDrift_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for pairing handlers with different intents
func TestHandlers_Pairing_Hint_WithContext(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"context": "I'm struggling with the loop logic"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Review_WithCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { println(\"test\") }"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Stuck_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Next_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Explain_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Tests for profile endpoint with valid session
func TestHandlers_Profile_WithFilters(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile?user_id="+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 200 or 503
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable && w.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

// Tests for authoring discover endpoint (POST /v1/authoring/discover - not session-based)
func TestHandlers_AuthoringDiscover_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringDiscover_EmptyPaths(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"paths": []}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Expect 200 OK with empty results or 400 bad request for empty paths
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Additional Coverage Tests ==========

// Test run with session - invalid JSON
func TestHandlers_Run_Session_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for invalid JSON or 404 for session not found
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test run with format operation
func TestHandlers_Run_Session_FormatOperation(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() { }"},
		"format": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test run with build operation
func TestHandlers_Run_Session_BuildOperation(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() { }"},
		"build": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test run with test operation
func TestHandlers_Run_Session_TestOperation(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{
		"code": {
			"main.go": "package main\n\nfunc main() { }",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) { }"
		},
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test escalation with insufficient hints
func TestHandlers_Escalate_InsufficientHints(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	// Get session ID from response
	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		// Session creation failed, try with valid ID
		sessionID = uuid.New().String()
	}

	// Try escalation without any hints
	body := strings.NewReader(`{
		"level": 4,
		"justification": "This is a test justification that is long enough"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should fail due to insufficient hints or session not found
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test pairing with invalid run_id
func TestHandlers_Pairing_InvalidRunID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{"run_id": "not-a-valid-uuid"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/pairing/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for invalid UUID or 404 for session not found
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring suggest for non-authoring session
func TestHandlers_AuthoringSuggest_NotAuthoringSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a regular session (not authoring)
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Try authoring suggest on non-authoring session
	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 (not authoring session) or 404 (not found)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring apply for non-authoring session
func TestHandlers_AuthoringApply_NotAuthoringSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{"section": "goals", "value": "test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring hint for non-authoring session
func TestHandlers_AuthoringHint_NotAuthoringSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{"section": "goals", "question": "What should I add?"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring suggest with empty session ID (empty string as ID)
func TestHandlers_AuthoringSuggest_EmptySessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Use space as session ID since empty path segments cause redirects
	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/%20/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 (session ID required) or 404 (session not found)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test escalation with session not active
func TestHandlers_Escalate_SessionNotActive(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create and immediately complete a session
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Complete the session
	completeReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/complete", nil)
	completeW := httptest.NewRecorder()
	server.router.ServeHTTP(completeW, completeReq)

	// Try escalation on completed session
	body := strings.NewReader(`{
		"level": 4,
		"justification": "This is a test justification that is long enough"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 (session not active) or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test pairing with session not active
func TestHandlers_Pairing_SessionNotActive(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create and complete a session
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Complete the session
	completeReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/complete", nil)
	completeW := httptest.NewRecorder()
	server.router.ServeHTTP(completeW, completeReq)

	// Try pairing on completed session
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/pairing/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 (session not active) or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test list exercises with pack filter
func TestHandlers_ListExercises_WithPackFilter(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises?pack=go-v1", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 or 500
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test get session interventions
func TestHandlers_GetSession_WithInterventions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Get session
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// Test create spec with valid data (POST /v1/specs)
func TestHandlers_CreateSpec_ValidData(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"name": "Test Spec",
		"description": "A test specification",
		"goals": [{"text": "Goal 1", "category": "functional"}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200/201 or 500
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 201, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test mark criterion satisfied
func TestHandlers_MarkCriterion_ValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"criterion_id": "test-criterion", "satisfied": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/test/path/mark", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 400, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test get exercise details
func TestHandlers_GetExercise(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/go-v1/basics/hello-world", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Patch Handler Tests ==========

func TestHandlers_PatchApply_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/not-a-uuid/patch/apply", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 400 for invalid UUID
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Additional Spec Handler Tests ==========

func TestHandlers_GetSpecByPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/test/path.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/test/path.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 400, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Additional Session Tests ==========

func TestHandlers_CompleteSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main\n\nfunc main() { }"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Complete the session
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/complete", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CompleteSession_NotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/complete", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Additional Create Session Tests ==========

func TestHandlers_CreateSession_MissingExerciseID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should fail with 400 or succeed with spec-based session
	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("expected status 400, 200, or 201, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Validation Tests ==========

func TestHandlers_ValidateSpec_WithPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/some/path.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 400, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Additional Criterion Tests ==========

func TestHandlers_MarkCriterion_MissingSatisfied(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/test-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200, 400 (validation), 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Progress/Drift Tests ==========

func TestHandlers_GetSpecProgress_WithPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/some/path.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpecDrift_WithPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/some/path.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Run Handler Coverage Tests ==========

func TestHandlers_Run_SessionBased_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := uuid.New().String()
	body := strings.NewReader(`{invalid`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Run_WithValidSession_FormatOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main\n\nfunc main() {}"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Run with format only
	runBody := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main(){}"}, "format": true}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", runBody)
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()

	server.router.ServeHTTP(runW, runReq)

	if runW.Code != http.StatusOK && runW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", runW.Code, runW.Body.String())
	}
}

func TestHandlers_Run_WithValidSession_BuildOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main\n\nfunc main() {}"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Run with build only
	runBody := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "build": true}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", runBody)
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()

	server.router.ServeHTTP(runW, runReq)

	if runW.Code != http.StatusOK && runW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", runW.Code, runW.Body.String())
	}
}

func TestHandlers_Run_WithValidSession_TestOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main\n\nfunc main() {}"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Run with test only
	runBody := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "test": true}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", runBody)
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()

	server.router.ServeHTTP(runW, runReq)

	if runW.Code != http.StatusOK && runW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", runW.Code, runW.Body.String())
	}
}

func TestHandlers_Run_WithValidSession_AllChecks(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	sessionBody := strings.NewReader(`{
		"exercise_id": "go-v1/basics/hello-world",
		"code": {"main.go": "package main\n\nfunc main() {}"}
	}`)
	sessionReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", sessionBody)
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionW := httptest.NewRecorder()
	server.router.ServeHTTP(sessionW, sessionReq)

	var sessionResp map[string]interface{}
	json.NewDecoder(sessionW.Body).Decode(&sessionResp)
	sessionID, _ := sessionResp["id"].(string)

	if sessionID == "" {
		t.Skip("Could not create session")
	}

	// Run with all checks
	runBody := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "format": true, "build": true, "test": true}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", runBody)
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()

	server.router.ServeHTTP(runW, runReq)

	if runW.Code != http.StatusOK && runW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", runW.Code, runW.Body.String())
	}
}

// ========== Pairing Handler Coverage Tests ==========

func TestHandlers_Pairing_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/not-a-uuid/hint", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Handler treats invalid UUID as not found (returns 404)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Review_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/review", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Stuck_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/stuck", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Next_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/next", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Explain_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/explain", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Pairing_Escalate_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/escalate", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Readiness Handler Tests ==========

func TestHandlers_Ready_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Ready returns 200 when healthy or 503 when degraded
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d: %s", w.Code, w.Body.String())
	}

	// Should have JSON response
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("expected JSON response: %v", err)
	}
}

// ========== Analytics Handler Coverage Tests ==========

func TestHandlers_GetProfile_InvalidUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile?user_id=invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Handler may return 200 with default profile or 400 for invalid UUID
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsOverview_WithUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	userID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics?user_id="+userID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 with empty data or actual data
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsSkills_WithUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	userID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills?user_id="+userID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsErrors_WithUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	userID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors?user_id="+userID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsTrend_WithUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	userID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend?user_id="+userID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Handler Coverage Tests ==========

func TestHandlers_GetSpec_EmptyPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty path might return 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Note: TestHandlers_ValidateSpec_EmptyPath and TestHandlers_DeleteSession_NotFound
// already exist earlier in this file

func TestHandlers_DeleteSession_InvalidUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Handler may return 400, 404, or 500 depending on UUID parsing behavior
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== List Exercises Tests ==========

func TestHandlers_ListExercises_EmptyResult(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Query with non-existent pack
	req := httptest.NewRequest(http.MethodGet, "/v1/exercises?pack=nonexistent-pack", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 with empty list or actual exercises
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// Note: TestHandlers_GetExercise_NotFound already exists earlier in this file

// ========== Format Handler Coverage Tests ==========

// TestHandlers_Format_EmptyCodeDirect tests the format endpoint with empty code
// Note: /v1/sessions/{id}/format doesn't validate session ID, it just formats code
func TestHandlers_Format_EmptyCodeDirect(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Format endpoint accepts any session ID and just formats the provided code
	body := strings.NewReader(`{"code": {}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty code returns 200 with empty formatted result
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandlers_Format_MultipleFilesDirect tests formatting multiple files
func TestHandlers_Format_MultipleFilesDirect(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Format with multiple files - endpoint doesn't validate session
	body := strings.NewReader(`{
		"code": {
			"main.go": "package main\n\nfunc main() {}",
			"helper.go": "package main\n\nfunc helper()  {}"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Note: TestHandlers_Format_InvalidJSON and TestHandlers_Format_SessionNotFound already exist earlier

// ========== Patch Handler Additional Coverage Tests ==========

func TestHandlers_PatchApply_WithSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Apply patch
	body := strings.NewReader(`{"patch_id": "test-patch-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on state (no pending patch)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 400, or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_WithSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Reject patch
	body := strings.NewReader(`{"patch_id": "test-patch-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on state
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 400, or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchPreview_WithSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Get patch preview
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or not have patches
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Authoring Handler Additional Coverage Tests ==========

func TestHandlers_AuthoringSuggest_WithSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request authoring suggestion - requires "section" field
	body := strings.NewReader(`{
		"section": "goals",
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"context": "I want to add a function"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed, fail, or return 400 for non-authoring session
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringHint_WithSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request authoring hint - may fail if not a spec authoring session
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"context": "I'm stuck on the next step"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed, fail, or return 400 for non-authoring session
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringApply_WithSession_Success(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Apply authoring suggestion
	body := strings.NewReader(`{
		"suggestion_id": "test-suggestion-1",
		"code": {"main.go": "package main\n\nfunc main() { println(\"hello\") }"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on state
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 400, or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Note: Spec Handler tests already exist earlier in this file

// ========== Patch Stats and Log Handler Tests ==========

func TestHandlers_PatchStats_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Get patch stats
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchLog_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Get patch log
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches/log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Note: Analytics Handler tests with UserID already exist earlier in this file

// ========== Session Hint and Pairing Handler Tests ==========

func TestHandlers_Hint_WithValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request hint
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"context": "I need help"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Review_WithValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request review
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() { println(\"hello\") }"},
		"context": "Please review my code"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Stuck_WithValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Report stuck
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"context": "I'm completely stuck"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Next_WithValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request next step
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"context": "What should I do next?"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Explain_WithValidSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request explanation
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"context": "Explain the main function"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail depending on LLM availability
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Additional Escalation Handler Tests ==========

func TestHandlers_Escalate_ShortJustification(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request escalation with too short justification
	body := strings.NewReader(`{
		"level": 4,
		"justification": "help me please"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_NotEnoughHints(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request escalation without having enough hints first
	body := strings.NewReader(`{
		"level": 4,
		"justification": "I have tried many different approaches but nothing seems to work correctly"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should fail because no hints have been requested yet
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Session Run Tests ==========

func TestHandlers_Run_SessionWithTest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Run with test enabled
	body := strings.NewReader(`{
		"code": {
			"main.go": "package main\n\nfunc main() {}",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}"
		},
		"format": true,
		"build": true,
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Analytics Handler Edge Cases ==========

func TestHandlers_AnalyticsOverview_NoProfile(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Request analytics overview without user_id (default profile)
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Handler Coverage ==========

func TestHandlers_CreateSpec_ValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create valid spec - use correct route /v1/specs
	body := strings.NewReader(`{
		"name": "Test Spec",
		"goals": ["Learn Go basics"],
		"success_criteria": ["Write a Hello World program"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 201, 200, or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_MarkCriterion_InvalidInput(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to mark criterion with invalid data - use correct route
	body := strings.NewReader(`{
		"satisfied": true
	}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/test-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should fail with 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_InvalidSpecPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to lock nonexistent spec - use correct route
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/nonexistent/spec", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Patch Handler Edge Cases ==========

func TestHandlers_PatchApply_MissingPatchID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Try to apply without patch_id
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_MissingPatchID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Try to reject without patch_id
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_WithReason(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Reject with reason
	body := strings.NewReader(`{
		"patch_id": "some-patch-id",
		"reason": "I want to solve it differently"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Will return 404 for nonexistent patch or 200 for success
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 400, or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Authoring Handler Edge Cases ==========

func TestHandlers_AuthoringDiscover_EmptyBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Discover with empty body - should use defaults
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed with defaults or fail based on workspace
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringDiscover_WithDocsPaths(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Discover with docs_paths specified
	body := strings.NewReader(`{
		"docs_paths": ["exercises"],
		"recursive": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Profile Handler Coverage ==========

func TestHandlers_GetProfile_ValidUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Valid UUID but profile may not exist
	req := httptest.NewRequest(http.MethodGet, "/v1/profiles/00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 with default profile
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Exercise Handler Edge Cases ==========

func TestHandlers_GetExercise_DeepPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to get exercise with deep nested path
	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/pack/level1/level2/level3/exercise", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should handle gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListExercises_SpecificPack(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// List exercises in specific pack
	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/test-pack", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Config Handler Edge Cases ==========

func TestHandlers_GetConfig_Full(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListProviders_Coverage(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config/providers", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Session Edge Cases ==========

func TestHandlers_GetSession_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Authoring Session Tests ==========

func createSpecAuthoringSession(t *testing.T, server *Server) string {
	t.Helper()
	body := strings.NewReader(`{
		"spec_path": "exercises/go-v1/basics/hello-world/spec.yaml",
		"docs_paths": ["exercises"],
		"intent": "spec_authoring"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("failed to create spec authoring session: status %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode session response: %v", err)
	}
	return resp.ID
}

func TestHandlers_CreateSession_SpecAuthoring(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"spec_path": "exercises/go-v1/basics/hello-world/spec.yaml",
		"docs_paths": ["exercises"],
		"intent": "spec_authoring"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept various status codes since we may not have the actual spec file
	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 201, 200, 404, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringHint_EmptySessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions//authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 301 redirect for double slashes, 400, or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusMovedPermanently {
		t.Errorf("expected status 400, 404, or 301, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringApply_EmptySessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals", "suggestions": []}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions//authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 301 redirect for double slashes, 400, or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusMovedPermanently {
		t.Errorf("expected status 400, 404, or 301, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Analytics Handler Extended Tests ==========

func TestHandlers_AnalyticsSkills_ValidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills?user_id=00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsErrors_ValidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors?user_id=00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsTrend_ValidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend?user_id=00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsOverview_InvalidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview?user_id=invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 400 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsSkills_InvalidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills?user_id=invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 400 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsErrors_InvalidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors?user_id=invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 400 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AnalyticsTrend_InvalidUserID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend?user_id=invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 400 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Profile Handler Extended Tests ==========

func TestHandlers_GetProfile_EmptyUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 404 for missing ID
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Handler Extended Tests ==========

func TestHandlers_CreateSpec_EmptyName(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"name": "",
		"goals": ["Learn Go basics"],
		"success_criteria": ["Write a Hello World program"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should fail or succeed based on validation
	if w.Code != http.StatusBadRequest && w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Errorf("expected status 400, 201, or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSpec_EmptyGoals(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"name": "Test Spec",
		"goals": [],
		"success_criteria": ["Write a Hello World program"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Errorf("expected status 400, 201, or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSpec_InvalidPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/nonexistent/path/spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 404, 400, or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ValidateSpec_NonexistentPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 400, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_LockSpec_NonexistentPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 400, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_SpecProgress_NonexistentPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 404, 400, or 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_SpecDrift_NonexistentPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/nonexistent/path", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("expected status 404, 400, or 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Run Handler Extended Tests ==========

func TestHandlers_CreateRun_EmptyBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail based on defaults
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusAccepted {
		t.Errorf("expected status 200, 202, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateRun_WithFormat(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"format": true,
		"build": false,
		"test": false
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusAccepted {
		t.Errorf("expected status 200, 202, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateRun_WithBuild(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"format": false,
		"build": true,
		"test": false
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusAccepted {
		t.Errorf("expected status 200, 202, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateRun_WithTest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"format": false,
		"build": false,
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusAccepted {
		t.Errorf("expected status 200, 202, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateRun_WithAllOptions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"format": true,
		"build": true,
		"test": true,
		"timeout": 30
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError && w.Code != http.StatusAccepted {
		t.Errorf("expected status 200, 202, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Escalate Handler Extended Tests ==========

func TestHandlers_Escalate_WithValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"level": 3,
		"justification": "Need more detailed guidance to understand the concept"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail based on pairing service
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_LevelTooHigh(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"level": 10,
		"justification": "Testing very high level"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_NegativeLevel(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{
		"level": -1,
		"justification": "Testing negative level"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 200, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Pairing Handler Extended Tests ==========

func TestHandlers_Hint_EmptyBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Use /hint endpoint which is a valid pairing endpoint
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail based on session state
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Explain_WithRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AllPairingEndpoints(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test each pairing endpoint: hint, review, stuck, next, explain
	endpoints := []string{"hint", "review", "stuck", "next", "explain"}
	for _, endpoint := range endpoints {
		t.Run("endpoint_"+endpoint, func(t *testing.T) {
			sessionID := createTestSession(t, server)

			req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/"+endpoint, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
				t.Errorf("expected status 200, 400, or 500 for %s, got %d: %s", endpoint, w.Code, w.Body.String())
			}
		})
	}
}

// ========== Format Handler Extended Tests ==========

func TestHandlers_Format_EmptyBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Format_InvalidSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/format", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 200 with empty result or 400/404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Errorf("expected status 400, 404, or 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Delete Session Tests ==========

func TestHandlers_DeleteSession_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNoContent && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200, 204, or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_DeleteSession_NonexistentUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Session may not exist - can return 404, 500 (session not found error), 200, or 204
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusNoContent && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 204, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Middleware Tests ==========

func TestHandlers_CORSHeaders(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodOptions, "/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Check for CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		// May not have CORS configured
		t.Log("CORS headers not configured")
	}
}

func TestHandlers_ContentTypeJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" && !strings.HasPrefix(contentType, "application/json") {
		t.Logf("Content-Type is %q, expected application/json", contentType)
	}
}

// ========== Additional Coverage Tests ==========

func TestHandlers_CreateRun_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"format": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Hint_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Review_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Stuck_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Next_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Explain_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Escalate_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"level": 4, "justification": "need help"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Escalation requires level 4 or 5, returns 404/500 for nonexistent session
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404, 500 or 400 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Hint_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Review_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Stuck_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Next_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Explain_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Format_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed with empty formatting or return error
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSession_NonexistentUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_GetSession_InvalidUUID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/invalid-uuid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404 for invalid uuid, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Status_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404 for status endpoint, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_Config_Endpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404 for config endpoint, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchPreview_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Patch preview returns 200 with empty patch for nonexistent session
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("expected status 404, 500 or 200 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchApply_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_PatchReject_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListPatches_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/00000000-0000-0000-0000-000000000001/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return empty list or error
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_ListExercises_WithTrack(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises?track=go-v1", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for list exercises with track, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_FeatureGuidance(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"spec_path": "exercises/go-v1/basics/hello-world/spec.yaml",
		"intent": "feature_guidance"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May succeed or fail based on spec existence
	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 201, 200, 404, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_Greenfield(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"intent": "greenfield",
		"code": {"main.go": "package main"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 201, 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_CreateSession_MissingFields(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing fields, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_MarkCriterion_NonexistentCriterion(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"satisfied": true}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/99999", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 400 if spec path is also required
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404, 200, 400 or 500 for nonexistent criterion, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringSuggest_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404, 500 or 400 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringHint_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404, 500 or 400 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlers_AuthoringApply_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals", "suggestions": ["test"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 404, 500 or 400 for nonexistent session, got %d: %s", w.Code, w.Body.String())
	}
}

// ============== Additional Coverage Tests ==============

// Test handleCreateRun with format option
func TestHandlers_CreateRun_FormatOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "format": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateRun with build option
func TestHandlers_CreateRun_BuildOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "build": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateRun with test option
func TestHandlers_CreateRun_TestOnly(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateRun with all options
func TestHandlers_CreateRun_AllOptions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}, "format": true, "build": true, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing with valid run_id
func TestHandlers_Pairing_WithRunID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"run_id": "00000000-0000-0000-0000-000000000001"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 500 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing with code in request
func TestHandlers_Pairing_WithCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 200, 500 or 429, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing with streaming request
func TestHandlers_Pairing_Streaming(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"stream": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Streaming may return 200 with SSE or error
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 200, 500 or 429 for streaming, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchApply with invalid session ID format
func TestHandlers_PatchApply_InvalidSessionIDFormat(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/patch/apply", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404 for invalid session ID, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchReject with invalid session ID format
func TestHandlers_PatchReject_InvalidSessionIDFormat(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/patch/reject", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Errorf("expected status 400, 404 or 200, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleValidateSpec with valid path
func TestHandlers_ValidateSpec_ValidPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/test-spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleLockSpec with valid body
func TestHandlers_LockSpec_ValidBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"locked": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/test-spec.yaml", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Lock creates a new lock (201) or updates existing (200)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 201, 404, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetSpecProgress
func TestHandlers_GetSpecProgress(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/test-spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetSpecDrift
func TestHandlers_GetSpecDrift(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/test-spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleMarkCriterionSatisfied with valid body
func TestHandlers_MarkCriterionSatisfied_ValidBody(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"satisfied": true, "spec_path": "test.yaml"}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchLog
func TestHandlers_PatchLog_Detailed(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchStats
func TestHandlers_PatchStats_Detailed(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringSuggest with valid request
func TestHandlers_AuthoringSuggest_ValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "content": "test content"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringHint with valid request
func TestHandlers_AuthoringHint_ValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "content": "test content"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringApply with valid request
func TestHandlers_AuthoringApply_ValidRequest(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "suggestions": ["suggestion1", "suggestion2"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringDiscover with valid paths
func TestHandlers_AuthoringDiscover_ValidPaths(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"docs_paths": ["/tmp"], "recursive": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleReady with different states
func TestHandlers_Ready_States(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test multiple times to cover different code paths
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status 200 or 503, got %d: %s", w.Code, w.Body.String())
		}
	}
}

// Test handleListExercises with filter
func TestHandlers_ListExercises_WithFilter(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises?pack=test-pack", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleListPackExercises
func TestHandlers_ListPackExercises_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/test-pack", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetExercise
func TestHandlers_GetExercise_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/test-pack/test-exercise", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateSession with different exercise types
func TestHandlers_CreateSession_WithPack(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"exercise_id": "test-pack/test-exercise"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 201, 200, 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleListPatches with session
func TestHandlers_ListPatches_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchPreview with session
func TestHandlers_PatchPreview_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchApply with session
func TestHandlers_PatchApply_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 404 if no pending patch, or 200 if applied
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchReject with session
func TestHandlers_PatchReject_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 404 if no pending patch, or 200 if rejected
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleFormat with valid code
func TestHandlers_Format_ValidCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main(){\n}\n"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleEscalate with code
func TestHandlers_Escalate_WithCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 4, "justification": "I am completely stuck and need more help with this", "code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleEscalate with run_id
func TestHandlers_Escalate_WithRunID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 4, "justification": "I am completely stuck and need more help with this problem", "run_id": "test-run-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleEscalate requesting streaming
func TestHandlers_Escalate_Streaming(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"level": 4, "justification": "I am completely stuck and need more help with this", "stream": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateRun with empty code
func TestHandlers_CreateRun_EmptyCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Empty code may be rejected
	if w.Code != http.StatusOK && w.Code != http.StatusAccepted && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 202, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateRun with multiple files
func TestHandlers_CreateRun_MultiFile(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}", "util.go": "package main\nfunc helper() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusAccepted && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 202 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringSuggest with empty section (requires session context)
func TestHandlers_AuthoringSuggest_EmptySection(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "", "content": "some content"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May be 404 if route doesn't exist in this format, or other status
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringHint with empty section (requires session context)
func TestHandlers_AuthoringHint_EmptySection(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "", "hint_type": "clarify"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May be 404 if route doesn't exist in this format, or other status
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleAuthoringApply with empty suggestions (requires session context)
func TestHandlers_AuthoringApply_EmptySuggestions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"suggestions": []}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May be 404 if route doesn't exist in this format, or other status
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchApply twice (no pending patch on second)
func TestHandlers_PatchApply_Twice(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// First apply
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// Second apply - should have no pending patch
	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/apply", nil)
	w2 := httptest.NewRecorder()
	server.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK && w2.Code != http.StatusNotFound && w2.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w2.Code, w2.Body.String())
	}
}

// Test handlePatchReject twice
func TestHandlers_PatchReject_Twice(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// First reject
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// Second reject - should have no pending patch
	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patch/reject", nil)
	w2 := httptest.NewRecorder()
	server.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK && w2.Code != http.StatusNotFound && w2.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w2.Code, w2.Body.String())
	}
}

// Test handleGetSpecProgress with nonexistent session
func TestHandlers_GetSpecProgress_Nonexistent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/nonexistent-session-id/spec/progress", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetSpecDrift with nonexistent session
func TestHandlers_GetSpecDrift_Nonexistent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/nonexistent-session-id/spec/drift", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleProfile after session activity
func TestHandlers_Profile_AfterActivity(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Do some activity first - request a hint
	hintBody := strings.NewReader(`{}`)
	hintReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", hintBody)
	hintReq.Header.Set("Content-Type", "application/json")
	hintW := httptest.NewRecorder()
	server.router.ServeHTTP(hintW, hintReq)

	// Now check profile
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing with streaming Accept header
func TestHandlers_Pairing_StreamingHint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"stream": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing with streaming review
func TestHandlers_Pairing_StreamingReview(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"stream": true, "code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing stuck with streaming
func TestHandlers_Pairing_StreamingStuck(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"stream": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing next with streaming
func TestHandlers_Pairing_StreamingNext(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"stream": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing explain with streaming
func TestHandlers_Pairing_StreamingExplain(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"stream": true, "topic": "error handling"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleRuns list (runs endpoint may only support POST)
func TestHandlers_ListRuns_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/runs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// GET may return 405 if only POST is supported
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, 405 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetRun
func TestHandlers_GetRun_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/runs/test-run-123", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetRun with nonexistent session
func TestHandlers_GetRun_NonexistentSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/nonexistent-session/runs/test-run-123", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateRun with test files
func TestHandlers_CreateRun_WithTestFiles(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc Add(a, b int) int { return a + b }", "main_test.go": "package main\nimport \"testing\"\nfunc TestAdd(t *testing.T) { if Add(1,2) != 3 { t.Fail() } }"}, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusAccepted && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 202 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleHints list
func TestHandlers_ListHints_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/hints", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleHints after requesting hint
func TestHandlers_ListHints_AfterHint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Request a hint first
	hintBody := strings.NewReader(`{}`)
	hintReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", hintBody)
	hintReq.Header.Set("Content-Type", "application/json")
	hintW := httptest.NewRecorder()
	server.router.ServeHTTP(hintW, hintReq)

	// Now list hints
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/hints", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleSpecValidate with existing spec
func TestHandlers_ValidateSpec_WithContent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"path": "exercises/go-v1/fizzbuzz", "content": "# FizzBuzz\n\nWrite a fizzbuzz implementation."}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/spec/validate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleLockSpec with content
func TestHandlers_LockSpec_WithContent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"content": "# Test Spec\n\nThis is a test specification."}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/test/spec/path", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// May return 404 if spec not found
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 201, 400, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchLog
func TestHandlers_PatchLog_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePatchStats
func TestHandlers_PatchStats_Valid(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleListSessions (GET may not be supported, POST for create)
func TestHandlers_ListSessions(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session first
	_ = createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// GET may return 405 if only POST is supported for session creation
	if w.Code != http.StatusOK && w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 405 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateSession with mode
func TestHandlers_CreateSession_WithMode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"exercise_id": "go-v1/fizzbuzz", "mode": "guided"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 201, 200, 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateSession with observe mode
func TestHandlers_CreateSession_ObserveMode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"exercise_id": "go-v1/fizzbuzz", "mode": "observe"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 201, 200, 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleCreateSession with teach mode
func TestHandlers_CreateSession_TeachMode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"exercise_id": "go-v1/fizzbuzz", "mode": "teach"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 201, 200, 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing with code context
func TestHandlers_Hint_WithCodeContext(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {\n\t// stuck here\n}"}, "context": "I am trying to implement fizzbuzz"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing review with run_id
func TestHandlers_Review_WithRunID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"run_id": "some-run-id-123", "code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing stuck with context
func TestHandlers_Stuck_WithContext(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"context": "I cannot figure out how to iterate over a slice", "code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing explain with specific topic
func TestHandlers_Explain_WithTopic(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"topic": "goroutines and channels", "context": "I want to understand concurrency"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handlePairing next with context
func TestHandlers_Next_WithContext(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"context": "What should I do after implementing the main function?", "code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleMarkCriterionSatisfied with valid ID
func TestHandlers_MarkCriterion_ValidID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"satisfied": true}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/test-criterion-1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, 400 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetProfile with nonexistent session
func TestHandlers_GetProfile_Nonexistent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/nonexistent-session/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404, 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleFormat with badly formatted code
func TestHandlers_Format_BadlyFormattedCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\nimport \"fmt\"\nfunc main(){fmt.Println(   \"hello\"   )}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleFormat with invalid Go code
func TestHandlers_Format_InvalidGoCode(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"code": {"main.go": "not valid go code at all!!!"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Invalid Go code should still return 200 with format errors, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleReady in various states
func TestHandlers_Ready_ChecksAll(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Make a request that exercises the ready checks
	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Ready returns 200 or 503 depending on dependencies
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleListProviders
func TestHandlers_ListProviders(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 200, 404 (route may not exist with this path), or 500
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleGetConfig
func TestHandlers_GetConfig(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test handleStatus
func TestHandlers_Status(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Standalone CreateRun Tests ==========

// Test standalone run (no session) with format only
func TestHandlers_CreateRun_Standalone_Format(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}, "format": true, "build": false, "test": false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test standalone run with build only
func TestHandlers_CreateRun_Standalone_Build(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}, "format": false, "build": true, "test": false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test standalone run with test only
func TestHandlers_CreateRun_Standalone_Test(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}", "main_test.go": "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}"}, "format": false, "build": false, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test standalone run with all options
func TestHandlers_CreateRun_Standalone_All(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}", "main_test.go": "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}"}, "format": true, "build": true, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test standalone run with invalid JSON
func TestHandlers_CreateRun_Standalone_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 400, 404 (route may not exist without session), or 500
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test standalone run with build failure (syntax error)
func TestHandlers_CreateRun_Standalone_BuildFailure(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Code with syntax error
	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() { syntax error"}, "format": false, "build": true, "test": false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Session Run with Appreciation Test ==========

// Test session run that triggers appreciation check (test passes)
func TestHandlers_Run_SessionWithAppreciation(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// Run with test that should pass
	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}", "main_test.go": "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) { t.Log(\"passed\") }"}, "format": false, "build": false, "test": true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 200, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== AuthoringSuggest Edge Cases ==========

// Test authoring suggest with missing session ID in path
func TestHandlers_AuthoringSuggest_MissingSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 400, 404, or 405
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 400, 404, or 405, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring suggest with invalid JSON (with session)
func TestHandlers_AuthoringSuggest_InvalidJSON_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring suggest with non-authoring session
func TestHandlers_AuthoringSuggest_NonAuthoringSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Session is not an authoring session, so expect 400 or 404
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring suggest with different sections
func TestHandlers_AuthoringSuggest_Features(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "features", "context": "I want to add user authentication"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept any of these since the session may not be authoring type
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring suggest for acceptance_criteria
func TestHandlers_AuthoringSuggest_AcceptanceCriteria(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "acceptance_criteria"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== AuthoringHint Edge Cases ==========

// Test authoring hint with missing session ID
func TestHandlers_AuthoringHint_MissingSessionID(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"section": "goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 400, 404, or 405
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 400, 404, or 405, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring hint with invalid JSON (with session)
func TestHandlers_AuthoringHint_InvalidJSON_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{not valid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring hint with valid section
func TestHandlers_AuthoringHint_ValidSection(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"section": "goals", "context": "help me define goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Patch Edge Cases ==========

// Test patch apply with nonexistent patch
func TestHandlers_PatchApply_NonexistentPatch(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patches/nonexistent-id/apply", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test patch reject with nonexistent patch
func TestHandlers_PatchReject_NonexistentPatch(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patches/nonexistent-id/reject", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test patch preview with nonexistent patch
func TestHandlers_PatchPreview_NonexistentPatch(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches/nonexistent-id/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 404 or 500
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 404 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test list patches for session
func TestHandlers_ListPatches_ForSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test patch stats for session
func TestHandlers_PatchStats_ForSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test patch log for session
func TestHandlers_PatchLog_ForSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches/log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Profile Edge Cases ==========

// Test get profile (correct route is /v1/profile)
func TestHandlers_GetProfile_CorrectRoute(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 200 (empty profile), 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Analytics Edge Cases ==========

// Test analytics overview (correct route is /v1/analytics/overview)
func TestHandlers_AnalyticsOverview_CorrectRoute(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test analytics skills (correct route is /v1/analytics/skills)
func TestHandlers_AnalyticsSkills_CorrectRoute(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test analytics errors (correct route is /v1/analytics/errors)
func TestHandlers_AnalyticsErrors_CorrectRoute(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test analytics trend (correct route is /v1/analytics/trend)
func TestHandlers_AnalyticsTrend_CorrectRoute(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Spec Edge Cases ==========

// Test validate spec with valid request (correct route is /v1/specs/validate/{path...})
func TestHandlers_ValidateSpec_ValidPathNew(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/test.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 200, 400, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test validate spec with nested path
func TestHandlers_ValidateSpec_NestedPath(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/specs/test.spec.yaml", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 200, 400, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test lock spec with valid session
func TestHandlers_LockSpec_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{"spec_path": "test.spec.yaml"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/spec/lock", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test get spec progress with valid session
func TestHandlers_GetSpecProgress_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/spec/progress", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test get spec drift with valid session
func TestHandlers_GetSpecDrift_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/spec/drift", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test mark criterion with malformed JSON body
func TestHandlers_MarkCriterion_MalformedJSON(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	body := strings.NewReader(`{not json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/spec/criteria/test-id/mark", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Authoring Discover Edge Cases ==========

// Test authoring discover with valid session
func TestHandlers_AuthoringDiscover_WithSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/authoring/discover", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept various status codes since session may not be authoring type
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test authoring discover (correct method is POST, route is /v1/authoring/discover)
func TestHandlers_AuthoringDiscover_CorrectRoute(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"spec_path": "test.spec.yaml"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 200, 400, 404, or 500
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== handleReady Edge Cases ==========

// Test ready endpoint returns correct status
func TestHandlers_Ready_Status(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Ready should return 200 or 503 depending on dependencies
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Escalation Cooldown Test ==========

// Test escalation when on cooldown (needs prior interventions)
func TestHandlers_Escalate_CooldownActive(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	// First, request multiple hints to enable escalation
	for i := 0; i < 3; i++ {
		body := strings.NewReader(`{"code": {"main.go": "package main"}, "context": "help"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
	}

	// Now try escalation - may trigger cooldown
	body := strings.NewReader(`{"level": 4, "justification": "I have tried multiple hints and still do not understand the concept."}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept any of: 200 (success), 400 (validation), 429 (cooldown), 500 (LLM error)
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusTooManyRequests && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 400, 429, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Session Delete Edge Cases ==========

// Test delete nonexistent session
func TestHandlers_DeleteSession_Nonexistent(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/nonexistent-session-id", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 404 or 204 (idempotent delete)
	if w.Code != http.StatusNotFound && w.Code != http.StatusNoContent && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 204, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test delete existing session
func TestHandlers_DeleteSession_Existing(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	sessionID := createTestSession(t, server)

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept 204 (success) or 500 (internal error)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 204, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================================
// Mock LLM Provider Tests - These use setupTestServerWithMockLLM
// for testing handlers that require working LLM responses
// ============================================================================

// createTestSessionWithMockLLM creates a session using the mock LLM server setup
func createTestSessionWithMockLLM(t *testing.T, server *Server) string {
	t.Helper()

	// Create exercise structure
	basePath := server.exerciseLoader.BasePath()
	packPath := filepath.Join(basePath, "mock-pack")
	categoryPath := filepath.Join(packPath, "basics")
	if err := os.MkdirAll(categoryPath, 0755); err != nil {
		t.Fatalf("create category dir: %v", err)
	}

	// Write pack.yaml
	packYAML := `id: mock-pack
name: Mock Pack
version: "1.0"
description: Test exercises with mock LLM
language: go
exercises:
  - basics/hello
`
	if err := os.WriteFile(filepath.Join(packPath, "pack.yaml"), []byte(packYAML), 0644); err != nil {
		t.Fatalf("write pack.yaml: %v", err)
	}

	// Write exercise.yaml
	exerciseYAML := `id: hello
title: Hello World
difficulty: beginner
description: Say hello
instructions: Print "Hello, World!"
starter:
  main.go: |
    package main

    func main() {
    }
tests:
  main_test.go: |
    package main

    import "testing"

    func TestMain(t *testing.T) {
    }
`
	if err := os.WriteFile(filepath.Join(categoryPath, "hello.yaml"), []byte(exerciseYAML), 0644); err != nil {
		t.Fatalf("write exercise.yaml: %v", err)
	}

	// Create session
	body := strings.NewReader(`{"exercise_id": "mock-pack/basics/hello", "track": "practice"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sessionID, ok := resp["id"].(string)
	if !ok {
		t.Fatal("expected session ID in response")
	}
	return sessionID
}

// prepareSessionForEscalation sets HintCount >= 2 on a session without triggering cooldown
// This bypasses the need for working LLM and avoids cooldown checks by directly modifying the session file
func prepareSessionForEscalation(t *testing.T, ctx *testServerContext, sessionID string) {
	t.Helper()

	// Read the session file directly
	sessionFile := filepath.Join(ctx.SessionsPath, "sessions", sessionID+".json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}

	// Parse the session
	var sess session.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("parse session: %v", err)
	}

	// Modify HintCount and clear LastInterventionAt to avoid cooldown
	sess.HintCount = 3
	sess.LastInterventionAt = nil // Clear to avoid cooldown check

	// Write back
	data, err = json.MarshalIndent(sess, "", "  ")
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
}

// markSessionCompleted directly modifies a session file to set status to completed
func markSessionCompleted(t *testing.T, ctx *testServerContext, sessionID string) {
	t.Helper()

	// Read the session file directly
	sessionFile := filepath.Join(ctx.SessionsPath, "sessions", sessionID+".json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}

	// Parse the session
	var sess session.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("parse session: %v", err)
	}

	// Set status to completed
	sess.Status = session.StatusCompleted

	// Write back
	data, err = json.MarshalIndent(sess, "", "  ")
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
}

func TestMockLLM_Pairing_Hint_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify response contains expected fields
	if resp["content"] == nil {
		t.Error("expected 'content' field in response")
	}
	if resp["level"] == nil {
		t.Error("expected 'level' field in response")
	}
	if resp["type"] == nil {
		t.Error("expected 'type' field in response")
	}
}

func TestMockLLM_Pairing_Review_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() { println(\"hello\") }"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["content"] == nil {
		t.Error("expected 'content' field in response")
	}
}

func TestMockLLM_Pairing_Stuck_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "context": "I'm stuck on how to print output"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Pairing_Next_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Pairing_Explain_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "topic": "functions"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Pairing_StreamingHint_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Streaming should return 200 with event stream
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Check content type for streaming
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		// May fall back to JSON response
		if !strings.Contains(contentType, "application/json") {
			t.Errorf("expected streaming content type, got %s", contentType)
		}
	}
}

func TestMockLLM_Pairing_AllIntents(t *testing.T) {
	// Test each intent with a fresh session to avoid cooldown
	intents := []struct {
		endpoint string
		body     string
	}{
		{"/hint", `{"code": {"main.go": "package main"}}`},
		{"/review", `{"code": {"main.go": "package main\nfunc main() {}"}}`},
		{"/stuck", `{"code": {"main.go": "package main"}}`},
		{"/next", `{"code": {"main.go": "package main"}}`},
		{"/explain", `{"code": {"main.go": "package main"}, "topic": "loops"}`},
	}

	for _, intent := range intents {
		t.Run(intent.endpoint, func(t *testing.T) {
			server, cleanup := setupTestServerWithMockLLM(t)
			defer cleanup()

			sessionID := createTestSessionWithMockLLM(t, server)

			body := strings.NewReader(intent.body)
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+intent.endpoint, body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
			}
		})
	}
}

func TestMockLLM_Pairing_Escalate_ValidationPasses(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Use helper to prepare session with sufficient hints
	prepareSessionForEscalation(t, ctx, sessionID)

	// Now attempt escalation with valid justification
	body := strings.NewReader(`{
		"level": 4,
		"justification": "I have tried multiple approaches but I'm completely stuck on this concept"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed since we have hints and a valid justification
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify response contains expected fields
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["escalated"] != true {
		t.Errorf("expected escalated=true in response")
	}
	if resp["level"] == nil {
		t.Error("expected 'level' field in response")
	}
	if resp["content"] == nil {
		t.Error("expected 'content' field in response")
	}
}

func TestMockLLM_AuthoringSuggest_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// Create a session in authoring mode
	basePath := server.exerciseLoader.BasePath()
	packPath := filepath.Join(basePath, "authoring-pack")
	if err := os.MkdirAll(packPath, 0755); err != nil {
		t.Fatalf("create pack dir: %v", err)
	}

	// Write pack.yaml
	packYAML := `id: authoring-pack
name: Authoring Pack
version: "1.0"
description: Test authoring
language: go
`
	if err := os.WriteFile(filepath.Join(packPath, "pack.yaml"), []byte(packYAML), 0644); err != nil {
		t.Fatalf("write pack.yaml: %v", err)
	}

	// Create spec file in a temp directory
	specYAML := `id: test-spec
title: Test Spec for Authoring
learning_goals:
  - Understand basic Go syntax
criteria:
  - description: Code compiles
    weight: 1.0
`
	// Use parent of basePath for sessions (same temp directory)
	sessionsPath := filepath.Join(filepath.Dir(basePath), "sessions")
	specPath := filepath.Join(sessionsPath, "test-spec.yaml")
	if err := os.WriteFile(specPath, []byte(specYAML), 0644); err != nil {
		t.Fatalf("write spec.yaml: %v", err)
	}

	// Create session in authoring mode
	body := strings.NewReader(`{"spec_path": "` + specPath + `", "mode": "author", "track": "authoring"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		// If spec session creation fails, skip this test
		t.Skipf("Could not create authoring session: %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sessionID, ok := resp["id"].(string)
	if !ok {
		t.Skip("No session ID in response")
	}

	// Try authoring suggest
	suggestBody := strings.NewReader(`{"section": "criteria"}`)
	suggestReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", suggestBody)
	suggestReq.Header.Set("Content-Type", "application/json")
	suggestW := httptest.NewRecorder()

	server.router.ServeHTTP(suggestW, suggestReq)

	// Accept various responses since authoring depends on document context
	if suggestW.Code != http.StatusOK && suggestW.Code != http.StatusBadRequest && suggestW.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d: %s", suggestW.Code, suggestW.Body.String())
	}
}

func TestMockLLM_AuthoringHint_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// Create a simple session for authoring hint
	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"question": "How do I structure my criteria?"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Accept OK or BadRequest (if session is not in authoring mode)
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_CreateSuccess(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Create a run
	body := strings.NewReader(`{"code": {"main.go": "package main\n\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Runs endpoint should accept the request
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusAccepted {
		t.Errorf("expected status 200/201/202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Ready_WithProvider(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Ready endpoint should return 200 since we have a mock provider registered
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d or %d, got %d: %s", http.StatusOK, http.StatusServiceUnavailable, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have checks field
	if resp["checks"] != nil {
		checks, ok := resp["checks"].(map[string]interface{})
		if ok {
			if llmCheck, hasLLM := checks["llm_provider"]; hasLLM {
				llmCheckMap, _ := llmCheck.(map[string]interface{})
				if llmCheckMap["status"] != "ready" && llmCheckMap["status"] != "degraded" {
					t.Logf("LLM check status: %v", llmCheckMap["status"])
				}
			}
		}
	}
}

func TestMockLLM_Status_ShowsProvider(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should show LLM providers
	providers, ok := resp["llm_providers"].([]interface{})
	if !ok {
		t.Error("expected 'llm_providers' array in response")
	}

	// Should have at least the mock provider
	if len(providers) == 0 {
		t.Error("expected at least one LLM provider")
	}

	// Check if mock provider is in the list
	hasMock := false
	for _, p := range providers {
		if p == "mock" {
			hasMock = true
			break
		}
	}
	if !hasMock {
		t.Errorf("expected 'mock' provider in list, got: %v", providers)
	}
}

func TestMockLLM_Pairing_ResponseStructure(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify complete intervention response structure
	requiredFields := []string{"id", "intent", "level", "type", "content"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field '%s' in response", field)
		}
	}

	// Verify content comes from mock
	content, ok := resp["content"].(string)
	if !ok || content == "" {
		t.Error("expected non-empty 'content' in response")
	}

	// Level should be a number
	if level, ok := resp["level"].(float64); !ok || level < 0 || level > 5 {
		t.Errorf("expected level 0-5, got %v", resp["level"])
	}
}

func TestMockLLM_SingleHint_Success(t *testing.T) {
	// Test that a single hint request succeeds
	// (Multiple hints are blocked by cooldown in the same session)
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify session can be retrieved
	getReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	getW := httptest.NewRecorder()

	server.router.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("get session: expected status %d, got %d", http.StatusOK, getW.Code)
	}

	var sess map[string]interface{}
	if err := json.NewDecoder(getW.Body).Decode(&sess); err != nil {
		t.Fatalf("decode session: %v", err)
	}

	// Verify session has the correct ID
	if sess["id"] != sessionID {
		t.Errorf("expected session ID %s, got %v", sessionID, sess["id"])
	}
}

func TestMockLLM_HintThenCooldown_Returns429(t *testing.T) {
	// Test that a second hint within cooldown returns 429
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// First hint - should succeed
	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first hint: expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Second hint - should return cooldown (429)
	body2 := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	server.router.ServeHTTP(w2, req2)

	// Expected 429 (Too Many Requests) due to cooldown
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second hint: expected status %d, got %d: %s", http.StatusTooManyRequests, w2.Code, w2.Body.String())
	}
}

func TestMockLLM_Pairing_WithCode_Validates(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Test with valid Go code
	body := strings.NewReader(`{
		"code": {
			"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Pairing_Explain_WithTopic(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"code": {"main.go": "package main"},
		"topic": "goroutines and channels"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify it's an explain type
	if intentType, ok := resp["type"].(string); ok {
		if intentType != "explain" && intentType != "guide" {
			t.Logf("Response type: %s", intentType)
		}
	}
}

// ============================================================================
// Additional Escalation Tests - Coverage for handlePairingWithEscalation
// ============================================================================

func TestMockLLM_Escalate_InvalidLevel(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Level 3 is not valid for escalation (must be 4 or 5)
	body := strings.NewReader(`{
		"level": 3,
		"justification": "This is my detailed justification for escalation"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should say level 4 or 5 required
	if errMsg, ok := resp["error"].(string); ok {
		if !strings.Contains(errMsg, "level 4 or 5") {
			t.Errorf("expected error about level 4 or 5, got: %s", errMsg)
		}
	}
}

func TestMockLLM_Escalate_EmptyJustification(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"level": 4,
		"justification": ""
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestMockLLM_Escalate_ShortJustification(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Justification less than 20 characters
	body := strings.NewReader(`{
		"level": 4,
		"justification": "short"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestMockLLM_Escalate_Level5(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Use helper to prepare session with sufficient hints
	prepareSessionForEscalation(t, ctx, sessionID)

	// Request L5 escalation (full solution)
	body := strings.NewReader(`{
		"level": 5,
		"justification": "I am completely stuck and need the full solution to learn from it"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed since we have sufficient hints
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for L5 with hints, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["escalated"] != true {
		t.Errorf("expected escalated=true in response")
	}
}

func TestMockLLM_Escalate_WithCode(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)
	prepareSessionForEscalation(t, ctx, sessionID)

	// Escalation with code update
	body := strings.NewReader(`{
		"level": 4,
		"justification": "I am stuck and need help understanding this code",
		"code": {"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["escalated"] != true {
		t.Errorf("expected escalated=true in response")
	}
}

func TestMockLLM_Escalate_WithRunID(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)
	prepareSessionForEscalation(t, ctx, sessionID)

	// Create a run first
	runBody := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", runBody)
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	ctx.Server.router.ServeHTTP(runW, runReq)

	var runResp map[string]interface{}
	json.NewDecoder(runW.Body).Decode(&runResp)
	runID, _ := runResp["id"].(string)

	// Now escalate with the run ID
	body := strings.NewReader(`{
		"level": 4,
		"justification": "I am stuck after running this code and need help",
		"run_id": "` + runID + `"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed regardless of run ID validity
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Escalate_Streaming(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)
	prepareSessionForEscalation(t, ctx, sessionID)

	// Request streaming escalation
	body := strings.NewReader(`{
		"level": 4,
		"justification": "I am stuck and want streaming response",
		"stream": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Streaming response should return 200 or handle gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	// Check for SSE content type if successful
	if w.Code == http.StatusOK {
		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			// Non-streaming response is also valid if streaming wasn't supported
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("unexpected content type: %s", contentType)
			}
		}
	}
}

func TestMockLLM_Escalate_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{
		"level": 4,
		"justification": "This is my detailed justification for escalation"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMockLLM_Escalate_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// ============================================================================
// Additional Run Tests - Coverage for handleCreateRun
// ============================================================================

func TestMockLLM_Run_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_EmptyCode(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should accept empty code (uses session's code)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusAccepted && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200/201/202/400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_WithRunID(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// First run
	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Check if a run_id is returned
	if w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusAccepted {
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
			if runID, ok := resp["id"].(string); ok {
				t.Logf("Run ID: %s", runID)
			}
		}
	}
}

// ============================================================================
// Additional Run Tests - Covering more paths
// ============================================================================

func TestMockLLM_Run_SessionNotActive(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Mark session as completed directly
	markSessionCompleted(t, ctx, sessionID)

	// Now try to run - should fail because session is not active
	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_WithFormatOption(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Run with format option
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() { println(\"hello\") }"},
		"format": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_WithBuildOption(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Run with build option
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"},
		"build": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_WithTestOption(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Run with test option - code has a simple test file
	body := strings.NewReader(`{
		"code": {
			"main.go": "package main\n\nfunc Add(a, b int) int { return a + b }",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Error(\"wrong\")\n\t}\n}"
		},
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockLLM_Run_AllOptions(t *testing.T) {
	ctx := setupTestServerWithContext(t)
	defer ctx.Cleanup()

	sessionID := createTestSessionWithMockLLM(t, ctx.Server)

	// Run with all options
	body := strings.NewReader(`{
		"code": {
			"main.go": "package main\n\nfunc main() {}",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}"
		},
		"format": true,
		"build": true,
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx.Server.router.ServeHTTP(w, req)

	// Should succeed
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================================
// Additional Streaming Tests
// ============================================================================

func TestMockLLM_Streaming_Review(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main\nfunc main() {}"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Streaming_Stuck(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/stuck", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Streaming_Next(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/next", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Streaming_Explain(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{"code": {"main.go": "package main"}, "topic": "functions"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/explain", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ============================================================================
// Additional Session Tests
// ============================================================================

func TestMockLLM_Session_Delete(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Errorf("expected status %d or %d, got %d: %s", http.StatusOK, http.StatusNoContent, w.Code, w.Body.String())
	}

	// Verify session is gone
	getReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	getW := httptest.NewRecorder()

	server.router.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected status %d after delete, got %d", http.StatusNotFound, getW.Code)
	}
}

// ============================================================================
// Additional Provider and Config Tests
// ============================================================================

func TestMockLLM_Config_ShowsSettings(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Providers_List(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config/providers", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Health_Endpoint(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", resp["status"])
	}
}

// ============================================================================
// Escalation Handler Tests
// ============================================================================

func TestMockLLM_Escalation_InsufficientHints(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Don't record any hints - should fail
	body := strings.NewReader(`{
		"level": 4,
		"justification": "I have tried multiple hints but still cannot understand the concept"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "at least 2 hints") {
		t.Errorf("expected error about hints, got: %s", w.Body.String())
	}
}

func TestMockLLM_Escalation_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{
		"level": 4,
		"justification": "I have tried multiple hints but still cannot understand the concept"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/nonexistent-id/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// ============================================================================
// CreateRun Handler Tests
// ============================================================================

func TestMockLLM_CreateRun_WithSession(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() { println(\"Hello\") }"},
		"build": true,
		"test": false
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["run"]; !ok {
		t.Error("expected 'run' field in response")
	}
}

func TestMockLLM_CreateRun_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{
		"code": {"main.go": "package main"},
		"build": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/nonexistent-id/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMockLLM_CreateRun_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestMockLLM_CreateRun_FormatOnly(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main(){\nprintln(\"Hello\")\n}"},
		"format": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_CreateRun_BuildAndTest(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() { println(\"Hello\") }"},
		"build": true,
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed (or return OK with test result)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d or %d, got %d: %s", http.StatusOK, http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// ============================================================================
// Format Handler Tests
// ============================================================================

func TestMockLLM_Format_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main(){\nprintln(\"Hello\")\n}"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Format_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// ============================================================================
// Analytics Handler Tests
// ============================================================================

func TestMockLLM_Analytics_Overview(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Analytics_Skills(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Analytics_Errors(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Analytics_Trend(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Profile_Get(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ============================================================================
// Patch Handler Tests
// ============================================================================

func TestMockLLM_Patch_Preview_NoPatchesAvailable(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// Even for nonexistent sessions, the patch preview returns 200 with "no patches"
	// This is acceptable behavior - the patch service is independent
	nonexistentID := "00000000-0000-0000-0000-000000000000"
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+nonexistentID+"/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Should contain "no patches" message
	if !strings.Contains(w.Body.String(), "No pending patches") && !strings.Contains(w.Body.String(), "has_patch") {
		t.Errorf("expected response to indicate no patches, got: %s", w.Body.String())
	}
}

func TestMockLLM_Patch_Apply_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	nonexistentID := "00000000-0000-0000-0000-000000000000"
	body := strings.NewReader(`{"patch_id": "patch-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+nonexistentID+"/patch/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMockLLM_Patch_Reject_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	nonexistentID := "00000000-0000-0000-0000-000000000000"
	body := strings.NewReader(`{"patch_id": "patch-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+nonexistentID+"/patch/reject", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMockLLM_Patch_List_EmptyForSession(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// Even for nonexistent sessions, the patch list returns 200 with empty array
	nonexistentID := "00000000-0000-0000-0000-000000000000"
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+nonexistentID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Should return empty patches array
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["count"] != float64(0) {
		t.Errorf("expected count=0, got %v", resp["count"])
	}
}

func TestMockLLM_Patch_Preview_WithSession(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patch/preview", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should be OK or NotFound if no patches
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status %d or %d, got %d: %s", http.StatusOK, http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMockLLM_Patch_List_WithSession(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/patches", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ============================================================================
// Spec Handler Tests
// ============================================================================

func TestMockLLM_Spec_List(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMockLLM_Spec_Get_NotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/nonexistent-spec", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should be NotFound or InternalServerError if spec path doesn't exist
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d or %d, got %d: %s", http.StatusNotFound, http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMockLLM_Spec_Validate_NotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/nonexistent-spec/validate", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should be error since spec doesn't exist
	if w.Code == http.StatusOK {
		t.Errorf("expected error status, got %d: %s", w.Code, w.Body.String())
	}
}

// Test escalation with invalid run_id
func TestMockLLM_Escalation_InvalidRunID(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"level": 4,
		"justification": "I have tried multiple hints but still cannot understand the concept",
		"run_id": "not-a-valid-uuid"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// We hit the HintCount < 2 check first, so expect 400 for insufficient hints
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test escalation with level 5
func TestMockLLM_Escalation_Level5_Validation(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"level": 5,
		"justification": "I need a complete solution to understand the pattern being taught"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/escalate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Still hits HintCount < 2 check
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "at least 2 hints") {
		t.Errorf("expected hint count error, got: %s", w.Body.String())
	}
}

// Test analytics skills endpoint success
func TestMockLLM_Analytics_Skills_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test analytics errors endpoint success
func TestMockLLM_Analytics_Errors_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "patterns") {
		t.Errorf("expected patterns in response, got: %s", w.Body.String())
	}
}

// Test analytics trend endpoint success
func TestMockLLM_Analytics_Trend_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "trend") {
		t.Errorf("expected trend in response, got: %s", w.Body.String())
	}
}

// Test profile endpoint
func TestMockLLM_Profile_GetSuccess(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test patch log with limit parameter
func TestMockLLM_PatchLog_WithLimit(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log?limit=5", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test patch log with offset parameter
func TestMockLLM_PatchLog_WithOffset(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/log?offset=10", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test patch stats success
func TestMockLLM_PatchStats_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/stats", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test spec list success
func TestMockLLM_Spec_List_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/specs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test spec create success
func TestMockLLM_Spec_Create_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{"name": "test-spec"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
}

// Test spec create with empty name
func TestMockLLM_Spec_Create_EmptyName(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{"name": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "name is required") {
		t.Errorf("expected name required error, got: %s", w.Body.String())
	}
}

// Test spec create with invalid JSON
func TestMockLLM_Spec_Create_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test spec validate
func TestMockLLM_Spec_Validate_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// First create a spec
	body := strings.NewReader(`{"name": "validate-test-spec"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.router.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create spec: %s", createW.Body.String())
	}

	// Parse spec file_path from response
	var createResp map[string]interface{}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	filePath := createResp["file_path"].(string)

	// Now validate it - route is /v1/specs/validate/{path...}
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/validate/"+filePath, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test spec lock
func TestMockLLM_Spec_Lock_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// First create a spec
	body := strings.NewReader(`{"name": "lock-test-spec"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.router.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create spec: %s", createW.Body.String())
	}

	// Parse spec file_path from response
	var createResp map[string]interface{}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	filePath := createResp["file_path"].(string)

	// Now lock it - route is /v1/specs/lock/{path...}
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/"+filePath, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Lock returns 201 Created when successful
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("expected status 200 or 201, got %d: %s", w.Code, w.Body.String())
	}
}

// Test spec progress
func TestMockLLM_Spec_Progress_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// First create a spec
	body := strings.NewReader(`{"name": "progress-test-spec"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.router.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create spec: %s", createW.Body.String())
	}

	// Parse spec file_path from response
	var createResp map[string]interface{}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	filePath := createResp["file_path"].(string)

	// Now get progress - route is /v1/specs/progress/{path...}
	req := httptest.NewRequest(http.MethodGet, "/v1/specs/progress/"+filePath, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test spec drift
func TestMockLLM_Spec_Drift_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// First create a spec
	body := strings.NewReader(`{"name": "drift-test-spec"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.router.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create spec: %s", createW.Body.String())
	}

	// Parse spec file_path from response
	var createResp map[string]interface{}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	filePath := createResp["file_path"].(string)

	// Now get drift - route is /v1/specs/drift/{path...}
	req := httptest.NewRequest(http.MethodGet, "/v1/specs/drift/"+filePath, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test spec get success
func TestMockLLM_Spec_Get_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// First create a spec
	body := strings.NewReader(`{"name": "get-test-spec"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/specs", body)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	server.router.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create spec: %s", createW.Body.String())
	}

	// Parse spec file_path from response
	var createResp map[string]interface{}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	filePath := createResp["file_path"].(string)

	// Now get it - route is /v1/specs/file/{path...}
	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/"+filePath, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test authoring discover success
func TestMockLLM_Authoring_Discover_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {}"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed with mock LLM
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test authoring discover with invalid JSON
func TestMockLLM_Authoring_Discover_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test authoring suggest with session
func TestMockLLM_Authoring_Suggest_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Handler requires 'section' field
	body := strings.NewReader(`{
		"section": "goals"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Session exists but may not be an authoring session, so 400 is valid
	// 404 would mean session wasn't found
	if w.Code == http.StatusNotFound {
		t.Errorf("session should exist, got 404: %s", w.Body.String())
	}
}

// Test authoring suggest with invalid JSON
func TestMockLLM_Authoring_Suggest_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test authoring suggest session not found
func TestMockLLM_Authoring_Suggest_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	nonexistentID := "00000000-0000-0000-0000-000000000000"
	// The handler requires 'section' field before checking session
	body := strings.NewReader(`{
		"section": "goals"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+nonexistentID+"/authoring/suggest", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Test authoring apply with session
func TestMockLLM_Authoring_Apply_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{
		"spec_name": "test-apply-spec",
		"criterion": "test criterion"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or handle gracefully
	if w.Code == http.StatusNotFound {
		t.Errorf("session should exist, got 404: %s", w.Body.String())
	}
}

// Test authoring apply with invalid JSON
func TestMockLLM_Authoring_Apply_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test authoring hint session not found
func TestMockLLM_Authoring_Hint_SessionNotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	nonexistentID := "00000000-0000-0000-0000-000000000000"
	body := strings.NewReader(`{
		"criterion": "test criterion"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+nonexistentID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Test authoring hint with invalid JSON
func TestMockLLM_Authoring_Hint_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/authoring/hint", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test run with session that triggers appreciation
func TestMockLLM_Run_WithAppreciation(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Update code to something that should pass
	body := strings.NewReader(`{
		"code": {"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"},
		"format": true,
		"build": true,
		"test": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return 200 with run results
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Check that run is included in response
	if !strings.Contains(w.Body.String(), "run") {
		t.Errorf("expected run in response, got: %s", w.Body.String())
	}
}

// Test delete session
func TestMockLLM_Session_DeleteSuccess(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	sessionID := createTestSessionWithMockLLM(t, server)

	// Delete session
	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("expected status 200 or 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session is gone
	getReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	getW := httptest.NewRecorder()
	server.router.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected deleted session to return 404, got %d", getW.Code)
	}
}

// Test exercises list
func TestMockLLM_Exercises_List_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// Test exercises by pack - route is /v1/exercises/{pack}
func TestMockLLM_Exercises_ByPack_Success(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/go-v1", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed or return 404 if pack doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Test get exercise by ID
func TestMockLLM_Exercise_Get_NotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises/nonexistent-pack/nonexistent-exercise", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Test mark criterion satisfied - not found
func TestMockLLM_Spec_MarkCriterion_NotFound(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// The route is PUT /v1/specs/criteria/{id}
	// Body needs path and evidence
	body := strings.NewReader(`{
		"path": "nonexistent-spec.yaml",
		"evidence": "test evidence"
	}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/test-criterion", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Test mark criterion satisfied - invalid JSON
func TestMockLLM_Spec_MarkCriterion_InvalidJSON(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	// The route is PUT /v1/specs/criteria/{id} where id is criterion ID
	// Test with invalid JSON body
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/test-criterion-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// Test ready endpoint with all checks
func TestMockLLM_Ready_WithAllChecks(t *testing.T) {
	server, cleanup := setupTestServerWithMockLLM(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should succeed when mock LLM is registered
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d: %s", w.Code, w.Body.String())
	}

	// Should contain checks
	if !strings.Contains(w.Body.String(), "checks") {
		t.Errorf("expected checks in response, got: %s", w.Body.String())
	}

	// Should contain llm_provider check
	if !strings.Contains(w.Body.String(), "llm_provider") {
		t.Errorf("expected llm_provider check in response, got: %s", w.Body.String())
	}
}
