package daemon

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/sandbox"
)

// Sandbox handlers

func (s *Server) handleCreateSandbox(w http.ResponseWriter, r *http.Request) {
	if s.SandboxManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "sandbox support not available (Docker required)", nil)
		return
	}

	sessionID := r.PathValue("id")

	var req struct {
		Language   string  `json:"language,omitempty"`
		Image      string  `json:"image,omitempty"`
		MemoryMB   int     `json:"memory_mb,omitempty"`
		CPULimit   float64 `json:"cpu_limit,omitempty"`
		NetworkOff *bool   `json:"network_off,omitempty"`
	}

	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	cfg := sandbox.DefaultConfig()
	if req.Language != "" {
		cfg.Language = req.Language
	}
	if req.Image != "" {
		cfg.Image = req.Image
	}
	if req.MemoryMB > 0 {
		cfg.MemoryMB = req.MemoryMB
	}
	if req.CPULimit > 0 {
		cfg.CPULimit = req.CPULimit
	}
	if req.NetworkOff != nil {
		cfg.NetworkOff = *req.NetworkOff
	}

	sb, err := s.SandboxManager.Create(r.Context(), sessionID, cfg)
	if err != nil {
		switch err {
		case sandbox.ErrSessionHasSandbox:
			s.jsonError(w, http.StatusConflict, "session already has a sandbox", nil)
		case sandbox.ErrMaxSandboxes:
			s.jsonError(w, http.StatusTooManyRequests, "maximum concurrent sandboxes reached", nil)
		default:
			s.jsonError(w, http.StatusInternalServerError, "failed to create sandbox", err)
		}
		return
	}

	s.jsonResponse(w, http.StatusCreated, sb)
}

func (s *Server) handleGetSandbox(w http.ResponseWriter, r *http.Request) {
	if s.SandboxManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "sandbox support not available", nil)
		return
	}

	sessionID := r.PathValue("id")

	sb, err := s.SandboxManager.GetBySession(r.Context(), sessionID)
	if err != nil {
		if err == sandbox.ErrSandboxNotFound {
			s.jsonError(w, http.StatusNotFound, "no sandbox for this session", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get sandbox", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, sb)
}

func (s *Server) handleDestroySandbox(w http.ResponseWriter, r *http.Request) {
	if s.SandboxManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "sandbox support not available", nil)
		return
	}

	sessionID := r.PathValue("id")

	sb, err := s.SandboxManager.GetBySession(r.Context(), sessionID)
	if err != nil {
		if err == sandbox.ErrSandboxNotFound {
			s.jsonError(w, http.StatusNotFound, "no sandbox for this session", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get sandbox", err)
		return
	}

	if err := s.SandboxManager.Destroy(r.Context(), sb.ID); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to destroy sandbox", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"destroyed": true,
	})
}

func (s *Server) handleSandboxExec(w http.ResponseWriter, r *http.Request) {
	if s.SandboxManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "sandbox support not available", nil)
		return
	}

	sessionID := r.PathValue("id")

	var req struct {
		Cmd     []string          `json:"cmd"`
		Code    map[string]string `json:"code,omitempty"`
		Timeout int               `json:"timeout,omitempty"` // seconds
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxRunBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	if len(req.Cmd) == 0 {
		s.jsonError(w, http.StatusBadRequest, "cmd is required", nil)
		return
	}

	cmdName := strings.TrimSpace(req.Cmd[0])
	if cmdName == "" {
		s.jsonError(w, http.StatusBadRequest, "cmd is required", nil)
		return
	}
	cmdBase := path.Base(cmdName)
	if !isSandboxCommandAllowed(cmdBase) {
		s.jsonError(w, http.StatusBadRequest, "command not allowed", nil)
		return
	}

	sb, err := s.SandboxManager.GetBySession(r.Context(), sessionID)
	if err != nil {
		if err == sandbox.ErrSandboxNotFound {
			s.jsonError(w, http.StatusNotFound, "no sandbox for this session", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get sandbox", err)
		return
	}

	// Copy code if provided
	if len(req.Code) > 0 {
		if err := s.SandboxManager.AttachCode(r.Context(), sb.ID, req.Code); err != nil {
			s.jsonError(w, http.StatusInternalServerError, "failed to copy files to sandbox", err)
			return
		}
	}

	timeout := 120 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	result, err := s.SandboxManager.Execute(r.Context(), sb.ID, req.Cmd, timeout)
	if err != nil {
		switch err {
		case sandbox.ErrSandboxExpired:
			s.jsonError(w, http.StatusGone, "sandbox has expired", nil)
		case sandbox.ErrSandboxNotReady:
			s.jsonError(w, http.StatusConflict, "sandbox is not ready", nil)
		default:
			s.jsonError(w, http.StatusInternalServerError, "execution failed", err)
		}
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

func isSandboxCommandAllowed(cmd string) bool {
	allowed := map[string]struct{}{
		"bash":        {},
		"echo":        {},
		"git":         {},
		"go":          {},
		"gofmt":       {},
		"goimports":   {},
		"govulncheck": {},
		"make":        {},
		"node":        {},
		"npm":         {},
		"pnpm":        {},
		"python":      {},
		"python3":     {},
		"sh":          {},
		"staticcheck": {},
		"yarn":        {},
	}

	_, ok := allowed[strings.ToLower(cmd)]
	return ok
}

func (s *Server) handlePauseSandbox(w http.ResponseWriter, r *http.Request) {
	if s.SandboxManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "sandbox support not available", nil)
		return
	}

	sessionID := r.PathValue("id")

	sb, err := s.SandboxManager.GetBySession(r.Context(), sessionID)
	if err != nil {
		if err == sandbox.ErrSandboxNotFound {
			s.jsonError(w, http.StatusNotFound, "no sandbox for this session", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get sandbox", err)
		return
	}

	if err := s.SandboxManager.Pause(r.Context(), sb.ID); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to pause sandbox", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"paused": true,
	})
}

func (s *Server) handleResumeSandbox(w http.ResponseWriter, r *http.Request) {
	if s.SandboxManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "sandbox support not available", nil)
		return
	}

	sessionID := r.PathValue("id")

	sb, err := s.SandboxManager.GetBySession(r.Context(), sessionID)
	if err != nil {
		if err == sandbox.ErrSandboxNotFound {
			s.jsonError(w, http.StatusNotFound, "no sandbox for this session", nil)
			return
		}
		s.jsonError(w, http.StatusInternalServerError, "failed to get sandbox", err)
		return
	}

	if err := s.SandboxManager.Resume(r.Context(), sb.ID); err != nil {
		s.jsonError(w, http.StatusInternalServerError, "failed to resume sandbox", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"resumed": true,
	})
}
