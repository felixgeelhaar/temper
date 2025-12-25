package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/felixgeelhaar/temper/internal/workspace"
	"github.com/google/uuid"
)

// WorkspaceHandler handles workspace endpoints
type WorkspaceHandler struct {
	service *workspace.Service
}

// NewWorkspaceHandler creates a new workspace handler
func NewWorkspaceHandler(service *workspace.Service) *WorkspaceHandler {
	return &WorkspaceHandler{service: service}
}

// WorkspaceResponse represents a workspace in API responses
type WorkspaceResponse struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	ExerciseID *string           `json:"exercise_id,omitempty"`
	Content    map[string]string `json:"content,omitempty"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
}

// CreateWorkspaceRequest is the request body for creating a workspace
type CreateWorkspaceRequest struct {
	Name    string            `json:"name"`
	Content map[string]string `json:"content"`
}

// UpdateWorkspaceRequest is the request body for updating a workspace
type UpdateWorkspaceRequest struct {
	Name    *string           `json:"name,omitempty"`
	Content map[string]string `json:"content,omitempty"`
}

// VersionResponse represents a version in API responses
type VersionResponse struct {
	ID        string            `json:"id"`
	Version   int               `json:"version"`
	Content   map[string]string `json:"content,omitempty"`
	CreatedAt string            `json:"created_at"`
}

// List returns all workspaces for the current user
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	workspaces, err := h.service.List(r.Context(), userID, limit, offset)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}

	response := make([]WorkspaceResponse, 0, len(workspaces))
	for _, ws := range workspaces {
		response = append(response, WorkspaceResponse{
			ID:         ws.ID.String(),
			Name:       ws.Name,
			ExerciseID: ws.ExerciseID,
			CreatedAt:  ws.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  ws.UpdatedAt.Format(time.RFC3339),
		})
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"workspaces": response,
		"total":      len(response),
	})
}

// Create creates a new workspace
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	ws, err := h.service.Create(r.Context(), workspace.CreateRequest{
		UserID:  userID,
		Name:    req.Name,
		Content: req.Content,
	})
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to create workspace")
		return
	}

	h.jsonResponse(w, http.StatusCreated, WorkspaceResponse{
		ID:        ws.ID.String(),
		Name:      ws.Name,
		Content:   ws.Content,
		CreatedAt: ws.CreatedAt.Format(time.RFC3339),
		UpdatedAt: ws.UpdatedAt.Format(time.RFC3339),
	})
}

// Get retrieves a workspace by ID
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	ws, err := h.service.Get(r.Context(), wsID, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "workspace not found")
		return
	}

	h.jsonResponse(w, http.StatusOK, WorkspaceResponse{
		ID:         ws.ID.String(),
		Name:       ws.Name,
		ExerciseID: ws.ExerciseID,
		Content:    ws.Content,
		CreatedAt:  ws.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  ws.UpdatedAt.Format(time.RFC3339),
	})
}

// Update updates a workspace
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	var req UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ws, err := h.service.Update(r.Context(), workspace.UpdateRequest{
		ID:      wsID,
		UserID:  userID,
		Name:    req.Name,
		Content: req.Content,
	})
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "workspace not found")
		return
	}

	h.jsonResponse(w, http.StatusOK, WorkspaceResponse{
		ID:         ws.ID.String(),
		Name:       ws.Name,
		ExerciseID: ws.ExerciseID,
		Content:    ws.Content,
		CreatedAt:  ws.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  ws.UpdatedAt.Format(time.RFC3339),
	})
}

// Delete removes a workspace
func (h *WorkspaceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	if err := h.service.Delete(r.Context(), wsID, userID); err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to delete workspace")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{"message": "workspace deleted"})
}

// CreateSnapshot creates a version snapshot
func (h *WorkspaceHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	version, err := h.service.CreateSnapshot(r.Context(), wsID, userID)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to create snapshot")
		return
	}

	h.jsonResponse(w, http.StatusCreated, VersionResponse{
		ID:        version.ID.String(),
		Version:   version.Version,
		CreatedAt: version.CreatedAt.Format(time.RFC3339),
	})
}

// ListVersions returns version history for a workspace
func (h *WorkspaceHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	versions, err := h.service.ListVersions(r.Context(), wsID, userID, limit)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "workspace not found")
		return
	}

	response := make([]VersionResponse, 0, len(versions))
	for _, v := range versions {
		response = append(response, VersionResponse{
			ID:        v.ID.String(),
			Version:   v.Version,
			CreatedAt: v.CreatedAt.Format(time.RFC3339),
		})
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"versions": response,
		"total":    len(response),
	})
}

func (h *WorkspaceHandler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *WorkspaceHandler) jsonError(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
