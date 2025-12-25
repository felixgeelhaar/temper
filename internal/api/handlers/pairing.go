package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/workspace"
	"github.com/google/uuid"
)

// PairingHandler handles pairing/intervention endpoints
type PairingHandler struct {
	pairingService   *pairing.Service
	workspaceService *workspace.Service
	exerciseRegistry *exercise.Registry
	sessions         map[uuid.UUID]*pairingSession
	mu               sync.RWMutex
}

type pairingSession struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	WorkspaceID uuid.UUID
	ExerciseID  *string
	Policy      domain.LearningPolicy
	CreatedAt   time.Time
}

// NewPairingHandler creates a new pairing handler
func NewPairingHandler(pairingService *pairing.Service, workspaceService *workspace.Service, exerciseRegistry *exercise.Registry) *PairingHandler {
	return &PairingHandler{
		pairingService:   pairingService,
		workspaceService: workspaceService,
		exerciseRegistry: exerciseRegistry,
		sessions:         make(map[uuid.UUID]*pairingSession),
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

	// Create session
	session := &pairingSession{
		ID:          uuid.New(),
		UserID:      userID,
		WorkspaceID: wsID,
		ExerciseID:  ws.ExerciseID,
		Policy:      policy,
		CreatedAt:   time.Now(),
	}

	h.mu.Lock()
	h.sessions[session.ID] = session
	h.mu.Unlock()

	h.jsonResponse(w, http.StatusCreated, SessionResponse{
		ID:          session.ID.String(),
		WorkspaceID: wsID.String(),
		MaxLevel:    int(policy.MaxLevel),
		CreatedAt:   session.CreatedAt.Format(time.RFC3339),
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

	// Get session
	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	if !ok || session.UserID != userID {
		h.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	// Get workspace code
	ws, err := h.workspaceService.Get(r.Context(), session.WorkspaceID, userID)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "failed to get workspace")
		return
	}

	// Get exercise if available
	var ex *domain.Exercise
	if session.ExerciseID != nil {
		ex, _ = h.exerciseRegistry.GetExercise(*session.ExerciseID)
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
		Policy: session.Policy,
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

	// Get session
	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	if !ok || session.UserID != userID {
		h.jsonError(w, http.StatusNotFound, "session not found")
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
	ws, err := h.workspaceService.Get(r.Context(), session.WorkspaceID, userID)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"failed to get workspace\"}\n\n")
		flusher.Flush()
		return
	}

	// Get exercise if available
	var ex *domain.Exercise
	if session.ExerciseID != nil {
		ex, _ = h.exerciseRegistry.GetExercise(*session.ExerciseID)
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
		Policy: session.Policy,
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

// GetSession retrieves a session
func (h *PairingHandler) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*pairingSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	session, ok := h.sessions[sessionID]
	if !ok || session.UserID != userID {
		return nil, false
	}
	return session, true
}

func (h *PairingHandler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *PairingHandler) jsonError(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
