package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/workspace"
	"github.com/google/uuid"
)

// ExerciseHandler handles exercise endpoints
type ExerciseHandler struct {
	registry         *exercise.Registry
	workspaceService *workspace.Service
}

// NewExerciseHandler creates a new exercise handler
func NewExerciseHandler(registry *exercise.Registry, workspaceService *workspace.Service) *ExerciseHandler {
	return &ExerciseHandler{
		registry:         registry,
		workspaceService: workspaceService,
	}
}

// PackResponse represents a pack in API responses
type PackResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// ExerciseSummary represents an exercise summary in API responses
type ExerciseSummary struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Difficulty  string   `json:"difficulty"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// ExerciseDetail represents detailed exercise info
type ExerciseDetail struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Difficulty  string            `json:"difficulty"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	StarterCode map[string]string `json:"starter_code"`
	TestCode    map[string]string `json:"test_code,omitempty"`
}

// ListPacks lists all available exercise packs
func (h *ExerciseHandler) ListPacks(w http.ResponseWriter, r *http.Request) {
	packs := h.registry.ListPacks()

	response := make([]PackResponse, 0, len(packs))
	for _, p := range packs {
		response = append(response, PackResponse{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Version:     p.Version,
		})
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"packs": response,
		"total": len(response),
	})
}

// ListPackExercises lists exercises in a pack
func (h *ExerciseHandler) ListPackExercises(w http.ResponseWriter, r *http.Request) {
	packID := r.PathValue("pack")

	pack, err := h.registry.GetPack(packID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "pack not found")
		return
	}

	exercises, err := h.registry.ListPackExercises(packID)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to list exercises")
		return
	}

	response := make([]ExerciseSummary, 0, len(exercises))
	for _, ex := range exercises {
		response = append(response, ExerciseSummary{
			ID:          ex.ID,
			Title:       ex.Title,
			Difficulty:  string(ex.Difficulty),
			Description: ex.Description,
			Tags:        ex.Tags,
		})
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"pack":      PackResponse{ID: pack.ID, Name: pack.Name, Description: pack.Description, Version: pack.Version},
		"exercises": response,
		"total":     len(response),
	})
}

// GetExercise gets a specific exercise
func (h *ExerciseHandler) GetExercise(w http.ResponseWriter, r *http.Request) {
	packID := r.PathValue("pack")
	category := r.PathValue("category")
	slug := r.PathValue("slug")

	// Exercise ID format: pack/category/slug
	exID := packID + "/" + category + "/" + slug

	ex, err := h.registry.GetExercise(exID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "exercise not found")
		return
	}

	h.jsonResponse(w, http.StatusOK, ExerciseDetail{
		ID:          ex.ID,
		Title:       ex.Title,
		Difficulty:  string(ex.Difficulty),
		Description: ex.Description,
		Tags:        ex.Tags,
		StarterCode: ex.StarterCode,
		TestCode:    ex.TestCode,
	})
}

// StartExercise creates a workspace from an exercise
func (h *ExerciseHandler) StartExercise(w http.ResponseWriter, r *http.Request) {
	packID := r.PathValue("pack")
	category := r.PathValue("category")
	slug := r.PathValue("slug")

	// Get user from context
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Exercise ID format: pack/category/slug
	exID := packID + "/" + category + "/" + slug

	ex, err := h.registry.GetExercise(exID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "exercise not found")
		return
	}

	// Create workspace from exercise
	artifact, err := h.workspaceService.CreateFromExercise(r.Context(), userID, ex)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to create workspace")
		return
	}

	h.jsonResponse(w, http.StatusCreated, map[string]any{
		"workspace": WorkspaceResponse{
			ID:         artifact.ID.String(),
			Name:       artifact.Name,
			ExerciseID: artifact.ExerciseID,
			CreatedAt:  artifact.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  artifact.UpdatedAt.Format(time.RFC3339),
		},
		"exercise": ExerciseSummary{
			ID:          ex.ID,
			Title:       ex.Title,
			Difficulty:  string(ex.Difficulty),
			Description: ex.Description,
			Tags:        ex.Tags,
		},
	})
}

// getUserIDFromContext extracts the user ID from request context
func getUserIDFromContext(ctx interface{ Value(any) any }) (uuid.UUID, bool) {
	user := ctx.Value(ContextKeyUser)
	if user == nil {
		return uuid.Nil, false
	}
	if u, ok := user.(*domain.User); ok {
		return u.ID, true
	}
	return uuid.Nil, false
}

// ContextKey type for context keys
type ContextKey string

const (
	ContextKeyUser ContextKey = "user"
)

func (h *ExerciseHandler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ExerciseHandler) jsonError(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
