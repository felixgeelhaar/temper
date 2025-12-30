package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/felixgeelhaar/temper/internal/api/handlers"
	"github.com/felixgeelhaar/temper/internal/api/middleware"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// Router wraps the HTTP multiplexer with middleware and handlers
type Router struct {
	mux      *http.ServeMux
	app      *App
	queries  *storage.Queries
	auth     *handlers.AuthHandler
	exercise *handlers.ExerciseHandler
	ws       *handlers.WorkspaceHandler
	run      *handlers.RunHandler
	pairing  *handlers.PairingHandler
}

// NewRouter creates a new API router with all routes configured
func NewRouter(app *App) (http.Handler, error) {
	r := &Router{
		mux:     http.NewServeMux(),
		app:     app,
		queries: storage.New(app.DB),
	}

	// Initialize handlers
	r.auth = handlers.NewAuthHandler(app.Auth, !app.Config.Debug, 7*24*3600)
	r.exercise = handlers.NewExerciseHandler(app.Exercises, app.Workspace)
	r.ws = handlers.NewWorkspaceHandler(app.Workspace)
	r.run = handlers.NewRunHandler(app.Runner, app.Workspace, app.Exercises, app.DB)
	r.pairing = handlers.NewPairingHandler(app.Pairing, app.Workspace, app.Exercises, app.DB)

	// Register routes
	r.registerRoutes()

	// Build middleware chain
	handler := r.buildMiddlewareChain(r.mux, app)

	return handler, nil
}

func (r *Router) registerRoutes() {
	// Health check
	r.mux.HandleFunc("GET /health", r.handleHealth)
	r.mux.HandleFunc("GET /ready", r.handleReady)

	// API v1 routes - Auth (no auth required)
	r.mux.HandleFunc("POST /api/v1/auth/register", r.auth.Register)
	r.mux.HandleFunc("POST /api/v1/auth/login", r.auth.Login)
	r.mux.HandleFunc("POST /api/v1/auth/logout", r.auth.Logout)
	r.mux.HandleFunc("GET /api/v1/auth/me", r.auth.Me)

	// Exercises (public read, auth required for start)
	r.mux.HandleFunc("GET /api/v1/exercises", r.exercise.ListPacks)
	r.mux.HandleFunc("GET /api/v1/exercises/next", r.requireAuth(r.exercise.GetNextExercise))
	r.mux.HandleFunc("GET /api/v1/exercises/{pack}", r.exercise.ListPackExercises)
	r.mux.HandleFunc("GET /api/v1/exercises/{pack}/{category}/{slug}", r.exercise.GetExercise)
	r.mux.HandleFunc("POST /api/v1/exercises/{pack}/{category}/{slug}/start", r.requireAuth(r.exercise.StartExercise))

	// Workspaces (requires auth)
	r.mux.HandleFunc("GET /api/v1/workspaces", r.requireAuth(r.ws.List))
	r.mux.HandleFunc("POST /api/v1/workspaces", r.requireAuth(r.ws.Create))
	r.mux.HandleFunc("GET /api/v1/workspaces/{id}", r.requireAuth(r.ws.Get))
	r.mux.HandleFunc("PUT /api/v1/workspaces/{id}", r.requireAuth(r.ws.Update))
	r.mux.HandleFunc("DELETE /api/v1/workspaces/{id}", r.requireAuth(r.ws.Delete))
	r.mux.HandleFunc("POST /api/v1/workspaces/{id}/snapshot", r.requireAuth(r.ws.CreateSnapshot))
	r.mux.HandleFunc("GET /api/v1/workspaces/{id}/versions", r.requireAuth(r.ws.ListVersions))

	// Runs (requires auth)
	r.mux.HandleFunc("POST /api/v1/runs", r.requireAuth(r.run.TriggerRun))
	r.mux.HandleFunc("GET /api/v1/runs/{id}", r.requireAuth(r.run.GetRun))
	r.mux.HandleFunc("GET /api/v1/runs/{id}/stream", r.requireAuth(r.run.StreamRun))
	r.mux.HandleFunc("POST /api/v1/workspaces/{workspace_id}/format", r.requireAuth(r.run.FormatWorkspace))

	// Pairing (requires auth)
	r.mux.HandleFunc("POST /api/v1/pairing/sessions", r.requireAuth(r.pairing.StartSession))
	r.mux.HandleFunc("POST /api/v1/pairing/intervene", r.requireAuth(r.pairing.Intervene))
	r.mux.HandleFunc("GET /api/v1/pairing/stream/{session_id}", r.requireAuth(r.pairing.StreamIntervention))

	// Profile (requires auth)
	r.mux.HandleFunc("GET /api/v1/profile", r.requireAuth(r.handleGetProfile))
	r.mux.HandleFunc("GET /api/v1/profile/recommendations", r.requireAuth(r.handleGetRecommendations))
}

func (r *Router) buildMiddlewareChain(handler http.Handler, app *App) http.Handler {
	// Apply middleware in reverse order (last applied = first executed)
	handler = middleware.Recovery(handler)
	handler = middleware.Logger(handler)

	// Apply rate limiting (skip in debug mode for easier development)
	if !app.Config.Debug {
		handler = middleware.RateLimitMiddleware(middleware.DefaultRateLimitConfig())(handler)
	}

	handler = middleware.RequestID(handler)
	handler = middleware.CORS(handler)

	return handler
}

// requireAuth wraps a handler with authentication
func (r *Router) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("session")
		if err != nil {
			Unauthorized(w, req, "authentication required")
			return
		}

		user, _, err := r.app.Auth.ValidateSession(req.Context(), cookie.Value)
		if err != nil {
			slog.Warn("invalid session",
				"error", err,
				"request_id", middleware.GetRequestID(req.Context()),
			)
			Unauthorized(w, req, "invalid or expired session")
			return
		}

		// Add user to context
		ctx := context.WithValue(req.Context(), handlers.ContextKeyUser, user)
		next(w, req.WithContext(ctx))
	}
}

// Health check handlers
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	r.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *Router) handleReady(w http.ResponseWriter, req *http.Request) {
	// Check database connectivity
	if err := r.app.DB.PingContext(req.Context()); err != nil {
		slog.Error("database health check failed",
			"error", err,
			"request_id", middleware.GetRequestID(req.Context()),
		)
		r.jsonResponse(w, http.StatusServiceUnavailable, map[string]any{
			"status": "not ready",
			"checks": map[string]string{
				"database": "unhealthy",
			},
		})
		return
	}

	r.jsonResponse(w, http.StatusOK, map[string]any{
		"status": "ready",
		"checks": map[string]string{
			"database": "healthy",
		},
	})
}

// Profile handlers
func (r *Router) handleGetProfile(w http.ResponseWriter, req *http.Request) {
	user := req.Context().Value(handlers.ContextKeyUser).(*domain.User)

	// Get profile from database
	profile, err := r.queries.GetLearningProfile(req.Context(), user.ID)
	if err != nil {
		// Profile doesn't exist - return defaults
		r.jsonResponse(w, http.StatusOK, map[string]any{
			"user_id":         user.ID.String(),
			"total_exercises": 0,
			"total_runs":      0,
			"hint_requests":   0,
			"topic_skills":    map[string]any{},
		})
		return
	}

	// Parse topic skills from JSON
	var topicSkills map[string]any
	if profile.TopicSkills != nil {
		_ = json.Unmarshal(profile.TopicSkills, &topicSkills)
	}
	if topicSkills == nil {
		topicSkills = map[string]any{}
	}

	r.jsonResponse(w, http.StatusOK, map[string]any{
		"user_id":         user.ID.String(),
		"total_exercises": profile.TotalExercises,
		"total_runs":      profile.TotalRuns,
		"hint_requests":   profile.HintRequests,
		"topic_skills":    topicSkills,
	})
}

func (r *Router) handleGetRecommendations(w http.ResponseWriter, req *http.Request) {
	// Return empty recommendations for now
	r.jsonResponse(w, http.StatusOK, map[string]any{
		"recommendations": []any{},
	})
}

// Helper for JSON responses
func (r *Router) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func (r *Router) jsonError(w http.ResponseWriter, status int, message string) {
	r.jsonResponse(w, status, map[string]string{"error": message})
}
