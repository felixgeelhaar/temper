package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	mcpserver "github.com/felixgeelhaar/temper/internal/mcp"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
)

// cmdMCP starts the MCP server for Cursor integration
func cmdMCP() error {
	// Load configuration
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize LLM registry
	registry := llm.NewRegistry()

	// Setup LLM providers
	for name, providerCfg := range cfg.LLM.Providers {
		if !providerCfg.Enabled || (providerCfg.APIKey == "" && name != "ollama") {
			continue
		}

		switch name {
		case "claude":
			provider := llm.NewClaudeProvider(llm.ClaudeConfig{
				APIKey: providerCfg.APIKey,
				Model:  providerCfg.Model,
			})
			registry.Register("claude", provider)
		case "openai":
			provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
				APIKey: providerCfg.APIKey,
				Model:  providerCfg.Model,
			})
			registry.Register("openai", provider)
		case "ollama":
			provider := llm.NewOllamaProvider(llm.OllamaConfig{
				BaseURL: providerCfg.URL,
				Model:   providerCfg.Model,
			})
			registry.Register("ollama", provider)
		}
	}

	// Initialize exercise loader
	temperDir, err := config.TemperDir()
	if err != nil {
		return fmt.Errorf("get temper dir: %w", err)
	}
	exercisePath := filepath.Join(temperDir, "exercises")
	loader := exercise.NewLoader(exercisePath)

	// Initialize runner (local for MCP - Docker might not be available)
	executor := runner.NewLocalExecutor("")

	// Initialize session store
	sessionsPath := filepath.Join(temperDir, "sessions")
	sessionStore, err := session.NewStore(sessionsPath)
	if err != nil {
		return fmt.Errorf("create session store: %w", err)
	}

	// Create services
	sessionService := session.NewService(sessionStore, loader, executor)
	pairingService := pairing.NewService(registry, cfg.LLM.DefaultProvider)

	// Create MCP server
	mcpSrv := mcpserver.NewServer(mcpserver.Config{
		SessionService: sessionService,
		PairingService: pairingService,
		ExerciseLoader: loader,
	})

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Serve on stdio
	return mcpSrv.ServeStdio(ctx)
}

func checkDocker() error {
	// Check if docker is in PATH
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH")
	}

	// Check if docker daemon is running
	cmd := exec.Command("docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon not running")
	}

	return nil
}

func checkOllama(url string) error {
	if url == "" {
		url = "http://localhost:11434"
	}

	resp, err := http.Get(url + "/api/tags")
	if err != nil {
		return fmt.Errorf("not reachable at %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
