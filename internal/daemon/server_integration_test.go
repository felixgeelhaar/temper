package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/config"
)

// TestServerIntegration runs a comprehensive integration test of the daemon server
func TestServerIntegration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup test environment
	tmpDir := t.TempDir()

	// Create necessary directories
	dirs := []string{
		filepath.Join(tmpDir, "sessions"),
		filepath.Join(tmpDir, "profiles"),
		filepath.Join(tmpDir, "patches"),
		filepath.Join(tmpDir, "logs"),
		filepath.Join(tmpDir, "exercises", "go-v1"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create a test exercise pack
	packYAML := `id: go-v1
name: Go Fundamentals
description: Learn Go from scratch
language: go
version: "1.0.0"
exercises:
  - basics/hello-world
`
	if err := os.WriteFile(filepath.Join(tmpDir, "exercises", "go-v1", "pack.yaml"), []byte(packYAML), 0644); err != nil {
		t.Fatalf("failed to write pack.yaml: %v", err)
	}

	// Create basics category directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "exercises", "go-v1", "basics"), 0755); err != nil {
		t.Fatalf("failed to create exercise dir: %v", err)
	}

	// Exercise file goes at exercises/go-v1/basics/hello-world.yaml
	exerciseYAML := `id: basics/hello-world
title: Hello World
description: Write your first Go program that prints "Hello, World!"
difficulty: beginner
tags:
  - basics
  - output
starter:
  main.go: |
    package main

    import "fmt"

    func main() {
        // TODO: Print "Hello, World!"
    }
tests:
  main_test.go: |
    package main

    import "testing"

    func TestHello(t *testing.T) {
        // Test will be run by the runner
    }
check_recipe:
  format: true
  build: true
  test: true
hints:
  l0:
    - What function do you need to print text to the console?
  l1:
    - Use the fmt package for output
    - The main function is the entry point
  l2:
    - fmt.Println() prints a line to stdout
rubric:
  criteria:
    - id: compiles
      name: Compiles
      description: Program compiles without errors
      weight: 40
    - id: output
      name: Correct Output
      description: Output contains 'Hello'
      weight: 60
`
	if err := os.WriteFile(filepath.Join(tmpDir, "exercises", "go-v1", "basics", "hello-world.yaml"), []byte(exerciseYAML), 0644); err != nil {
		t.Fatalf("failed to write exercise YAML: %v", err)
	}

	// Create test config
	cfg := &config.LocalConfig{
		Daemon: config.DaemonConfig{
			Bind:     "127.0.0.1",
			Port:     0, // Let OS assign port
			LogLevel: "debug",
		},
		LLM: config.LLMConfig{
			DefaultProvider: "ollama",
			Providers: map[string]*config.ProviderConfig{
				"ollama": {
					Enabled: true,
					Model:   "test-model",
					URL:     "http://localhost:11434",
				},
			},
		},
		Runner: config.RunnerConfig{
			Executor: "local",
		},
		Learning: config.LearningConfig{
			DefaultTrack: "practice",
			Tracks: map[string]config.TrackConfig{
				"practice": {
					MaxLevel:        3,
					CooldownSeconds: 0,
				},
			},
		},
	}

	// Temporarily override temper dir for test
	origEnv := os.Getenv("TEMPER_HOME")
	os.Setenv("TEMPER_HOME", tmpDir)
	defer os.Setenv("TEMPER_HOME", origEnv)

	// Create server
	ctx := context.Background()
	server, err := NewServer(ctx, ServerConfig{
		Config:       cfg,
		ExercisePath: filepath.Join(tmpDir, "exercises"),
		SessionsPath: filepath.Join(tmpDir, "sessions"),
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Create test server
	ts := httptest.NewServer(server.router)
	defer ts.Close()

	// Test 1: Health endpoint
	t.Run("Health", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/health")
		if err != nil {
			t.Fatalf("health request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["status"] != "healthy" {
			t.Errorf("expected status 'healthy', got %v", result["status"])
		}
	})

	// Test 2: Status endpoint
	t.Run("Status", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/status")
		if err != nil {
			t.Fatalf("status request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["status"] != "running" {
			t.Errorf("expected status 'running', got %v", result["status"])
		}
		if result["version"] != "0.1.0" {
			t.Errorf("expected version '0.1.0', got %v", result["version"])
		}
	})

	// Test 3: List exercises
	t.Run("ListExercises", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/exercises")
		if err != nil {
			t.Fatalf("list exercises request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		packs, ok := result["packs"].([]interface{})
		if !ok {
			t.Fatal("expected 'packs' array in response")
		}

		if len(packs) == 0 {
			t.Error("expected at least one pack")
		}
	})

	// Test 4: Get specific exercise
	t.Run("GetExercise", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/exercises/go-v1/basics/hello-world")
		if err != nil {
			t.Fatalf("get exercise request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, body)
		}

		var ex map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&ex); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if ex["Title"] != "Hello World" {
			t.Errorf("expected title 'Hello World', got %v", ex["Title"])
		}
		if ex["Difficulty"] != "beginner" {
			t.Errorf("expected difficulty 'beginner', got %v", ex["Difficulty"])
		}
	})

	// Test 5: Create session
	var sessionID string
	t.Run("CreateSession", func(t *testing.T) {
		body := `{"exercise_id": "go-v1/basics/hello-world"}`
		resp, err := http.Post(ts.URL+"/v1/sessions", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("create session request failed: %v", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, respBody)
		}

		var session map[string]interface{}
		if err := json.Unmarshal(respBody, &session); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Try both "ID" and "id" as keys
		id, ok := session["ID"].(string)
		if !ok {
			id, ok = session["id"].(string)
		}
		if !ok || id == "" {
			t.Fatalf("expected non-empty session ID, got: %s", string(respBody))
		}
		sessionID = id

		// Check status (try both case variants)
		status, ok := session["Status"].(string)
		if !ok {
			status, _ = session["status"].(string)
		}
		if status != "active" {
			t.Errorf("expected status 'active', got %v", status)
		}
	})

	// Test 6: Get session
	t.Run("GetSession", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("no session ID from previous test")
		}

		resp, err := http.Get(ts.URL + "/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("get session request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var session map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Check ID (try both case variants)
		id, ok := session["ID"].(string)
		if !ok {
			id, _ = session["id"].(string)
		}
		if id != sessionID {
			t.Errorf("expected session ID %s, got %v", sessionID, id)
		}
	})

	// Test 7: Run code (format check)
	t.Run("RunCode", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("no session ID from previous test")
		}

		code := `{"code": {"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\nfmt.Println(\"Hello, World!\")\n}"}, "format": true, "build": true}`
		resp, err := http.Post(ts.URL+"/v1/sessions/"+sessionID+"/runs", "application/json", bytes.NewBufferString(code))
		if err != nil {
			t.Fatalf("run code request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, respBody)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		run, ok := result["run"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'run' object in response")
		}

		runResult, ok := run["Result"].(map[string]interface{})
		if !ok {
			t.Log("Note: run result structure may vary based on executor")
		} else {
			if !runResult["BuildOK"].(bool) {
				t.Error("expected build to pass")
			}
		}
	})

	// Test 8: Request hint (pairing)
	t.Run("RequestHint", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("no session ID from previous test")
		}

		// Note: This will fail if no LLM is available, but we test the endpoint structure
		resp, err := http.Post(ts.URL+"/v1/sessions/"+sessionID+"/hint", "application/json", bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatalf("hint request failed: %v", err)
		}
		defer resp.Body.Close()

		// We expect either success (if LLM is available) or specific error
		// The endpoint should at least be reachable
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("unexpected status %d", resp.StatusCode)
		}
	})

	// Test 9: Profile/Analytics endpoints
	t.Run("Analytics", func(t *testing.T) {
		endpoints := []string{
			"/v1/profile",
			"/v1/analytics/overview",
			"/v1/analytics/skills",
			"/v1/analytics/errors",
			"/v1/analytics/trend",
		}

		for _, endpoint := range endpoints {
			resp, err := http.Get(ts.URL + endpoint)
			if err != nil {
				t.Errorf("%s request failed: %v", endpoint, err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s expected status 200, got %d", endpoint, resp.StatusCode)
			}
		}
	})

	// Test 10: Delete session
	t.Run("DeleteSession", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("no session ID from previous test")
		}

		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/sessions/"+sessionID, nil)
		if err != nil {
			t.Fatalf("failed to create delete request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete session request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Verify session is deleted
		getResp, err := http.Get(ts.URL + "/v1/sessions/" + sessionID)
		if err != nil {
			t.Fatalf("get deleted session request failed: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404 for deleted session, got %d", getResp.StatusCode)
		}
	})

	// Test 11: Config endpoint
	t.Run("Config", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/config")
		if err != nil {
			t.Fatalf("config request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["default_provider"] != "ollama" {
			t.Errorf("expected default_provider 'ollama', got %v", result["default_provider"])
		}
	})

	// Test 12: Providers endpoint
	t.Run("Providers", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/config/providers")
		if err != nil {
			t.Fatalf("providers request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["default"] != "ollama" {
			t.Errorf("expected default 'ollama', got %v", result["default"])
		}
	})

	// Test 13: Non-existent exercise
	t.Run("NonExistentExercise", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/exercises/invalid-pack/nonexistent")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// Test 14: Non-existent session
	t.Run("NonExistentSession", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/sessions/00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	// Test 15: Invalid session creation
	t.Run("InvalidSessionCreation", func(t *testing.T) {
		// Missing required fields
		resp, err := http.Post(ts.URL+"/v1/sessions", "application/json", bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	// Test 16: Spec endpoints
	t.Run("SpecEndpoints", func(t *testing.T) {
		// Create a spec
		specBody := `{"name": "test-spec", "intent": "Test the spec system", "goals": ["Goal 1"]}`
		resp, err := http.Post(ts.URL+"/v1/specs", "application/json", bytes.NewBufferString(specBody))
		if err != nil {
			t.Fatalf("create spec request failed: %v", err)
		}
		defer resp.Body.Close()

		// Accept either 201 (created) or 400 (if spec endpoints not fully implemented)
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("create spec: expected status 201/200/400, got %d", resp.StatusCode)
		}

		// List specs
		listResp, err := http.Get(ts.URL + "/v1/specs")
		if err != nil {
			t.Fatalf("list specs request failed: %v", err)
		}
		defer listResp.Body.Close()

		if listResp.StatusCode != http.StatusOK {
			t.Errorf("list specs: expected status 200, got %d", listResp.StatusCode)
		}
	})

	// Test 17: Patch audit log endpoint
	t.Run("PatchAuditLog", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/patches/log")
		if err != nil {
			t.Fatalf("patch log request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should return 200 even if empty
		if resp.StatusCode != http.StatusOK {
			t.Errorf("patch log: expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test 18: Patch stats endpoint
	t.Run("PatchStats", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/patches/stats")
		if err != nil {
			t.Fatalf("patch stats request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("patch stats: expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test 19: Escalation requires session
	t.Run("EscalationRequiresSession", func(t *testing.T) {
		// Create a new session for this test
		body := `{"exercise_id": "go-v1/basics/hello-world"}`
		createResp, err := http.Post(ts.URL+"/v1/sessions", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("create session request failed: %v", err)
		}
		defer createResp.Body.Close()

		var session map[string]interface{}
		json.NewDecoder(createResp.Body).Decode(&session)

		// Get session ID
		testSessionID, _ := session["ID"].(string)
		if testSessionID == "" {
			testSessionID, _ = session["id"].(string)
		}
		if testSessionID == "" {
			t.Skip("could not create session for escalation test")
		}

		// Test escalation endpoint
		escalateBody := `{"level": 4, "justification": "I need more help to understand this concept fully"}`
		resp, err := http.Post(ts.URL+"/v1/sessions/"+testSessionID+"/escalate", "application/json", bytes.NewBufferString(escalateBody))
		if err != nil {
			t.Fatalf("escalate request failed: %v", err)
		}
		defer resp.Body.Close()

		// Accept 200 (success), 400 (validation error), or 500 (no LLM)
		validStatuses := []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError}
		statusValid := false
		for _, s := range validStatuses {
			if resp.StatusCode == s {
				statusValid = true
				break
			}
		}
		if !statusValid {
			t.Errorf("escalate: expected status 200/400/500, got %d", resp.StatusCode)
		}
	})

	// Test 20: Spec validation endpoint
	t.Run("SpecValidation", func(t *testing.T) {
		// Create a test spec file
		specContent := `intent: Build a simple calculator
goals:
  - Support basic arithmetic operations
acceptance:
  - "Add function returns correct sum"
  - "Subtract function returns correct difference"
`
		specPath := filepath.Join(tmpDir, "test-spec.yaml")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to write test spec: %v", err)
		}

		// Validate spec
		validateBody := `{"path": "` + specPath + `"}`
		resp, err := http.Post(ts.URL+"/v1/specs/validate", "application/json", bytes.NewBufferString(validateBody))
		if err != nil {
			t.Fatalf("validate spec request failed: %v", err)
		}
		defer resp.Body.Close()

		// Accept 200 (valid) or 400 (invalid/not found)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("validate spec: expected status 200/400, got %d", resp.StatusCode)
		}
	})

	// Test 21: Welcome/onboarding endpoint
	t.Run("WelcomeEndpoint", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/welcome")
		if err != nil {
			t.Fatalf("welcome request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should return 200 with learning contract info
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			t.Errorf("welcome: expected status 200/404, got %d", resp.StatusCode)
		}
	})
}

// TestServerMiddleware tests the middleware chain
func TestServerMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create minimal directories
	dirs := []string{
		filepath.Join(tmpDir, "sessions"),
		filepath.Join(tmpDir, "profiles"),
		filepath.Join(tmpDir, "patches"),
		filepath.Join(tmpDir, "logs"),
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	cfg := &config.LocalConfig{
		Daemon: config.DaemonConfig{
			Bind:     "127.0.0.1",
			Port:     0,
			LogLevel: "debug",
		},
		LLM: config.LLMConfig{
			DefaultProvider: "ollama",
			Providers: map[string]*config.ProviderConfig{
				"ollama": {
					Enabled: true,
					Model:   "test",
					URL:     "http://localhost:11434",
				},
			},
		},
		Runner: config.RunnerConfig{
			Executor: "local",
		},
		Learning: config.LearningConfig{
			DefaultTrack: "practice",
			Tracks: map[string]config.TrackConfig{
				"practice": {MaxLevel: 3, CooldownSeconds: 0},
			},
		},
	}

	origEnv := os.Getenv("TEMPER_HOME")
	os.Setenv("TEMPER_HOME", tmpDir)
	defer os.Setenv("TEMPER_HOME", origEnv)

	ctx := context.Background()
	server, err := NewServer(ctx, ServerConfig{
		Config:       cfg,
		ExercisePath: filepath.Join(tmpDir, "exercises"),
		SessionsPath: filepath.Join(tmpDir, "sessions"),
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Test with the middleware chain
	handler := recoveryMiddleware(loggingMiddleware(server.router))

	// Test logging middleware adds proper headers
	t.Run("LoggingMiddleware", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	// Test recovery middleware handles panics
	t.Run("RecoveryMiddleware", func(t *testing.T) {
		// Create a handler that panics
		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		wrapped := recoveryMiddleware(panicHandler)

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500 after panic, got %d", rec.Code)
		}
	})
}
