package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/felixgeelhaar/temper/internal/metrics"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/patch"
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/sandbox"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/felixgeelhaar/temper/internal/spec"
	sqlitestore "github.com/felixgeelhaar/temper/internal/storage/sqlite"
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

	// Track store for learning contract presets
	trackStore *sqlitestore.TrackStore

	// Sandbox manager for persistent containers (using interface for testability)
	SandboxManager SandboxManager

	// Document index service for external context
	docindexService *docindex.Service

	// Idempotency cache for non-idempotent POSTs (run, sandbox-exec).
	idempotency *IdempotencyCache

	// In-process metrics registry. Exposed at /v1/metrics in Prometheus
	// text format. Pairing.ClampViolations() is exported separately and
	// merged into the response.
	metrics *metrics.Registry
}

// SandboxManager defines the interface for sandbox operations
type SandboxManager interface {
	Create(ctx context.Context, sessionID string, cfg sandbox.Config) (*sandbox.Sandbox, error)
	Get(ctx context.Context, id string) (*sandbox.Sandbox, error)
	GetBySession(ctx context.Context, sessionID string) (*sandbox.Sandbox, error)
	AttachCode(ctx context.Context, sandboxID string, code map[string]string) error
	Execute(ctx context.Context, sandboxID string, cmd []string, timeout time.Duration) (*sandbox.ExecResult, error)
	Pause(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	Destroy(ctx context.Context, id string) error
	Cleanup(ctx context.Context) (int, error)
	StartCleanupLoop(ctx context.Context, interval time.Duration)
	Close(ctx context.Context) error
}

// GetSandboxManager returns the sandbox manager (for testing)
func (s *Server) GetSandboxManager() SandboxManager {
	return s.SandboxManager
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
		cfg:         cfg.Config,
		router:      http.NewServeMux(),
		idempotency: NewIdempotencyCache(),
		metrics:     metrics.New(),
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

	// Initialize runner. Docker is mandatory: per-language sandboxing is
	// only safe with the container boundary, and the multi-language
	// dispatch table lives entirely in the docker executors. The previous
	// LocalExecutor fallback was Go-only and silently produced incorrect
	// results for Python/TS/Java/Rust/C exercises.
	dockerCfg := runner.DockerConfig{
		BaseImage:  cfg.Config.Runner.Docker.Image,
		MemoryMB:   int64(cfg.Config.Runner.Docker.MemoryMB),
		CPULimit:   cfg.Config.Runner.Docker.CPULimit,
		NetworkOff: cfg.Config.Runner.Docker.NetworkOff,
		Timeout:    time.Duration(cfg.Config.Runner.Docker.TimeoutSeconds) * time.Second,
	}
	executor, err := runner.NewDockerExecutor(dockerCfg)
	if err != nil {
		return nil, fmt.Errorf("docker executor unavailable (Docker is required; install Docker Desktop or run `colima start`): %w", err)
	}
	s.runnerExecutor = executor

	// Get temper directory for data storage
	temperDir, err := config.TemperDir()
	if err != nil {
		return nil, fmt.Errorf("get temper dir: %w", err)
	}

	// Initialize storage backend based on config
	var sessionStore session.SessionStore
	var profileStore profile.ProfileStore

	switch cfg.Config.Storage.Driver {
	case "json":
		// Legacy JSON file storage
		sessionsPath := cfg.SessionsPath
		if sessionsPath == "" {
			sessionsPath = filepath.Join(temperDir, "sessions")
		}
		jsonSessionStore, err := session.NewStore(sessionsPath)
		if err != nil {
			return nil, fmt.Errorf("create json session store: %w", err)
		}
		sessionStore = jsonSessionStore

		jsonProfileStore, err := profile.NewStore(filepath.Join(temperDir, "profiles"))
		if err != nil {
			return nil, fmt.Errorf("create json profile store: %w", err)
		}
		profileStore = jsonProfileStore

	default: // "sqlite" (default)
		dbPath := cfg.Config.Storage.Path
		if dbPath == "" {
			dbPath = filepath.Join(temperDir, "temper.db")
		}
		db, err := sqlitestore.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}
		if err := db.Migrate(); err != nil {
			return nil, fmt.Errorf("run migrations: %w", err)
		}
		slog.Info("sqlite storage initialized", "path", dbPath)

		sessionStore = sqlitestore.NewSessionStore(db)
		profileStore = sqlitestore.NewProfileStore(db)
		s.trackStore = sqlitestore.NewTrackStore(db)

		// Initialize sandbox manager (optional — requires Docker)
		sandboxBackend, err := sandbox.NewDockerBackend()
		if err != nil {
			slog.Warn("sandbox support disabled: Docker not available", "error", err)
		} else {
			sandboxStore := sqlitestore.NewSandboxStore(db)
			s.SandboxManager = sandbox.NewManager(sandboxStore, sandboxBackend)
			s.SandboxManager.StartCleanupLoop(ctx, 5*time.Minute)
			slog.Info("sandbox support enabled")
		}

		// Initialize document index service (keyword embedder as default)
		s.docindexService = docindex.NewService(db.DB, nil)
		slog.Info("document index service initialized")
	}

	sessionSvc := session.NewService(sessionStore, s.exerciseLoader, s.runnerExecutor)
	s.sessionService = sessionSvc
	s.sessionServiceConcrete = sessionSvc

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
	pairingSvc := pairing.NewService(s.llmRegistryConcrete, cfg.Config.LLM.DefaultProvider)
	if levelMap := buildLevelModelMap(cfg.Config.LLM.LevelModels); len(levelMap) > 0 {
		pairingSvc.SetLevelModels(levelMap)
	}
	s.pairingService = pairingSvc

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

	// Build middleware chain.
	// Order (outermost first): host guard -> CORS -> correlation ID ->
	// recovery -> logging -> auth -> router.
	// Host guard runs first to reject DNS-rebinding attempts before any
	// processing. CORS is outside auth so the OPTIONS preflight does not
	// require a token. Auth gates router and is the trust boundary for
	// authenticated endpoints.
	addr := fmt.Sprintf("%s:%d", cfg.Config.Daemon.Bind, cfg.Config.Daemon.Port)

	if cfg.Config.Daemon.Bind != "127.0.0.1" && cfg.Config.Daemon.AuthToken == "" {
		return nil, fmt.Errorf("non-loopback bind %q requires daemon.auth_token in secrets.yaml", cfg.Config.Daemon.Bind)
	}

	allowedHosts := []string{"127.0.0.1"}
	allowedOrigins := []string{
		"http://127.0.0.1:4321",
		"http://127.0.0.1:7432",
		fmt.Sprintf("http://127.0.0.1:%d", cfg.Config.Daemon.Port),
	}

	var handler http.Handler = s.router
	if cfg.Config.Daemon.AuthToken != "" {
		handler = authMiddleware(cfg.Config.Daemon.AuthToken)(handler)
	} else {
		slog.Warn("daemon.auth_token is empty: API is unauthenticated. Run `temper init` to generate a token.")
	}
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)
	handler = correlationIDMiddleware(handler)
	handler = corsMiddleware(allowedOrigins)(handler)
	handler = hostGuardMiddleware(allowedHosts)(handler)

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
	s.router.HandleFunc("GET /v1/metrics", s.handleMetrics)

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

	// Sandboxes
	s.router.HandleFunc("POST /v1/sessions/{id}/sandbox", s.handleCreateSandbox)
	s.router.HandleFunc("GET /v1/sessions/{id}/sandbox", s.handleGetSandbox)
	s.router.HandleFunc("DELETE /v1/sessions/{id}/sandbox", s.handleDestroySandbox)
	s.router.HandleFunc("POST /v1/sessions/{id}/sandbox/exec", s.handleSandboxExec)
	s.router.HandleFunc("POST /v1/sessions/{id}/sandbox/pause", s.handlePauseSandbox)
	s.router.HandleFunc("POST /v1/sessions/{id}/sandbox/resume", s.handleResumeSandbox)

	// AI Spec Generation
	s.router.HandleFunc("POST /v1/specs/generate", s.handleGenerateSpec)

	// Document Index (External Context Providers)
	s.router.HandleFunc("POST /v1/docindex/index", s.handleDocIndexIndex)
	s.router.HandleFunc("GET /v1/docindex/status", s.handleDocIndexStatus)
	s.router.HandleFunc("POST /v1/docindex/search", s.handleDocIndexSearch)
	s.router.HandleFunc("GET /v1/docindex/documents", s.handleDocIndexListDocuments)
	s.router.HandleFunc("POST /v1/docindex/reindex", s.handleDocIndexReindex)

	// Tracks (Learning Contract Presets)
	s.router.HandleFunc("GET /v1/tracks", s.handleListTracks)
	s.router.HandleFunc("GET /v1/tracks/{id}", s.handleGetTrack)
	s.router.HandleFunc("POST /v1/tracks", s.handleCreateTrack)
	s.router.HandleFunc("PUT /v1/tracks/{id}", s.handleUpdateTrack)
	s.router.HandleFunc("DELETE /v1/tracks/{id}", s.handleDeleteTrack)
	s.router.HandleFunc("POST /v1/tracks/export", s.handleExportTrack)
	s.router.HandleFunc("POST /v1/tracks/import", s.handleImportTrack)
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

	// Close sandbox manager (destroys active sandboxes)
	if s.SandboxManager != nil {
		if err := s.SandboxManager.Close(ctx); err != nil {
			slog.Warn("failed to close sandbox manager", "error", err)
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
		s.jsonErrorCode(w, http.StatusNotFound, ErrCodeExerciseNotFound, "exercise not found", err)
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

	// Get learning policy from track store (SQLite) or config fallback
	var policy *domain.LearningPolicy
	if req.Track != "" {
		// Try SQLite track store first
		if s.trackStore != nil {
			if track, err := s.trackStore.Get(req.Track); err == nil {
				p := track.ToPolicy()
				policy = &p
			}
		}
		// Fallback to config tracks
		if policy == nil {
			if track, ok := s.cfg.Learning.Tracks[req.Track]; ok {
				policy = &domain.LearningPolicy{
					MaxLevel:        domain.InterventionLevel(track.MaxLevel),
					CooldownSeconds: track.CooldownSeconds,
					Track:           req.Track,
				}
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeExerciseNotFound, "exercise not found", err)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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

	// Read the body once so we can both deduplicate via Idempotency-Key
	// and decode it. MaxBytesReader still bounds total bytes consumed.
	r.Body = http.MaxBytesReader(w, r.Body, MaxRunBodyBytes)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		s.jsonErrorCode(w, http.StatusRequestEntityTooLarge, ErrCodePayloadTooLarge, "request body too large", err)
		return
	}

	idemKey := r.Header.Get(IdempotencyHeader)
	if idemKey != "" && s.idempotency != nil {
		if entry, ok, conflict := s.idempotency.Lookup(sessionID, idemKey, bodyBytes); conflict {
			s.jsonErrorCode(w, http.StatusConflict, ErrCodeConflict,
				"Idempotency-Key reused with a different request body", nil)
			return
		} else if ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotent-Replay", "true")
			w.WriteHeader(entry.statusCode)
			w.Write(entry.payload)
			return
		}
	}

	var req struct {
		Code   map[string]string `json:"code"`
		Format bool              `json:"format"`
		Build  bool              `json:"build"`
		Test   bool              `json:"test"`
	}

	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := validateCodePayload(req.Code); err != nil {
		if pe := asPayloadError(err); pe != nil {
			s.jsonError(w, http.StatusRequestEntityTooLarge, pe.Message, err)
			return
		}
		s.jsonError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	// Wrap the response writer so successful responses can be cached for
	// future replay under the same idempotency key.
	if idemKey != "" && s.idempotency != nil {
		captured := newCapturingResponseWriter(w)
		defer func() {
			if captured.statusCode >= 200 && captured.statusCode < 300 {
				s.idempotency.Store(sessionID, idemKey, bodyBytes, captured.statusCode, captured.body.Bytes())
			}
		}()
		w = captured
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
				s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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

	if s.metrics != nil {
		s.metrics.Counter("hint_requests_total",
			"Pairing intervention requests by intent.").
			Inc(map[string]string{"intent": string(intent)})
	}

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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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
	s.jsonErrorCode(w, status, defaultErrorCodeForStatus(status), message, err)
}

// jsonErrorCode writes a structured error response with an explicit
// machine-readable error_code. Editor clients should switch on error_code,
// never on the message text.
func (s *Server) jsonErrorCode(w http.ResponseWriter, status int, code, message string, err error) {
	if code == "" {
		code = defaultErrorCodeForStatus(status)
	}
	response := map[string]interface{}{
		"error":      message,
		"error_code": code,
		"status":     status,
	}
	if err != nil {
		// If the underlying error already carries a payload-specific code,
		// surface it as the authoritative code; payload validators set
		// these.
		if pe := asPayloadError(err); pe != nil {
			response["error_code"] = pe.Code
		}
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSpecNotFound, "spec not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSpecNotFound, "spec not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSpecNotFound, "spec not found", nil)
			return
		}
		if err == spec.ErrCriterionNotFound {
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeCriterionNotFound, "criterion not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSpecNotFound, "spec not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSpecNotFound, "spec not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSpecNotFound, "spec not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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
			s.jsonErrorCode(w, http.StatusNotFound, ErrCodeSessionNotFound, "session not found", nil)
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


// AI Spec Generation handler

func (s *Server) handleGenerateSpec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Goals       []string `json:"goals,omitempty"`
		Context     string   `json:"context,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Name == "" {
		s.jsonError(w, http.StatusBadRequest, "name is required", nil)
		return
	}
	if req.Description == "" {
		s.jsonError(w, http.StatusBadRequest, "description is required", nil)
		return
	}

	// Get LLM provider
	provider, err := s.llmRegistry.Default()
	if err != nil {
		s.jsonError(w, http.StatusServiceUnavailable, "no LLM provider available", err)
		return
	}

	// Build prompt for spec generation
	goalsSection := ""
	if len(req.Goals) > 0 {
		goalsSection = "\n\nUser-provided goals:\n"
		for _, g := range req.Goals {
			goalsSection += "- " + g + "\n"
		}
	}
	contextSection := ""
	if req.Context != "" {
		contextSection = "\n\nAdditional context:\n" + req.Context
	}

	prompt := fmt.Sprintf(`Generate a complete product specification in JSON format for the following project:

Name: %s
Description: %s%s%s

Generate a JSON object with these exact fields:
{
  "name": "the project name",
  "version": "0.1.0",
  "goals": ["list of 3-5 clear project goals"],
  "features": [
    {
      "id": "feature-slug",
      "title": "Feature Title",
      "description": "Detailed description",
      "priority": "high|medium|low",
      "success_criteria": ["measurable criteria"]
    }
  ],
  "non_functional": {
    "performance": ["performance requirements"],
    "security": ["security requirements"],
    "scalability": ["scalability requirements"]
  },
  "acceptance_criteria": [
    {
      "id": "ac-NNN",
      "description": "Given/When/Then format acceptance criterion",
      "satisfied": false
    }
  ],
  "milestones": [
    {
      "id": "ms-NNN",
      "name": "Milestone Name",
      "features": ["feature-ids"],
      "target": "relative timeline",
      "description": "What this milestone delivers"
    }
  ]
}

Requirements:
- Generate 3-8 features ordered by priority
- Each feature should have 2-4 success criteria
- Generate 5-15 acceptance criteria in Given/When/Then format
- Generate 2-4 milestones
- All IDs should be kebab-case slugs
- Be specific and actionable, not generic
- Output ONLY valid JSON, no markdown fences or explanation`, req.Name, req.Description, goalsSection, contextSection)

	llmReq := &llm.Request{
		System:      "You are a product specification generator. Output only valid JSON.",
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	resp, err := provider.Generate(r.Context(), llmReq)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to generate spec", err)
		return
	}

	// Parse the LLM response as a ProductSpec
	var generatedSpec domain.ProductSpec
	if err := json.Unmarshal([]byte(resp.Content), &generatedSpec); err != nil {
		// If direct parse fails, try to extract JSON from the response
		jsonStart := strings.Index(resp.Content, "{")
		jsonEnd := strings.LastIndex(resp.Content, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			extracted := resp.Content[jsonStart : jsonEnd+1]
			if err2 := json.Unmarshal([]byte(extracted), &generatedSpec); err2 != nil {
				s.jsonError(w, http.StatusInternalServerError, "failed to parse generated spec", err2)
				return
			}
		} else {
			s.jsonError(w, http.StatusInternalServerError, "failed to parse generated spec", err)
			return
		}
	}

	// Ensure all acceptance criteria IDs are set
	for i := range generatedSpec.AcceptanceCriteria {
		if generatedSpec.AcceptanceCriteria[i].ID == "" {
			generatedSpec.AcceptanceCriteria[i].ID = fmt.Sprintf("ac-%03d", i+1)
		}
	}

	// Save the spec
	if err := s.specService.Save(r.Context(), &generatedSpec); err != nil {
		// If save fails (e.g. no .specs dir), still return the generated spec
		slog.Warn("failed to save generated spec", "error", err)
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"spec":    generatedSpec,
			"saved":   false,
			"message": "Spec generated but not saved: " + err.Error(),
		})
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"spec":  generatedSpec,
		"saved": true,
	})
}


// --- Document Index (External Context) Handlers ---

func (s *Server) handleDocIndexIndex(w http.ResponseWriter, r *http.Request) {
	if s.docindexService == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "document indexing requires sqlite storage", nil)
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	basePath := req.Path
	if basePath == "" {
		basePath = s.specService.GetWorkspaceRoot()
	}

	result, err := s.docindexService.IndexDirectory(r.Context(), basePath)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "indexing failed", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleDocIndexStatus(w http.ResponseWriter, r *http.Request) {
	if s.docindexService == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "document indexing requires sqlite storage", nil)
		return
	}

	stats, err := s.docindexService.Stats()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to get index stats", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, stats)
}

func (s *Server) handleDocIndexSearch(w http.ResponseWriter, r *http.Request) {
	if s.docindexService == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "document indexing requires sqlite storage", nil)
		return
	}

	var req struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Query == "" {
		s.jsonError(w, http.StatusBadRequest, "query is required", nil)
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}

	results, err := s.docindexService.Search(r.Context(), req.Query, topK)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "search failed", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"query":   req.Query,
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleDocIndexListDocuments(w http.ResponseWriter, r *http.Request) {
	if s.docindexService == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "document indexing requires sqlite storage", nil)
		return
	}

	docs, err := s.docindexService.ListDocuments()
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to list documents", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"documents": docs,
		"count":     len(docs),
	})
}

func (s *Server) handleDocIndexReindex(w http.ResponseWriter, r *http.Request) {
	if s.docindexService == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "document indexing requires sqlite storage", nil)
		return
	}

	result, err := s.docindexService.ReindexAll(r.Context())
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "reindexing failed", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

// buildLevelModelMap converts the YAML-friendly map[string]string from
// config into the typed map the pairing service expects. Keys "0".."5"
// are recognized; unknown keys are ignored with a slog warn.
func buildLevelModelMap(in map[string]string) map[domain.InterventionLevel]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[domain.InterventionLevel]string, 6)
	for k, v := range in {
		level, ok := parseLevelKey(k)
		if !ok {
			slog.Warn("ignoring unknown level_models key", "key", k)
			continue
		}
		out[level] = v
	}
	return out
}

func parseLevelKey(k string) (domain.InterventionLevel, bool) {
	switch k {
	case "0", "L0":
		return domain.L0Clarify, true
	case "1", "L1":
		return domain.L1CategoryHint, true
	case "2", "L2":
		return domain.L2LocationConcept, true
	case "3", "L3":
		return domain.L3ConstrainedSnippet, true
	case "4", "L4":
		return domain.L4PartialSolution, true
	case "5", "L5":
		return domain.L5FullSolution, true
	default:
		return 0, false
	}
}
