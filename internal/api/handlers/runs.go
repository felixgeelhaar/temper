package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/storage"
	"github.com/felixgeelhaar/temper/internal/workspace"
	"github.com/google/uuid"
)

// RunHandler handles run endpoints
type RunHandler struct {
	runnerService    *runner.Service
	workspaceService *workspace.Service
	exerciseRegistry *exercise.Registry
	queries          *storage.Queries
}

// NewRunHandler creates a new run handler
func NewRunHandler(runnerService *runner.Service, workspaceService *workspace.Service, exerciseRegistry *exercise.Registry, db *sql.DB) *RunHandler {
	return &RunHandler{
		runnerService:    runnerService,
		workspaceService: workspaceService,
		exerciseRegistry: exerciseRegistry,
		queries:          storage.New(db),
	}
}

// TriggerRunRequest is the request body for triggering a run
type TriggerRunRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// RunResponse represents a run in API responses
type RunResponse struct {
	ID           string                `json:"id"`
	WorkspaceID  string                `json:"workspace_id"`
	Status       string                `json:"status"`
	Output       *domain.RunOutput     `json:"output,omitempty"`
	Duration     string                `json:"duration,omitempty"`
	CreatedAt    string                `json:"created_at"`
}

// TriggerRun starts a new code execution
func (h *RunHandler) TriggerRun(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req TriggerRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	wsID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	// Get workspace
	ws, err := h.workspaceService.Get(r.Context(), wsID, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "workspace not found")
		return
	}

	// Determine check recipe - default to format, build, test
	recipe := domain.CheckRecipe{
		Format:  true,
		Build:   true,
		Test:    true,
		Timeout: 30,
	}
	if ws.ExerciseID != nil {
		// Try to get exercise-specific recipe
		if ex, err := h.exerciseRegistry.GetExercise(*ws.ExerciseID); err == nil && ex != nil {
			recipe = ex.CheckRecipe
		}
	}

	// Create run
	runID := uuid.New()
	output, err := h.runnerService.Execute(r.Context(), runner.ExecuteRequest{
		RunID:      runID,
		UserID:     userID,
		ArtifactID: wsID,
		ExerciseID: ws.ExerciseID,
		Code:       ws.Content,
		Recipe:     recipe,
	})

	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("execution failed: %v", err))
		return
	}

	// Update learning profile - increment run count
	_, err = h.queries.IncrementProfileRuns(r.Context(), userID)
	if err != nil {
		// Profile might not exist - try creating it first
		_, createErr := h.queries.CreateLearningProfile(r.Context(), userID)
		if createErr == nil {
			// Profile created, increment now
			_, _ = h.queries.IncrementProfileRuns(r.Context(), userID)
		} else {
			slog.Warn("failed to update profile runs", "user_id", userID, "error", err)
		}
	}

	h.jsonResponse(w, http.StatusOK, RunResponse{
		ID:          runID.String(),
		WorkspaceID: wsID.String(),
		Status:      "completed",
		Output:      output,
		Duration:    output.Duration.String(),
		CreatedAt:   time.Now().Format(time.RFC3339),
	})
}

// GetRun retrieves a run by ID
func (h *RunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	_, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	runID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Check if run is still in progress
	if h.runnerService.IsRunning(runID) {
		h.jsonResponse(w, http.StatusOK, RunResponse{
			ID:     runID.String(),
			Status: "running",
		})
		return
	}

	// For now, return not found for completed runs
	// In a full implementation, we would store run results in the database
	h.jsonError(w, http.StatusNotFound, "run not found or already completed")
}

// FormatWorkspace formats the code in a workspace using gofmt
func (h *RunHandler) FormatWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wsID, err := uuid.Parse(r.PathValue("workspace_id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	// Get workspace
	ws, err := h.workspaceService.Get(r.Context(), wsID, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "workspace not found")
		return
	}

	// Format the code
	formattedCode, err := h.runnerService.FormatCode(r.Context(), ws.Content)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("format failed: %v", err))
		return
	}

	// Update the workspace with formatted code
	_, err = h.workspaceService.Update(r.Context(), workspace.UpdateRequest{
		ID:      wsID,
		UserID:  userID,
		Content: formattedCode,
	})
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to update workspace")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"content": formattedCode,
		"message": "code formatted successfully",
	})
}

// StreamRun streams run progress via SSE
func (h *RunHandler) StreamRun(w http.ResponseWriter, r *http.Request) {
	_, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	runID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"run_id\": \"%s\"}\n\n", runID)
	flusher.Flush()

	// Wait for run to complete or context to be done
	if h.runnerService.IsRunning(runID) {
		err := h.runnerService.Wait(r.Context(), runID)
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
		} else {
			fmt.Fprintf(w, "event: completed\ndata: {\"status\": \"completed\"}\n\n")
		}
		flusher.Flush()
	} else {
		fmt.Fprintf(w, "event: completed\ndata: {\"status\": \"not_running\"}\n\n")
		flusher.Flush()
	}
}

// parseExerciseID splits "pack/slug" into parts
func parseExerciseID(id string) []string {
	for i, c := range id {
		if c == '/' {
			return []string{id[:i], id[i+1:]}
		}
	}
	return []string{id, ""}
}

func (h *RunHandler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *RunHandler) jsonError(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
