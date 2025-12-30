package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
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
	pairingService *pairing.Service
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

	// Initialize pairing service
	s.pairingService = pairing.NewService(s.llmRegistry, cfg.Config.LLM.DefaultProvider)

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

// Pairing handlers

// pairingRequest is the common request format for pairing endpoints
type pairingRequest struct {
	Code     map[string]string `json:"code,omitempty"`       // Optional: override session code
	Context  string            `json:"context,omitempty"`    // Optional: additional context
	RunID    string            `json:"run_id,omitempty"`     // Optional: reference to a run
	Stream   bool              `json:"stream,omitempty"`     // Whether to stream the response
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
			"error":             "cooldown active",
			"cooldown_remaining": remaining.Seconds(),
			"message":           fmt.Sprintf("Please wait %.0f seconds before requesting more detailed help", remaining.Seconds()),
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
		runUUID := uuid.MustParse(req.RunID)
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

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":      intervention.ID.String(),
		"intent":  intervention.Intent,
		"level":   intervention.Level,
		"type":    intervention.Type,
		"content": intervention.Content,
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
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
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
				fmt.Fprintf(w, "event: metadata\ndata: {\"level\":%d,\"type\":\"%s\"}\n\n", level, interventionType)
			}
		case "content":
			contentBuilder.WriteString(chunk.Content)
			fmt.Fprintf(w, "event: content\ndata: %s\n\n", chunk.Content)
		case "error":
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", chunk.Error.Error())
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

			fmt.Fprintf(w, "event: done\ndata: {\"id\":\"%s\"}\n\n", intervention.ID)
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
		"error":   message,
		"status":  status,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	s.jsonResponse(w, status, response)
}
