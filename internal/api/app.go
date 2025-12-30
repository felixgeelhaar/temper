package api

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/auth"
	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/workspace"
)

// App holds all application dependencies
type App struct {
	Config    *config.Config
	DB        *sql.DB
	Auth      *auth.Service
	Exercises *exercise.Registry
	Workspace *workspace.Service
	Runner    *runner.Service
	Pairing   *pairing.Service
	LLM       *llm.Registry
}

// AppConfig holds configuration for application initialization
type AppConfig struct {
	Config       *config.Config
	DB           *sql.DB
	ExercisePath string
}

// NewApp creates a new application instance with all dependencies wired
func NewApp(ctx context.Context, cfg AppConfig) (*App, error) {
	app := &App{
		Config: cfg.Config,
		DB:     cfg.DB,
	}

	// Initialize repositories
	authRepo := NewAuthRepository(cfg.DB)
	workspaceRepo := NewWorkspaceRepository(cfg.DB)

	// Initialize auth service
	sessionMaxAge := 7 * 24 * time.Hour // 7 days
	app.Auth = auth.NewService(authRepo, sessionMaxAge)

	// Initialize exercise registry
	exercisePath := cfg.ExercisePath
	if exercisePath == "" {
		exercisePath = "exercises"
	}
	loader := exercise.NewLoader(exercisePath)
	app.Exercises = exercise.NewRegistry(loader)
	if err := app.Exercises.Load(); err != nil {
		return nil, fmt.Errorf("load exercises: %w", err)
	}

	// Initialize workspace service
	app.Workspace = workspace.NewService(workspaceRepo)

	// Initialize LLM registry
	app.LLM = llm.NewRegistry()
	if err := initLLMProviders(app.LLM, cfg.Config); err != nil {
		return nil, fmt.Errorf("init LLM providers: %w", err)
	}

	// Initialize runner service
	runnerCfg := runner.DefaultConfig()
	runnerCfg.Timeout = time.Duration(cfg.Config.RunnerTimeout) * time.Second
	var executor runner.Executor
	if cfg.Config.Debug {
		executor = runner.NewLocalExecutor(".")
	} else {
		dockerExec, err := runner.NewDockerExecutor(runner.DockerConfig{
			BaseImage:  cfg.Config.RunnerImage,
			MemoryMB:   int64(runnerCfg.MemoryMB),
			CPULimit:   runnerCfg.CPULimit,
			NetworkOff: true,
			Timeout:    runnerCfg.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Docker executor: %w", err)
		}
		executor = dockerExec
	}
	app.Runner = runner.NewService(runnerCfg, executor)

	// Initialize pairing service
	app.Pairing = pairing.NewService(app.LLM, cfg.Config.LLMProvider)

	return app, nil
}

// initLLMProviders sets up LLM providers based on configuration
func initLLMProviders(registry *llm.Registry, cfg *config.Config) error {
	switch cfg.LLMProvider {
	case "claude":
		if cfg.LLMAPIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY required for claude provider")
		}
		provider := llm.NewClaudeProvider(llm.ClaudeConfig{
			APIKey: cfg.LLMAPIKey,
			Model:  cfg.LLMModel,
		})
		registry.Register("claude", provider)
		registry.SetDefault("claude")

	case "openai":
		if cfg.LLMAPIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY required for openai provider")
		}
		provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
			APIKey: cfg.LLMAPIKey,
			Model:  cfg.LLMModel,
		})
		registry.Register("openai", provider)
		registry.SetDefault("openai")

	case "ollama":
		ollamaURL := cfg.OllamaURL
		if ollamaURL == "" {
			ollamaURL = "http://localhost:11434"
		}
		model := cfg.LLMModel
		if model == "" || model == "claude-sonnet-4-20250514" {
			model = "llama3.2:latest"
		}
		provider := llm.NewOllamaProvider(llm.OllamaConfig{
			BaseURL: ollamaURL,
			Model:   model,
		})
		registry.Register("ollama", provider)
		registry.SetDefault("ollama")

	default:
		// Register all available providers
		if cfg.LLMAPIKey != "" {
			if cfg.LLMModel == "" || cfg.LLMModel == "claude-sonnet-4-20250514" {
				provider := llm.NewClaudeProvider(llm.ClaudeConfig{
					APIKey: cfg.LLMAPIKey,
					Model:  "claude-sonnet-4-20250514",
				})
				registry.Register("claude", provider)
				registry.SetDefault("claude")
			} else {
				provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
					APIKey: cfg.LLMAPIKey,
					Model:  cfg.LLMModel,
				})
				registry.Register("openai", provider)
				registry.SetDefault("openai")
			}
		}
		// Always register ollama for local development
		ollamaURL := cfg.OllamaURL
		if ollamaURL == "" {
			ollamaURL = "http://localhost:11434"
		}
		provider := llm.NewOllamaProvider(llm.OllamaConfig{
			BaseURL: ollamaURL,
			Model:   "llama3.2:latest",
		})
		registry.Register("ollama", provider)
		// Only set ollama as default if no other provider was set
		if registry.DefaultName() == "" {
			registry.SetDefault("ollama")
		}
	}

	return nil
}

// Close cleans up application resources
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
