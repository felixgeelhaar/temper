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
)

// setupTestServer creates a test server with minimal configuration
func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "temper-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	// Create subdirectories
	for _, dir := range []string{"sessions", "exercises", "logs"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("create subdir %s: %v", dir, err)
		}
	}

	// Create test config
	cfg := config.DefaultLocalConfig()
	cfg.Daemon.Port = 0 // Let system choose port

	// Create server
	server, err := NewServer(context.Background(), ServerConfig{
		Config:       cfg,
		ExercisePath: filepath.Join(tmpDir, "exercises"),
		SessionsPath: filepath.Join(tmpDir, "sessions"),
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create server: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", resp["status"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Check expected fields
	if resp["status"] != "running" {
		t.Errorf("expected status 'running', got %v", resp["status"])
	}
	if _, ok := resp["version"]; !ok {
		t.Error("expected 'version' field in response")
	}
}

func TestConfigEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestListProvidersEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/config/providers", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["providers"]; !ok {
		t.Error("expected 'providers' field in response")
	}
}

func TestListExercisesEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/exercises", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["packs"]; !ok {
		t.Error("expected 'packs' field in response")
	}
}

func TestCreateSession(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a test exercise pack first
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

	// Write exercise.yaml (at basics/hello.yaml)
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
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sessionID, ok := resp["id"].(string)
	if !ok || sessionID == "" {
		t.Error("expected session ID in response")
	}

	// Get session
	req = httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Delete session
	req = httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestSessionNotFound(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/nonexistent-id", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestMiddleware(t *testing.T) {
	t.Run("logging middleware", func(t *testing.T) {
		handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("recovery middleware", func(t *testing.T) {
		handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}
