package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
)

// Server represents the Temper daemon HTTP server
type Server struct {
	cfg      *config.LocalConfig
	server   *http.Server
	router   *http.ServeMux

	// Services
	llmRegistry    *llm.Registry
	exerciseLoader *exercise.Loader
	runnerExecutor runner.Executor
	sessionService *session.Service
}

// ServerConfig holds configuration for creating a new server
type ServerConfig struct {
	Config        *config.LocalConfig
	ExercisePath  string // Primary exercise path
	SessionsPath  string // Path for session storage
}

// NewServer creates a new daemon server
func NewServer(ctx context.Context, cfg ServerConfig) (*Server, error) {
	s := &Server{
		cfg:    cfg.Config,
		router: http.NewServeMux(),
	}

	// Initialize LLM registry
	registry := llm.NewRegistry()
	if err := s.setupLLMProviders(registry); err != nil {
		return nil, fmt.Errorf("setup llm providers: %w", err)
	}
	s.llmRegistry = registry

	// Initialize exercise loader
	s.exerciseLoader = exercise.NewLoader(cfg.ExercisePath)

	// Initialize runner (Docker executor)
	if cfg.Config.Runner.Executor == "docker" {
		dockerCfg := runner.DockerConfig{
			BaseImage:  cfg.Config.Runner.Docker.Image,
			MemoryMB:   int64(cfg.Config.Runner.Docker.MemoryMB),
			CPULimit:   cfg.Config.Runner.Docker.CPULimit,
			NetworkOff: cfg.Config.Runner.Docker.NetworkOff,
			Timeout:    time.Duration(cfg.Config.Runner.Docker.TimeoutSeconds) * time.Second,
		}
		executor, err := runner.NewDockerExecutor(dockerCfg)
		if err != nil {
			slog.Warn("Docker executor not available, using local executor", "error", err)
			s.runnerExecutor = runner.NewLocalExecutor("")
		} else {
			s.runnerExecutor = executor
		}
	} else {
		s.runnerExecutor = runner.NewLocalExecutor("")
	}

	// Initialize session service
	sessionsPath := cfg.SessionsPath
	if sessionsPath == "" {
		temperDir, err := config.TemperDir()
		if err != nil {
			return nil, fmt.Errorf("get temper dir: %w", err)
		}
		sessionsPath = filepath.Join(temperDir, "sessions")
	}

	sessionStore, err := session.NewStore(sessionsPath)
	if err != nil {
		return nil, fmt.Errorf("create session store: %w", err)
	}
	s.sessionService = session.NewService(sessionStore, s.exerciseLoader, s.runnerExecutor)

	// Setup routes
	s.setupRoutes()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Config.Daemon.Bind, cfg.Config.Daemon.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second, // Long for SSE
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

// setupLLMProviders initializes configured LLM providers
func (s *Server) setupLLMProviders(registry *llm.Registry) error {
	for name, providerCfg := range s.cfg.LLM.Providers {
		if !providerCfg.Enabled {
			continue
		}

		switch name {
		case "claude":
			if providerCfg.APIKey == "" {
				slog.Debug("Claude provider enabled but no API key set")
				continue
			}
			provider := llm.NewClaudeProvider(llm.ClaudeConfig{
				APIKey: providerCfg.APIKey,
				Model:  providerCfg.Model,
			})
			registry.Register("claude", provider)
			slog.Info("registered LLM provider", "name", "claude", "model", providerCfg.Model)

		case "openai":
			if providerCfg.APIKey == "" {
				slog.Debug("OpenAI provider enabled but no API key set")
				continue
			}
			provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
				APIKey: providerCfg.APIKey,
				Model:  providerCfg.Model,
			})
			registry.Register("openai", provider)
			slog.Info("registered LLM provider", "name", "openai", "model", providerCfg.Model)

		case "ollama":
			provider := llm.NewOllamaProvider(llm.OllamaConfig{
				BaseURL: providerCfg.URL,
				Model:   providerCfg.Model,
			})
			registry.Register("ollama", provider)
			slog.Info("registered LLM provider", "name", "ollama", "model", providerCfg.Model)
		}
	}

	return nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health & status
	s.router.HandleFunc("GET /v1/health", s.handleHealth)
	s.router.HandleFunc("GET /v1/status", s.handleStatus)

	// Config
	s.router.HandleFunc("GET /v1/config", s.handleGetConfig)
	s.router.HandleFunc("GET /v1/config/providers", s.handleListProviders)

	// Exercises
	s.router.HandleFunc("GET /v1/exercises", s.handleListExercises)
	s.router.HandleFunc("GET /v1/exercises/{pack}", s.handleListPackExercises)
	s.router.HandleFunc("GET /v1/exercises/{pack}/{slug...}", s.handleGetExercise)

	// Sessions (to be implemented in session package)
	s.router.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	s.router.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	s.router.HandleFunc("DELETE /v1/sessions/{id}", s.handleDeleteSession)

	// Runs
	s.router.HandleFunc("POST /v1/sessions/{id}/runs", s.handleCreateRun)
	s.router.HandleFunc("POST /v1/sessions/{id}/format", s.handleFormat)

	// Pairing
	s.router.HandleFunc("POST /v1/sessions/{id}/hint", s.handleHint)
	s.router.HandleFunc("POST /v1/sessions/{id}/review", s.handleReview)
	s.router.HandleFunc("POST /v1/sessions/{id}/stuck", s.handleStuck)
	s.router.HandleFunc("POST /v1/sessions/{id}/next", s.handleNext)
	s.router.HandleFunc("POST /v1/sessions/{id}/explain", s.handleExplain)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	slog.Info("starting temper daemon",
		"addr", s.server.Addr,
		"llm_providers", s.llmRegistry.List(),
	)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down daemon...")

	// Close executor
	if closer, ok := s.runnerExecutor.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			slog.Warn("failed to close executor", "error", err)
		}
	}

	return s.server.Shutdown(ctx)
}

// Handler implementations

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "running",
		"version":       "0.1.0",
		"llm_providers": s.llmRegistry.List(),
		"runner":        s.cfg.Runner.Executor,
	})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return config without secrets
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"daemon":           s.cfg.Daemon,
		"learning_contract": s.cfg.Learning,
		"runner":           s.cfg.Runner,
		"default_provider": s.cfg.LLM.DefaultProvider,
	})
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]map[string]interface{}, 0)
	for name, cfg := range s.cfg.LLM.Providers {
		providers = append(providers, map[string]interface{}{
			"name":       name,
			"enabled":    cfg.Enabled,
			"model":      cfg.Model,
			"configured": cfg.APIKey != "" || name == "ollama",
		})
	}
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"default":   s.cfg.LLM.DefaultProvider,
		"providers": providers,
	})
}

func (s *Server) handleListExercises(w http.ResponseWriter, r *http.Request) {
	packs, err := s.exerciseLoader.LoadAllPacks()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to load exercises", err)
		return
	}

	result := make([]map[string]interface{}, 0, len(packs))
	for _, pack := range packs {
		result = append(result, map[string]interface{}{
			"id":             pack.ID,
			"name":           pack.Name,
			"description":    pack.Description,
			"language":       pack.Language,
			"exercise_count": len(pack.ExerciseIDs),
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"packs": result,
	})
}

func (s *Server) handleListPackExercises(w http.ResponseWriter, r *http.Request) {
	packID := r.PathValue("pack")

	exercises, err := s.exerciseLoader.LoadPackExercises(packID)
	if err != nil {
		s.jsonError(w, http.StatusNotFound, "pack not found", err)
		return
	}

	result := make([]map[string]interface{}, 0, len(exercises))
	for _, ex := range exercises {
		result = append(result, map[string]interface{}{
			"id":         ex.ID,
			"title":      ex.Title,
			"difficulty": ex.Difficulty,
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"pack_id":   packID,
		"exercises": result,
	})
}

func (s *Server) handleGetExercise(w http.ResponseWriter, r *http.Request) {
	packID := r.PathValue("pack")
	slug := r.PathValue("slug")

	ex, err := s.exerciseLoader.LoadExercise(packID, slug)
	if err != nil {
		s.jsonError(w, http.StatusNotFound, "exercise not found", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, ex)
}

// Session handlers

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExerciseID string `json:"exercise_id"`
		Track      string `json:"track,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.ExerciseID == "" {
		s.jsonError(w, http.StatusBadRequest, "exercise_id is required", nil)
		return
	}

	// Get learning policy from config
	var policy *domain.LearningPolicy
	if req.Track != "" {
		if track, ok := s.cfg.Learning.Tracks[req.Track]; ok {
			policy = &domain.LearningPolicy{
				MaxLevel:        domain.InterventionLevel(track.MaxLevel),
				CooldownSeconds: track.CooldownSeconds,
				Track:           req.Track,
			}
		}
	}

	sess, err := s.sessionService.Create(r.Context(), session.CreateRequest{
		ExerciseID: req.ExerciseID,
		Policy:     policy,
	})
	if err != nil {
		if err == session.ErrExerciseNotFound {
			s.jsonError(w, http.StatusNotFound, "exercise not found", err)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to create session", err)
		return
	}

	s.jsonResponse(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.sessionService.Get(r.Context(), id)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.sessionService.Delete(r.Context(), id); err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to delete session", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
	})
}

// Run handlers

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req struct {
		Code   map[string]string `json:"code"`
		Format bool              `json:"format"`
		Build  bool              `json:"build"`
		Test   bool              `json:"test"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// If session ID is provided, use session service
	if sessionID != "" {
		run, err := s.sessionService.RunCode(r.Context(), sessionID, session.RunRequest{
			Code:   req.Code,
			Format: req.Format,
			Build:  req.Build,
			Test:   req.Test,
		})
		if err != nil {
			if err == session.ErrSessionNotFound {
				s.jsonError(w, http.StatusNotFound, "session not found", nil)
				return
			}
			if err == session.ErrSessionNotActive {
				s.jsonError(w, http.StatusBadRequest, "session is not active", nil)
				return
			}
			s.jsonError(w, http.StatusInternalServerError, "run failed", err)
			return
		}

		s.jsonResponse(w, http.StatusOK, run)
		return
	}

	// Standalone run (no session)
	ctx := r.Context()
	result := make(map[string]interface{})

	if req.Format {
		formatResult, err := s.runnerExecutor.RunFormat(ctx, req.Code)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, "format check failed", err)
			return
		}
		result["format"] = map[string]interface{}{
			"ok":   formatResult.OK,
			"diff": formatResult.Diff,
		}
	}

	if req.Build {
		buildResult, err := s.runnerExecutor.RunBuild(ctx, req.Code)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, "build check failed", err)
			return
		}
		result["build"] = map[string]interface{}{
			"ok":     buildResult.OK,
			"output": buildResult.Output,
		}

		if !buildResult.OK {
			s.jsonResponse(w, http.StatusOK, result)
			return
		}
	}

	if req.Test {
		testResult, err := s.runnerExecutor.RunTests(ctx, req.Code, []string{"-v"})
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, "test run failed", err)
			return
		}
		result["test"] = map[string]interface{}{
			"ok":       testResult.OK,
			"output":   testResult.Output,
			"duration": testResult.Duration.String(),
		}
	}

	s.jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleFormat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code map[string]string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	formatted, err := s.runnerExecutor.RunFormatFix(r.Context(), req.Code)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "format failed", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"formatted": formatted,
	})
}

// Pairing handlers (placeholder - will use pairing service)

func (s *Server) handleHint(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotImplemented, "pairing not yet implemented", nil)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotImplemented, "pairing not yet implemented", nil)
}

func (s *Server) handleStuck(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotImplemented, "pairing not yet implemented", nil)
}

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotImplemented, "pairing not yet implemented", nil)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, http.StatusNotImplemented, "pairing not yet implemented", nil)
}

// Helper methods

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func (s *Server) jsonError(w http.ResponseWriter, status int, message string, err error) {
	response := map[string]interface{}{
		"error":   message,
		"status":  status,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	s.jsonResponse(w, status, response)
}
