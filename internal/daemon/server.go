package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/appreciation"
	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/docindex"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/patch"
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/felixgeelhaar/temper/internal/spec"
	"github.com/google/uuid"
)

// Server represents the Temper daemon HTTP server
type Server struct {
	cfg    *config.LocalConfig
	server *http.Server
	router *http.ServeMux

	// Services (using interfaces for testability)
	llmRegistry         llm.LLMRegistry
	exerciseLoader      *exercise.Loader
	runnerExecutor      runner.Executor
	sessionService      session.SessionService
	pairingService      pairing.PairingService
	profileService      profile.ProfileService
	specService         spec.SpecService
	appreciationService *appreciation.Service
	patchService        patch.PatchService

	// Concrete services needed for internal setup (SetProfileService, SetSpecService)
	sessionServiceConcrete *session.Service
	llmRegistryConcrete    *llm.Registry
	specServiceConcrete    *spec.Service
}

// ServerConfig holds configuration for creating a new server
type ServerConfig struct {
	Config       *config.LocalConfig
	ExercisePath string // Primary exercise path
	SessionsPath string // Path for session storage
	SpecsPath    string // Path for spec storage (workspace root for .specs/)
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
	s.llmRegistryConcrete = registry

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
			if cfg.Config.Runner.AllowLocalFallback {
				slog.Warn("Docker executor not available, using local executor", "error", err)
				s.runnerExecutor = runner.NewLocalExecutor("")
			} else {
				return nil, fmt.Errorf("docker executor unavailable: %w", err)
			}
		} else {
			s.runnerExecutor = executor
		}
	} else {
		s.runnerExecutor = runner.NewLocalExecutor("")
	}

	// Get temper directory for data storage
	temperDir, err := config.TemperDir()
	if err != nil {
		return nil, fmt.Errorf("get temper dir: %w", err)
	}

	// Initialize session service
	sessionsPath := cfg.SessionsPath
	if sessionsPath == "" {
		sessionsPath = filepath.Join(temperDir, "sessions")
	}

	sessionStore, err := session.NewStore(sessionsPath)
	if err != nil {
		return nil, fmt.Errorf("create session store: %w", err)
	}
	sessionSvc := session.NewService(sessionStore, s.exerciseLoader, s.runnerExecutor)
	s.sessionService = sessionSvc
	s.sessionServiceConcrete = sessionSvc

	// Initialize profile service
	profileStore, err := profile.NewStore(filepath.Join(temperDir, "profiles"))
	if err != nil {
		return nil, fmt.Errorf("create profile store: %w", err)
	}
	profileSvc := profile.NewService(profileStore)
	s.profileService = profileSvc

	// Connect profile service to session service for event hooks
	s.sessionServiceConcrete.SetProfileService(profileSvc)

	// Initialize spec service
	specsPath := cfg.SpecsPath
	if specsPath == "" {
		specsPath = "." // Default to current working directory
	}
	specSvc := spec.NewService(specsPath)
	s.specService = specSvc
	s.specServiceConcrete = specSvc

	// Connect spec service to session service for feature guidance
	s.sessionServiceConcrete.SetSpecService(specSvc)

	// Initialize pairing service
	s.pairingService = pairing.NewService(s.llmRegistryConcrete, cfg.Config.LLM.DefaultProvider)

	// Initialize appreciation service
	s.appreciationService = appreciation.NewService()

	// Initialize patch service with logging
	patchLogDir := filepath.Join(temperDir, "patches")
	patchService, err := patch.NewServiceWithLogger(patchLogDir)
	if err != nil {
		slog.Warn("Patch logging not available", "error", err)
		s.patchService = patch.NewService()
	} else {
		s.patchService = patchService
	}

	// Setup routes
	s.setupRoutes()

	// Create HTTP server with middleware chain
	// Order: correlationID -> recovery -> logging -> router
	// This ensures correlation ID is available in all logs and error handling
	addr := fmt.Sprintf("%s:%d", cfg.Config.Daemon.Bind, cfg.Config.Daemon.Port)
	handler := correlationIDMiddleware(recoveryMiddleware(loggingMiddleware(s.router)))
	s.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
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
	s.router.HandleFunc("GET /v1/ready", s.handleReady)
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
	s.router.HandleFunc("POST /v1/sessions/{id}/escalate", s.handleEscalate)

	// Profile & Analytics
	s.router.HandleFunc("GET /v1/profile", s.handleGetProfile)
	s.router.HandleFunc("GET /v1/analytics/overview", s.handleAnalyticsOverview)
	s.router.HandleFunc("GET /v1/analytics/skills", s.handleAnalyticsSkills)
	s.router.HandleFunc("GET /v1/analytics/errors", s.handleAnalyticsErrors)
	s.router.HandleFunc("GET /v1/analytics/trend", s.handleAnalyticsTrend)

	// Specs (Specular format)
	s.router.HandleFunc("POST /v1/specs", s.handleCreateSpec)
	s.router.HandleFunc("GET /v1/specs", s.handleListSpecs)
	s.router.HandleFunc("POST /v1/specs/validate/{path...}", s.handleValidateSpec)
	s.router.HandleFunc("PUT /v1/specs/criteria/{id}", s.handleMarkCriterionSatisfied)
	s.router.HandleFunc("POST /v1/specs/lock/{path...}", s.handleLockSpec)
	s.router.HandleFunc("GET /v1/specs/progress/{path...}", s.handleGetSpecProgress)
	s.router.HandleFunc("GET /v1/specs/drift/{path...}", s.handleGetSpecDrift)
	s.router.HandleFunc("GET /v1/specs/file/{path...}", s.handleGetSpec)

	// Patches
	s.router.HandleFunc("GET /v1/sessions/{id}/patch/preview", s.handlePatchPreview)
	s.router.HandleFunc("POST /v1/sessions/{id}/patch/apply", s.handlePatchApply)
	s.router.HandleFunc("POST /v1/sessions/{id}/patch/reject", s.handlePatchReject)
	s.router.HandleFunc("GET /v1/sessions/{id}/patches", s.handleListPatches)

	// Patch logs
	s.router.HandleFunc("GET /v1/patches/log", s.handlePatchLog)
	s.router.HandleFunc("GET /v1/patches/stats", s.handlePatchStats)

	// Spec Authoring
	s.router.HandleFunc("POST /v1/authoring/discover", s.handleAuthoringDiscover)
	s.router.HandleFunc("POST /v1/sessions/{id}/authoring/suggest", s.handleAuthoringSuggest)
	s.router.HandleFunc("POST /v1/sessions/{id}/authoring/apply", s.handleAuthoringApply)
	s.router.HandleFunc("POST /v1/sessions/{id}/authoring/hint", s.handleAuthoringHint)
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

// ReadinessCheck represents a single readiness check result
type ReadinessCheck struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]ReadinessCheck)
	allReady := true

	// Check LLM provider availability
	providers := s.llmRegistry.List()
	if len(providers) > 0 {
		checks["llm_provider"] = ReadinessCheck{
			Status:  "ready",
			Message: fmt.Sprintf("providers registered: %v", providers),
		}
	} else {
		allReady = false
		checks["llm_provider"] = ReadinessCheck{
			Status:  "not_ready",
			Message: "no LLM providers registered",
		}
	}

	// Check runner/executor availability
	if s.runnerExecutor != nil {
		// Try to check if the executor is functional
		// For docker executor, we just check it's not nil (actual health is complex)
		checks["runner"] = ReadinessCheck{
			Status:  "ready",
			Message: fmt.Sprintf("executor type: %s", s.cfg.Runner.Executor),
		}
	} else {
		allReady = false
		checks["runner"] = ReadinessCheck{
			Status:  "not_ready",
			Message: "no runner executor configured",
		}
	}

	// Build response
	status := "ready"
	statusCode := http.StatusOK
	if !allReady {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	s.jsonResponse(w, statusCode, map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    checks,
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
		"daemon":            s.cfg.Daemon,
		"learning_contract": s.cfg.Learning,
		"runner":            s.cfg.Runner,
		"default_provider":  s.cfg.LLM.DefaultProvider,
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
		exercises := make([]map[string]interface{}, 0, len(pack.ExerciseIDs))
		if packExercises, err := s.exerciseLoader.LoadPackExercises(pack.ID); err == nil {
			for _, ex := range packExercises {
				exercises = append(exercises, map[string]interface{}{
					"id":         ex.ID,
					"title":      ex.Title,
					"difficulty": ex.Difficulty,
				})
			}
		} else {
			slog.Warn("failed to load pack exercises", "pack", pack.ID, "error", err)
		}

		result = append(result, map[string]interface{}{
			"id":             pack.ID,
			"name":           pack.Name,
			"description":    pack.Description,
			"language":       pack.Language,
			"exercise_count": len(pack.ExerciseIDs),
			"exercises":      exercises,
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
		ExerciseID string            `json:"exercise_id,omitempty"` // For training intent
		SpecPath   string            `json:"spec_path,omitempty"`   // For feature guidance or spec authoring intent
		DocsPaths  []string          `json:"docs_paths,omitempty"`  // For spec authoring intent
		Intent     string            `json:"intent,omitempty"`      // Explicit intent (optional)
		Code       map[string]string `json:"code,omitempty"`        // Initial code (for greenfield/feature)
		Track      string            `json:"track,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// At least one of exercise_id or spec_path should be provided for non-greenfield
	if req.ExerciseID == "" && req.SpecPath == "" && req.Intent != "greenfield" {
		s.jsonError(w, http.StatusBadRequest, "exercise_id or spec_path is required", nil)
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

	// Map intent string to SessionIntent
	var intent session.SessionIntent
	switch req.Intent {
	case "training":
		intent = session.IntentTraining
	case "feature_guidance":
		intent = session.IntentFeatureGuidance
	case "greenfield":
		intent = session.IntentGreenfield
	case "spec_authoring":
		intent = session.IntentSpecAuthoring
	default:
		intent = "" // Let the service infer it
	}

	sess, err := s.sessionService.Create(r.Context(), session.CreateRequest{
		ExerciseID: req.ExerciseID,
		SpecPath:   req.SpecPath,
		DocsPaths:  req.DocsPaths,
		Intent:     intent,
		Code:       req.Code,
		Policy:     policy,
	})
	if err != nil {
		if err == session.ErrExerciseNotFound {
			s.jsonError(w, http.StatusNotFound, "exercise not found", err)
			return
		}
		if err == session.ErrSpecRequired {
			s.jsonError(w, http.StatusBadRequest, "spec_path required for feature_guidance intent", err)
			return
		}
		if err == session.ErrSpecInvalid {
			s.jsonError(w, http.StatusBadRequest, "spec validation failed", err)
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

	// Get session before deleting to generate summary
	sess, err := s.sessionService.Get(r.Context(), id)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	// Get spec progress if this is a feature guidance session
	var specProgress string
	if sess.SpecPath != "" && s.specService != nil {
		if progress, err := s.specService.GetProgress(r.Context(), sess.SpecPath); err == nil {
			specProgress = fmt.Sprintf("%.0f%%", progress.PercentComplete)
		}
	}

	// Generate session summary
	var summary *appreciation.SessionSummary
	if s.appreciationService != nil {
		summary = s.appreciationService.GenerateSessionSummary(sess, specProgress)
	}

	// Delete the session
	if err := s.sessionService.Delete(r.Context(), id); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to delete session", err)
		return
	}

	// Build response with summary
	response := map[string]interface{}{
		"deleted": true,
	}

	if summary != nil {
		response["summary"] = summary
	}

	s.jsonResponse(w, http.StatusOK, response)
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

		// Build response with optional appreciation
		response := map[string]interface{}{
			"run": run,
		}

		// Check for appreciation moments after successful run
		if run.Result != nil && run.Result.TestOK {
			sess, err := s.sessionService.Get(r.Context(), sessionID)
			if err == nil {
				// Convert session.RunResult to domain.RunOutput for appreciation detection
				testsPassed := 0
				if run.Result.TestOK {
					testsPassed = 1 // At least 1 test passed for AllTestsPassed() to return true
				}
				runOutput := &domain.RunOutput{
					TestOK:      run.Result.TestOK,
					BuildOK:     run.Result.BuildOK,
					TestsPassed: testsPassed,
					TestsFailed: 0,
				}

				// Use a fixed user ID for now (single user mode)
				userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
				if msg := s.appreciationService.CheckSession(userID, sess, runOutput); msg != nil {
					response["appreciation"] = msg
				}
			}
		}

		s.jsonResponse(w, http.StatusOK, response)
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

// Pairing handlers

// pairingRequest is the common request format for pairing endpoints
type pairingRequest struct {
	Code          map[string]string `json:"code,omitempty"`          // Optional: override session code
	Context       string            `json:"context,omitempty"`       // Optional: additional context
	RunID         string            `json:"run_id,omitempty"`        // Optional: reference to a run
	Stream        bool              `json:"stream,omitempty"`        // Whether to stream the response
	RequestLevel  int               `json:"request_level,omitempty"` // Explicit level request (4 or 5 for escalation)
	Justification string            `json:"justification,omitempty"` // Required for L4/L5 escalation
}

func (s *Server) handleHint(w http.ResponseWriter, r *http.Request) {
	s.handlePairing(w, r, domain.IntentHint)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	s.handlePairing(w, r, domain.IntentReview)
}

func (s *Server) handleStuck(w http.ResponseWriter, r *http.Request) {
	s.handlePairing(w, r, domain.IntentStuck)
}

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
	s.handlePairing(w, r, domain.IntentNext)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	s.handlePairing(w, r, domain.IntentExplain)
}

func (s *Server) handleEscalate(w http.ResponseWriter, r *http.Request) {
	s.handlePairingWithEscalation(w, r)
}

// handlePairingWithEscalation handles explicit escalation to L4/L5 levels
func (s *Server) handlePairingWithEscalation(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	// Parse request - must have escalation details
	var req struct {
		Code          map[string]string `json:"code,omitempty"`
		Context       string            `json:"context,omitempty"`
		RunID         string            `json:"run_id,omitempty"`
		Stream        bool              `json:"stream,omitempty"`
		Level         int               `json:"level"`         // Required: 4 or 5
		Justification string            `json:"justification"` // Required: why escalation is needed
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate escalation request
	if req.Level != 4 && req.Level != 5 {
		s.jsonError(w, http.StatusBadRequest, "escalation requires level 4 or 5", nil)
		return
	}

	if strings.TrimSpace(req.Justification) == "" {
		s.jsonError(w, http.StatusBadRequest, "justification required for escalation", nil)
		return
	}

	if len(req.Justification) < 20 {
		s.jsonError(w, http.StatusBadRequest, "please provide a more detailed justification (at least 20 characters)", nil)
		return
	}

	// Get session
	sess, err := s.sessionService.Get(r.Context(), sessionID)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	if sess.Status != session.StatusActive {
		s.jsonError(w, http.StatusBadRequest, "session is not active", nil)
		return
	}

	// Check if user has made sufficient attempts before allowing escalation
	if sess.HintCount < 2 {
		s.jsonError(w, http.StatusBadRequest, "please try at least 2 hints before requesting escalation", nil)
		return
	}

	// Check cooldown for high-level interventions
	if !sess.CanRequestIntervention(domain.L4PartialSolution) {
		remaining := sess.CooldownRemaining()
		s.jsonResponse(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":              "cooldown active",
			"cooldown_remaining": remaining.Seconds(),
			"message":            fmt.Sprintf("Please wait %.0f seconds before requesting escalation", remaining.Seconds()),
		})
		return
	}

	// Load exercise for context
	var ex *domain.Exercise
	parts := strings.SplitN(sess.ExerciseID, "/", 2)
	if len(parts) >= 2 {
		ex, _ = s.exerciseLoader.LoadExercise(parts[0], parts[1])
	}

	// Use provided code or session's code
	code := sess.Code
	if len(req.Code) > 0 {
		code = req.Code
	}

	// Create escalation policy that allows higher levels
	escalationPolicy := sess.Policy
	escalationPolicy.MaxLevel = domain.InterventionLevel(req.Level)

	// Build intervention context with justification
	pairingCtx := pairing.InterventionContext{
		Exercise: ex,
		Code:     code,
	}

	// Build intervention request with escalation
	pairingReq := pairing.InterventionRequest{
		SessionID:     uuid.MustParse(sess.ID),
		UserID:        uuid.Nil,
		Intent:        domain.IntentStuck, // Escalation uses stuck intent
		Context:       pairingCtx,
		Policy:        escalationPolicy,
		ExplicitLevel: domain.InterventionLevel(req.Level),
		Justification: req.Justification,
	}

	if req.RunID != "" {
		runUUID, err := uuid.Parse(req.RunID)
		if err != nil {
			s.jsonError(w, http.StatusBadRequest, "invalid run_id", err)
			return
		}
		pairingReq.RunID = &runUUID
	}

	// Log the escalation request
	slog.Info("explicit escalation requested",
		"session_id", sessionID,
		"level", req.Level,
		"justification", req.Justification,
		"hint_count", sess.HintCount,
	)

	// Handle streaming vs non-streaming
	if req.Stream {
		s.handlePairingStream(w, r, pairingReq, sess)
		return
	}

	// Non-streaming: generate intervention
	intervention, err := s.pairingService.Intervene(r.Context(), pairingReq)
	if err != nil {
		slog.Error("escalation intervention failed", "error", err)
		s.jsonError(w, http.StatusInternalServerError, "failed to generate escalation response", err)
		return
	}

	// Record intervention in session
	sessionIntervention := &session.Intervention{
		ID:        intervention.ID.String(),
		SessionID: sess.ID,
		Intent:    intervention.Intent,
		Level:     intervention.Level,
		Type:      intervention.Type,
		Content:   intervention.Content,
		CreatedAt: time.Now(),
	}
	if req.RunID != "" {
		sessionIntervention.RunID = &req.RunID
	}

	if err := s.sessionService.RecordIntervention(r.Context(), sessionIntervention); err != nil {
		slog.Warn("failed to record escalation", "error", err)
	}

	// Extract patches from L4/L5 interventions
	var hasPatch bool
	if intervention.Level >= domain.L4PartialSolution {
		patches := s.patchService.ExtractFromIntervention(intervention, uuid.MustParse(sess.ID), sess.Code)
		hasPatch = len(patches) > 0
		if hasPatch {
			slog.Info("patches extracted from escalation",
				"session_id", sess.ID,
				"patch_count", len(patches),
			)
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":            intervention.ID.String(),
		"intent":        intervention.Intent,
		"level":         intervention.Level,
		"type":          intervention.Type,
		"content":       intervention.Content,
		"escalated":     true,
		"justification": req.Justification,
		"has_patch":     hasPatch,
	})
}

// handlePairing is the common handler for all pairing endpoints
func (s *Server) handlePairing(w http.ResponseWriter, r *http.Request, intent domain.Intent) {
	sessionID := r.PathValue("id")

	// Parse request
	var req pairingRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	// Get session
	sess, err := s.sessionService.Get(r.Context(), sessionID)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	if sess.Status != session.StatusActive {
		s.jsonError(w, http.StatusBadRequest, "session is not active", nil)
		return
	}

	// Check cooldown for L3+ interventions
	if !sess.CanRequestIntervention(domain.L3ConstrainedSnippet) {
		remaining := sess.CooldownRemaining()
		s.jsonResponse(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":              "cooldown active",
			"cooldown_remaining": remaining.Seconds(),
			"message":            fmt.Sprintf("Please wait %.0f seconds before requesting more detailed help", remaining.Seconds()),
		})
		return
	}

	// Load exercise for context
	var ex *domain.Exercise
	parts := strings.SplitN(sess.ExerciseID, "/", 2)
	if len(parts) >= 2 {
		ex, _ = s.exerciseLoader.LoadExercise(parts[0], parts[1])
	}

	// Use provided code or session's code
	code := sess.Code
	if len(req.Code) > 0 {
		code = req.Code
	}

	// Build intervention context
	pairingCtx := pairing.InterventionContext{
		Exercise: ex,
		Code:     code,
	}

	// Build intervention request
	pairingReq := pairing.InterventionRequest{
		SessionID: uuid.MustParse(sess.ID),
		UserID:    uuid.Nil, // Local daemon - no user auth
		Intent:    intent,
		Context:   pairingCtx,
		Policy:    sess.Policy,
	}

	if req.RunID != "" {
		runUUID, err := uuid.Parse(req.RunID)
		if err != nil {
			s.jsonError(w, http.StatusBadRequest, "invalid run_id", err)
			return
		}
		pairingReq.RunID = &runUUID
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		s.handlePairingStream(w, r, pairingReq, sess)
		return
	}

	// Non-streaming: generate intervention
	intervention, err := s.pairingService.Intervene(r.Context(), pairingReq)
	if err != nil {
		slog.Error("intervention failed", "error", err)
		s.jsonError(w, http.StatusInternalServerError, "failed to generate intervention", err)
		return
	}

	// Record intervention in session
	sessionIntervention := &session.Intervention{
		ID:        intervention.ID.String(),
		SessionID: sess.ID,
		Intent:    intervention.Intent,
		Level:     intervention.Level,
		Type:      intervention.Type,
		Content:   intervention.Content,
		CreatedAt: time.Now(),
	}
	if req.RunID != "" {
		sessionIntervention.RunID = &req.RunID
	}

	if err := s.sessionService.RecordIntervention(r.Context(), sessionIntervention); err != nil {
		slog.Warn("failed to record intervention", "error", err)
	}

	// Extract patches from L4/L5 interventions
	var hasPatch bool
	if intervention.Level >= domain.L4PartialSolution {
		patches := s.patchService.ExtractFromIntervention(intervention, uuid.MustParse(sess.ID), sess.Code)
		hasPatch = len(patches) > 0
		if hasPatch {
			slog.Info("patches extracted from intervention",
				"session_id", sess.ID,
				"patch_count", len(patches),
			)
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":        intervention.ID.String(),
		"intent":    intervention.Intent,
		"level":     intervention.Level,
		"type":      intervention.Type,
		"content":   intervention.Content,
		"has_patch": hasPatch,
	})
}

// handlePairingStream handles streaming intervention responses via SSE
func (s *Server) handlePairingStream(w http.ResponseWriter, r *http.Request, req pairing.InterventionRequest, sess *session.Session) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.jsonError(w, http.StatusInternalServerError, "streaming not supported", nil)
		return
	}

	// Start streaming
	stream, err := s.pairingService.IntervenStream(r.Context(), req)
	if err != nil {
		writeSSEEvent(w, "error", err.Error())
		flusher.Flush()
		return
	}

	var contentBuilder strings.Builder
	var level domain.InterventionLevel
	var interventionType domain.InterventionType

	for chunk := range stream {
		switch chunk.Type {
		case "metadata":
			if chunk.Metadata != nil {
				level = chunk.Metadata.Level
				interventionType = chunk.Metadata.Type
				writeSSEEvent(w, "metadata", fmt.Sprintf("{\"level\":%d,\"type\":\"%s\"}", level, interventionType))
			}
		case "content":
			contentBuilder.WriteString(chunk.Content)
			writeSSEEvent(w, "content", chunk.Content)
		case "error":
			writeSSEEvent(w, "error", chunk.Error.Error())
		case "done":
			// Record the complete intervention
			intervention := &session.Intervention{
				ID:        uuid.New().String(),
				SessionID: sess.ID,
				Intent:    req.Intent,
				Level:     level,
				Type:      interventionType,
				Content:   contentBuilder.String(),
				CreatedAt: time.Now(),
			}
			if err := s.sessionService.RecordIntervention(r.Context(), intervention); err != nil {
				slog.Warn("failed to record intervention", "error", err)
			}

			writeSSEEvent(w, "done", fmt.Sprintf("{\"id\":\"%s\"}", intervention.ID))
		}
		flusher.Flush()
	}
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
		"error":  message,
		"status": status,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	s.jsonResponse(w, status, response)
}

func writeSSEEvent(w http.ResponseWriter, event string, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	normalized := strings.ReplaceAll(data, "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}

// Profile & Analytics handlers

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.profileService.GetProfile(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get profile", err)
		return
	}
	s.jsonResponse(w, http.StatusOK, profile)
}

func (s *Server) handleAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := s.profileService.GetOverview(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get analytics overview", err)
		return
	}

	// Build response with optional appreciation for progress
	response := map[string]interface{}{
		"overview": overview,
	}

	// Check for progress appreciation moments
	storedProfile, err := s.profileService.GetProfile(r.Context())
	if err == nil && storedProfile != nil {
		// Convert StoredProfile to domain.LearningProfile for appreciation check
		learningProfile := s.convertToDomainProfile(storedProfile)

		// Use a fixed user ID for now (single user mode)
		userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		if msg := s.appreciationService.CheckProgress(userID, learningProfile, nil); msg != nil {
			response["appreciation"] = msg
		}
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// convertToDomainProfile converts a profile.StoredProfile to domain.LearningProfile
func (s *Server) convertToDomainProfile(stored *profile.StoredProfile) *domain.LearningProfile {
	if stored == nil {
		return nil
	}

	// Convert topic skills
	topicSkills := make(map[string]domain.SkillLevel)
	for topic, skill := range stored.TopicSkills {
		topicSkills[topic] = domain.SkillLevel{
			Level:    skill.Level,
			Attempts: skill.Attempts,
		}
	}

	return &domain.LearningProfile{
		TopicSkills:    topicSkills,
		TotalExercises: stored.TotalExercises,
		TotalRuns:      stored.TotalRuns,
		HintRequests:   stored.HintRequests,
		AvgTimeToGreen: time.Duration(stored.AvgTimeToGreenMs) * time.Millisecond,
		UpdatedAt:      stored.UpdatedAt,
	}
}

func (s *Server) handleAnalyticsSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.profileService.GetSkillBreakdown(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get skill breakdown", err)
		return
	}
	s.jsonResponse(w, http.StatusOK, skills)
}

func (s *Server) handleAnalyticsErrors(w http.ResponseWriter, r *http.Request) {
	errors, err := s.profileService.GetErrorPatterns(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get error patterns", err)
		return
	}
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"patterns": errors,
	})
}

func (s *Server) handleAnalyticsTrend(w http.ResponseWriter, r *http.Request) {
	trend, err := s.profileService.GetHintTrend(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get hint trend", err)
		return
	}
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"trend": trend,
	})
}

// Spec handlers

func (s *Server) handleCreateSpec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Name == "" {
		s.jsonError(w, http.StatusBadRequest, "name is required", nil)
		return
	}

	specObj, err := s.specService.Create(r.Context(), req.Name)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to create spec", err)
		return
	}

	s.jsonResponse(w, http.StatusCreated, specObj)
}

func (s *Server) handleListSpecs(w http.ResponseWriter, r *http.Request) {
	specs, err := s.specService.List(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to list specs", err)
		return
	}

	// Return summary info
	result := make([]map[string]interface{}, 0, len(specs))
	for _, sp := range specs {
		progress := sp.GetProgress()
		result = append(result, map[string]interface{}{
			"name":      sp.Name,
			"version":   sp.Version,
			"file_path": sp.FilePath,
			"progress": map[string]interface{}{
				"satisfied": progress.SatisfiedCriteria,
				"total":     progress.TotalCriteria,
				"percent":   progress.PercentComplete,
			},
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"specs": result,
	})
}

func (s *Server) handleGetSpec(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.jsonError(w, http.StatusBadRequest, "spec path is required", nil)
		return
	}

	specObj, err := s.specService.Load(r.Context(), path)
	if err != nil {
		if err == spec.ErrSpecNotFound {
			s.jsonError(w, http.StatusNotFound, "spec not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to load spec", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, specObj)
}

func (s *Server) handleValidateSpec(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.jsonError(w, http.StatusBadRequest, "spec path is required", nil)
		return
	}

	validation, err := s.specService.Validate(r.Context(), path)
	if err != nil {
		if err == spec.ErrSpecNotFound {
			s.jsonError(w, http.StatusNotFound, "spec not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to validate spec", err)
		return
	}

	status := http.StatusOK
	if !validation.Valid {
		status = http.StatusUnprocessableEntity
	}

	s.jsonResponse(w, status, validation)
}

func (s *Server) handleMarkCriterionSatisfied(w http.ResponseWriter, r *http.Request) {
	criterionID := r.PathValue("id")

	var req struct {
		Path     string `json:"path"`
		Evidence string `json:"evidence"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Path == "" || criterionID == "" {
		s.jsonError(w, http.StatusBadRequest, "spec path and criterion id are required", nil)
		return
	}

	path := req.Path

	if err := s.specService.MarkCriterionSatisfied(r.Context(), path, criterionID, req.Evidence); err != nil {
		if err == spec.ErrSpecNotFound {
			s.jsonError(w, http.StatusNotFound, "spec not found", nil)
			return
		}
		if err == spec.ErrCriterionNotFound {
			s.jsonError(w, http.StatusNotFound, "criterion not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to mark criterion satisfied", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"criterion_id": criterionID,
		"satisfied":    true,
	})
}

func (s *Server) handleLockSpec(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.jsonError(w, http.StatusBadRequest, "spec path is required", nil)
		return
	}

	lock, err := s.specService.Lock(r.Context(), path)
	if err != nil {
		if err == spec.ErrSpecNotFound {
			s.jsonError(w, http.StatusNotFound, "spec not found", nil)
			return
		}
		if err == spec.ErrSpecInvalid {
			s.jsonError(w, http.StatusUnprocessableEntity, "spec must be valid before locking", err)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to lock spec", err)
		return
	}

	s.jsonResponse(w, http.StatusCreated, lock)
}

func (s *Server) handleGetSpecProgress(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.jsonError(w, http.StatusBadRequest, "spec path is required", nil)
		return
	}

	progress, err := s.specService.GetProgress(r.Context(), path)
	if err != nil {
		if err == spec.ErrSpecNotFound {
			s.jsonError(w, http.StatusNotFound, "spec not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get progress", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, progress)
}

func (s *Server) handleGetSpecDrift(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		s.jsonError(w, http.StatusBadRequest, "spec path is required", nil)
		return
	}

	drift, err := s.specService.GetDrift(r.Context(), path)
	if err != nil {
		if err == spec.ErrSpecNotFound {
			s.jsonError(w, http.StatusNotFound, "spec not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get drift report", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, drift)
}

// Patch handlers

func (s *Server) handlePatchPreview(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	// Parse session ID
	sessUUID, err := uuid.Parse(sessionID)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid session ID", nil)
		return
	}

	// Get pending patch preview
	preview, err := s.patchService.PreviewPending(sessUUID)
	if err != nil {
		if err == patch.ErrPatchNotFound {
			s.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"has_patch": false,
				"message":   "No pending patches for this session",
			})
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get patch preview", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"has_patch": true,
		"preview":   preview,
	})
}

func (s *Server) handlePatchApply(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	sessUUID, err := uuid.Parse(sessionID)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid session ID", nil)
		return
	}

	// Get the session to access current code
	sess, err := s.sessionService.Get(r.Context(), sessionID)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	// Apply the pending patch
	file, content, err := s.patchService.ApplyPending(sessUUID)
	if err != nil {
		switch err {
		case patch.ErrPatchNotFound:
			s.jsonError(w, http.StatusNotFound, "no pending patch to apply", nil)
		case patch.ErrPatchApplied:
			s.jsonError(w, http.StatusConflict, "patch already applied", nil)
		case patch.ErrPatchRejected:
			s.jsonError(w, http.StatusConflict, "patch was rejected", nil)
		case patch.ErrPatchExpired:
			s.jsonError(w, http.StatusGone, "patch has expired", nil)
		default:
			s.jsonError(w, http.StatusInternalServerError, "failed to apply patch", err)
		}
		return
	}

	// Update session code
	newCode := make(map[string]string)
	for k, v := range sess.Code {
		newCode[k] = v
	}
	newCode[file] = content

	// Update session with new code
	if _, err := s.sessionService.UpdateCode(r.Context(), sessionID, newCode); err != nil {
		slog.Warn("failed to update session code after patch apply", "error", err)
	}

	slog.Info("patch applied",
		"session_id", sessionID,
		"file", file,
	)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"applied": true,
		"file":    file,
		"content": content,
	})
}

func (s *Server) handlePatchReject(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	sessUUID, err := uuid.Parse(sessionID)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid session ID", nil)
		return
	}

	if err := s.patchService.RejectPending(sessUUID); err != nil {
		if err == patch.ErrPatchNotFound {
			s.jsonError(w, http.StatusNotFound, "no pending patch to reject", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to reject patch", err)
		return
	}

	slog.Info("patch rejected", "session_id", sessionID)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"rejected": true,
	})
}

func (s *Server) handleListPatches(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	sessUUID, err := uuid.Parse(sessionID)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid session ID", nil)
		return
	}

	patches := s.patchService.GetSessionPatches(sessUUID)
	if patches == nil {
		patches = []*domain.Patch{}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"patches": patches,
		"count":   len(patches),
	})
}

func (s *Server) handlePatchLog(w http.ResponseWriter, r *http.Request) {
	logger := s.patchService.GetLogger()
	if logger == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "patch logging not available", nil)
		return
	}

	// Parse optional query params
	limitStr := r.URL.Query().Get("limit")
	sessionID := r.URL.Query().Get("session_id")

	var entries []patch.LogEntry
	if sessionID != "" {
		entries = logger.GetSessionEntries(sessionID)
	} else if limitStr != "" {
		limit := 50 // default
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
		entries = logger.GetRecentEntries(limit)
	} else {
		entries = logger.GetEntries()
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

func (s *Server) handlePatchStats(w http.ResponseWriter, r *http.Request) {
	logger := s.patchService.GetLogger()
	if logger == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "patch logging not available", nil)
		return
	}

	stats := logger.GetStats()
	s.jsonResponse(w, http.StatusOK, stats)
}

// Spec Authoring handlers

func (s *Server) handleAuthoringDiscover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SpecPath  string   `json:"spec_path"`
		DocsPaths []string `json:"docs_paths"`
		Recursive bool     `json:"recursive"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Use default paths if none provided
	if len(req.DocsPaths) == 0 {
		req.DocsPaths = docindex.DefaultDocPaths
	}

	// Get workspace root from spec service
	workspaceRoot := s.specService.GetWorkspaceRoot()

	// Discover documents
	discoverer := docindex.NewDiscoverer(workspaceRoot)
	docs, err := discoverer.Discover(docindex.DiscoverOptions{
		Paths:     req.DocsPaths,
		Recursive: req.Recursive,
		MaxDepth:  3,
	})
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to discover documents", err)
		return
	}

	// Build response with document summaries
	docSummaries := make([]map[string]interface{}, 0, len(docs))
	totalSections := 0
	for _, doc := range docs {
		totalSections += len(doc.Sections)
		docSummaries = append(docSummaries, map[string]interface{}{
			"path":     doc.Path,
			"title":    doc.Title,
			"type":     doc.Type,
			"sections": len(doc.Sections),
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"documents":      docSummaries,
		"total_sections": totalSections,
	})
}

func (s *Server) handleAuthoringSuggest(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req struct {
		Section string `json:"section"` // goals, features, acceptance_criteria, non_functional
		Context string `json:"context,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Section == "" {
		s.jsonError(w, http.StatusBadRequest, "section is required", nil)
		return
	}

	// Get session
	sess, err := s.sessionService.Get(r.Context(), sessionID)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	// Verify it's an authoring session
	if sess.Intent != session.IntentSpecAuthoring {
		s.jsonError(w, http.StatusBadRequest, "session is not a spec authoring session", nil)
		return
	}

	// Load spec
	spec, err := s.specService.Load(r.Context(), sess.SpecPath)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to load spec", err)
		return
	}

	// Discover and load documents
	workspaceRoot := s.specService.GetWorkspaceRoot()
	discoverer := docindex.NewDiscoverer(workspaceRoot)
	docs, err := discoverer.Discover(docindex.DiscoverOptions{
		Paths:     sess.AuthoringDocs,
		Recursive: true,
		MaxDepth:  3,
	})
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to discover documents", err)
		return
	}

	if len(docs) == 0 {
		s.jsonError(w, http.StatusBadRequest, "no documents found for authoring", nil)
		return
	}

	// Update session's current section
	sess.SetAuthoringSection(req.Section)
	// Note: We don't save this to store since it's just for context

	// Build authoring context for LLM
	ctx := pairing.AuthoringContext{
		Spec:      spec,
		Section:   req.Section,
		Documents: docs,
	}

	// Get suggestions from pairing service
	suggestions, err := s.pairingService.SuggestForSection(r.Context(), ctx)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to generate suggestions", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"section":     req.Section,
		"suggestions": suggestions,
	})
}

func (s *Server) handleAuthoringApply(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req struct {
		Section      string `json:"section"`
		SuggestionID string `json:"suggestion_id"`
		Value        any    `json:"value,omitempty"` // Direct value to apply (alternative to suggestion_id)
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Section == "" {
		s.jsonError(w, http.StatusBadRequest, "section is required", nil)
		return
	}

	// Get session
	sess, err := s.sessionService.Get(r.Context(), sessionID)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	// Verify it's an authoring session
	if sess.Intent != session.IntentSpecAuthoring {
		s.jsonError(w, http.StatusBadRequest, "session is not a spec authoring session", nil)
		return
	}

	// Apply the suggestion to the spec
	// For now, we return the value that should be applied - the editor will update the file
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"applied":       true,
		"section":       req.Section,
		"suggestion_id": req.SuggestionID,
		"value":         req.Value,
	})
}

func (s *Server) handleAuthoringHint(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var req struct {
		Section  string `json:"section"`
		Question string `json:"question"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Get session
	sess, err := s.sessionService.Get(r.Context(), sessionID)
	if err != nil {
		if err == session.ErrSessionNotFound {
			s.jsonError(w, http.StatusNotFound, "session not found", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get session", err)
		return
	}

	// Verify it's an authoring session
	if sess.Intent != session.IntentSpecAuthoring {
		s.jsonError(w, http.StatusBadRequest, "session is not a spec authoring session", nil)
		return
	}

	// Load spec
	spec, err := s.specService.Load(r.Context(), sess.SpecPath)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to load spec", err)
		return
	}

	// Discover and load documents
	workspaceRoot := s.specService.GetWorkspaceRoot()
	discoverer := docindex.NewDiscoverer(workspaceRoot)
	docs, err := discoverer.Discover(docindex.DiscoverOptions{
		Paths:     sess.AuthoringDocs,
		Recursive: true,
		MaxDepth:  3,
	})
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to discover documents", err)
		return
	}

	// Build authoring context
	ctx := pairing.AuthoringContext{
		Spec:      spec,
		Section:   req.Section,
		Documents: docs,
		Question:  req.Question,
	}

	// Get hint from pairing service
	hint, err := s.pairingService.AuthoringHint(r.Context(), ctx)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to generate hint", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, hint)
}
