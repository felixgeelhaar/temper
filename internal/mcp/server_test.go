package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
)

// setupTestServer creates a test MCP server with minimal configuration
func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "temper-mcp-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	// Create subdirectories
	for _, dir := range []string{"sessions", "exercises"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("create subdir %s: %v", dir, err)
		}
	}

	// Create exercise loader
	exercisePath := filepath.Join(tmpDir, "exercises")
	loader := exercise.NewLoader(exercisePath)

	// Create executor
	executor := runner.NewLocalExecutor("")

	// Create session store and service
	sessionsPath := filepath.Join(tmpDir, "sessions")
	store, err := session.NewStore(sessionsPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create session store: %v", err)
	}
	sessionService := session.NewService(store, loader, executor)

	// Create LLM registry with mock provider
	registry := llm.NewRegistry()
	registry.Register("mock", &mockProvider{})

	// Create pairing service
	pairingService := pairing.NewService(registry, "mock")

	// Create MCP server
	server := NewServer(Config{
		SessionService: sessionService,
		PairingService: pairingService,
		ExerciseLoader: loader,
	})

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

// mockProvider is a simple mock LLM provider for testing
type mockProvider struct{}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Generate(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: "Mock response",
	}, nil
}

func (m *mockProvider) GenerateStream(ctx context.Context, req *llm.Request) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockProvider) SupportsStreaming() bool {
	return false
}

func TestNewServer(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	if server == nil {
		t.Fatal("expected non-nil server")
	}

	if server.mcpServer == nil {
		t.Fatal("expected non-nil MCP server")
	}

	if server.sessionService == nil {
		t.Fatal("expected non-nil session service")
	}

	if server.pairingService == nil {
		t.Fatal("expected non-nil pairing service")
	}

	if server.exerciseLoader == nil {
		t.Fatal("expected non-nil exercise loader")
	}
}

func TestServerConfig(t *testing.T) {
	cfg := Config{}

	// Test with nil services - should not panic
	server := NewServer(cfg)
	if server == nil {
		t.Fatal("expected non-nil server even with nil config")
	}
}

func TestGetMCPServer(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	mcpServer := server.GetMCPServer()
	if mcpServer == nil {
		t.Fatal("expected non-nil underlying MCP server")
	}
}

func TestInputOutputTypes(t *testing.T) {
	// Test StartInput
	t.Run("StartInput", func(t *testing.T) {
		input := StartInput{
			ExerciseID: "test-pack/basics/hello",
			Track:      "practice",
		}
		if input.ExerciseID == "" {
			t.Error("expected non-empty ExerciseID")
		}
	})

	// Test InterventionInput
	t.Run("InterventionInput", func(t *testing.T) {
		input := InterventionInput{
			SessionID: "test-session-id",
			Code:      map[string]string{"main.go": "package main"},
			Context:   "test context",
		}
		if input.SessionID == "" {
			t.Error("expected non-empty SessionID")
		}
		if len(input.Code) == 0 {
			t.Error("expected non-empty Code map")
		}
	})

	// Test RunInput
	t.Run("RunInput", func(t *testing.T) {
		format := true
		input := RunInput{
			SessionID: "test-session-id",
			Code:      map[string]string{"main.go": "package main"},
			Format:    &format,
		}
		if input.Format == nil || !*input.Format {
			t.Error("expected Format to be true")
		}
	})

	// Test StartOutput
	t.Run("StartOutput", func(t *testing.T) {
		output := StartOutput{
			SessionID:  "test-session-id",
			ExerciseID: "test-pack/hello",
			Track:      "practice",
			Message:    "Session started",
		}
		if output.SessionID == "" {
			t.Error("expected non-empty SessionID")
		}
	})

	// Test InterventionOutput
	t.Run("InterventionOutput", func(t *testing.T) {
		output := InterventionOutput{
			Level:   2,
			Type:    "hint",
			Content: "Try using a for loop",
		}
		if output.Level < 0 {
			t.Error("expected non-negative Level")
		}
	})

	// Test RunOutput
	t.Run("RunOutput", func(t *testing.T) {
		output := RunOutput{
			FormatOK: true,
			BuildOK:  true,
			TestOK:   false,
			Summary:  "Format: ✓ | Build: ✓ | Tests: ✗",
		}
		if !output.FormatOK {
			t.Error("expected FormatOK to be true")
		}
	})

	// Test StatusOutput
	t.Run("StatusOutput", func(t *testing.T) {
		output := StatusOutput{
			SessionID:  "test-id",
			ExerciseID: "pack/exercise",
			Status:     "active",
			RunCount:   5,
			HintCount:  2,
			Track:      "practice",
			MaxLevel:   3,
		}
		if output.MaxLevel < 0 {
			t.Error("expected non-negative MaxLevel")
		}
	})
}
