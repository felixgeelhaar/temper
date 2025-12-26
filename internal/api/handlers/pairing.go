package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/storage"
	"github.com/felixgeelhaar/temper/internal/workspace"
	"github.com/google/uuid"
)

// PairingHandler handles pairing/intervention endpoints
type PairingHandler struct {
	pairingService   *pairing.Service
	workspaceService *workspace.Service
	exerciseRegistry *exercise.Registry
	queries          *storage.Queries
}

// NewPairingHandler creates a new pairing handler
func NewPairingHandler(pairingService *pairing.Service, workspaceService *workspace.Service, exerciseRegistry *exercise.Registry, db *sql.DB) *PairingHandler {
	return &PairingHandler{
		pairingService:   pairingService,
		workspaceService: workspaceService,
		exerciseRegistry: exerciseRegistry,
		queries:          storage.New(db),
	}
}

// StartSessionRequest is the request body for starting a pairing session
type StartSessionRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// SessionResponse represents a pairing session in API responses
type SessionResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	MaxLevel    int    `json:"max_level"`
	CreatedAt   string `json:"created_at"`
}

// InterveneRequest is the request body for requesting an intervention
type InterveneRequest struct {
	SessionID   string            `json:"session_id"`
	Intent      string            `json:"intent"`
	RunOutput   *domain.RunOutput `json:"run_output,omitempty"`
}

// InterventionResponse represents an intervention in API responses
type InterventionResponse struct {
	ID       string `json:"id"`
	Level    int    `json:"level"`
	LevelStr string `json:"level_str"`
	Type     string `json:"type"`
	Content  string `json:"content"`
}

// StartSession starts a new pairing session
func (h *PairingHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req StartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	wsID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid workspace ID")
		return
	}

	// Verify workspace access
	ws, err := h.workspaceService.Get(r.Context(), wsID, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "workspace not found")
		return
	}

	// Determine policy
	policy := domain.DefaultPolicy()
	if ws.ExerciseID != nil {
		parts := parseExerciseID(*ws.ExerciseID)
		if pack, err := h.exerciseRegistry.GetPack(parts[0]); err == nil && pack != nil {
			policy = pack.DefaultPolicy
		}
	}

	// Serialize policy to JSON
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to serialize policy")
		return
	}

	// Create session in database
	session, err := h.queries.CreatePairingSession(r.Context(), storage.CreatePairingSessionParams{
		UserID:     userID,
		ArtifactID: uuid.NullUUID{UUID: wsID, Valid: true},
		ExerciseID: sql.NullString{String: stringPtrToString(ws.ExerciseID), Valid: ws.ExerciseID != nil},
		Policy:     policyJSON,
	})
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create session: %v", err))
		return
	}

	h.jsonResponse(w, http.StatusCreated, SessionResponse{
		ID:          session.ID.String(),
		WorkspaceID: wsID.String(),
		MaxLevel:    int(policy.MaxLevel),
		CreatedAt:   session.StartedAt.Format(time.RFC3339),
	})
}

// Intervene requests an intervention
func (h *PairingHandler) Intervene(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req InterveneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	// Get session from database
	session, err := h.queries.GetPairingSession(r.Context(), sessionID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	if session.UserID != userID {
		h.jsonError(w, http.StatusForbidden, "access denied")
		return
	}

	// Parse policy from JSON
	var policy domain.LearningPolicy
	if err := json.Unmarshal(session.Policy, &policy); err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}

	// Get workspace code
	ws, err := h.workspaceService.Get(r.Context(), session.ArtifactID.UUID, userID)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to get workspace")
		return
	}

	// Get exercise if available
	var ex *domain.Exercise
	if session.ExerciseID.Valid {
		ex, _ = h.exerciseRegistry.GetExercise(session.ExerciseID.String)
	}

	// Parse intent
	intent := parseIntent(req.Intent)

	// Request intervention
	intervention, err := h.pairingService.Intervene(r.Context(), pairing.InterventionRequest{
		UserID:    userID,
		SessionID: sessionID,
		Intent:    intent,
		Context: pairing.InterventionContext{
			Exercise:  ex,
			Code:      ws.Content,
			RunOutput: req.RunOutput,
		},
		Policy: policy,
	})
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("intervention failed: %v", err))
		return
	}

	h.jsonResponse(w, http.StatusOK, InterventionResponse{
		ID:       intervention.ID.String(),
		Level:    int(intervention.Level),
		LevelStr: intervention.Level.String(),
		Type:     string(intervention.Type),
		Content:  intervention.Content,
	})
}

// StreamIntervention streams an intervention response via SSE
func (h *PairingHandler) StreamIntervention(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		h.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sessionID, err := uuid.Parse(r.PathValue("session_id"))
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	// Get session from database
	session, err := h.queries.GetPairingSession(r.Context(), sessionID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	if session.UserID != userID {
		h.jsonError(w, http.StatusForbidden, "access denied")
		return
	}

	// Parse policy from JSON
	var policy domain.LearningPolicy
	if err := json.Unmarshal(session.Policy, &policy); err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to parse policy")
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
	fmt.Fprintf(w, "event: connected\ndata: {\"session_id\": \"%s\"}\n\n", sessionID)
	flusher.Flush()

	// Parse intent from query params
	intentStr := r.URL.Query().Get("intent")
	if intentStr == "" {
		intentStr = "hint"
	}
	intent := parseIntent(intentStr)

	// Get workspace code
	ws, err := h.workspaceService.Get(r.Context(), session.ArtifactID.UUID, userID)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"failed to get workspace\"}\n\n")
		flusher.Flush()
		return
	}

	// Get exercise if available
	var ex *domain.Exercise
	if session.ExerciseID.Valid {
		ex, _ = h.exerciseRegistry.GetExercise(session.ExerciseID.String)
	}

	// Update learning profile - increment hint count
	_, err = h.queries.IncrementProfileHints(r.Context(), userID)
	if err != nil {
		// Profile might not exist - try creating it first
		_, createErr := h.queries.CreateLearningProfile(r.Context(), userID)
		if createErr == nil {
			_, _ = h.queries.IncrementProfileHints(r.Context(), userID)
		} else {
			slog.Warn("failed to update profile hints", "user_id", userID, "error", err)
		}
	}

	// Stream intervention
	chunks, err := h.pairingService.IntervenStream(r.Context(), pairing.InterventionRequest{
		UserID:    userID,
		SessionID: sessionID,
		Intent:    intent,
		Context: pairing.InterventionContext{
			Exercise: ex,
			Code:     ws.Content,
		},
		Policy: policy,
	})
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Stream chunks
	for chunk := range chunks {
		if chunk.Error != nil {
			fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", chunk.Error.Error())
			flusher.Flush()
			break
		}

		data, _ := json.Marshal(map[string]string{"content": chunk.Content})
		fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", data)
		flusher.Flush()

		if chunk.Type == "done" {
			break
		}
	}

	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

// parseIntent converts a string to an Intent
func parseIntent(s string) domain.Intent {
	switch s {
	case "hint":
		return domain.IntentHint
	case "review":
		return domain.IntentReview
	case "stuck":
		return domain.IntentStuck
	case "next":
		return domain.IntentNext
	case "explain":
		return domain.IntentExplain
	default:
		return domain.IntentHint
	}
}

// GetSession retrieves a session from database
func (h *PairingHandler) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*storage.PairingSession, error) {
	session, err := h.queries.GetPairingSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}
	return &session, nil
}

// stringPtrToString converts *string to string
func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *PairingHandler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *PairingHandler) jsonError(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
