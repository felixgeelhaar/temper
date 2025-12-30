package patch

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

var (
	ErrPatchNotFound = errors.New("patch not found")
	ErrPatchExpired  = errors.New("patch has expired")
	ErrPatchApplied  = errors.New("patch already applied")
	ErrPatchRejected = errors.New("patch was rejected")
)

// Service manages patch lifecycle
type Service struct {
	extractor *Extractor
	logger    *Logger
	mu        sync.RWMutex
	patches   map[uuid.UUID]*domain.Patch       // patchID -> patch
	sessions  map[uuid.UUID][]*domain.Patch     // sessionID -> patches
	pending   map[uuid.UUID]*domain.Patch       // sessionID -> current pending patch
}

// NewService creates a new patch service
func NewService() *Service {
	return &Service{
		extractor: NewExtractor(),
		patches:   make(map[uuid.UUID]*domain.Patch),
		sessions:  make(map[uuid.UUID][]*domain.Patch),
		pending:   make(map[uuid.UUID]*domain.Patch),
	}
}

// NewServiceWithLogger creates a new patch service with logging enabled
func NewServiceWithLogger(logDir string) (*Service, error) {
	logger, err := NewLogger(logDir)
	if err != nil {
		return nil, err
	}

	return &Service{
		extractor: NewExtractor(),
		logger:    logger,
		patches:   make(map[uuid.UUID]*domain.Patch),
		sessions:  make(map[uuid.UUID][]*domain.Patch),
		pending:   make(map[uuid.UUID]*domain.Patch),
	}, nil
}

// SetLogger sets the logger for the service
func (s *Service) SetLogger(logger *Logger) {
	s.logger = logger
}

// GetLogger returns the logger (may be nil if logging is disabled)
func (s *Service) GetLogger() *Logger {
	return s.logger
}

// ExtractFromIntervention extracts patches from an intervention and stores them
func (s *Service) ExtractFromIntervention(intervention *domain.Intervention, sessionID uuid.UUID, currentCode map[string]string) []*domain.Patch {
	patches := s.extractor.ExtractPatches(intervention, sessionID, currentCode)
	if len(patches) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, p := range patches {
		p.CreatedAt = now
		s.patches[p.ID] = p
		s.sessions[sessionID] = append(s.sessions[sessionID], p)

		// Log patch creation
		if s.logger != nil {
			_ = s.logger.Log(LogActionCreated, p)
		}
	}

	// Set the first pending patch for the session
	if len(patches) > 0 && s.pending[sessionID] == nil {
		s.pending[sessionID] = patches[0]
	}

	return patches
}

// GetPending returns the current pending patch for a session
func (s *Service) GetPending(sessionID uuid.UUID) *domain.Patch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pending[sessionID]
}

// GetPatch retrieves a patch by ID
func (s *Service) GetPatch(patchID uuid.UUID) (*domain.Patch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	patch, ok := s.patches[patchID]
	if !ok {
		return nil, ErrPatchNotFound
	}
	return patch, nil
}

// GetSessionPatches returns all patches for a session
func (s *Service) GetSessionPatches(sessionID uuid.UUID) []*domain.Patch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

// Preview generates a preview for a patch
func (s *Service) Preview(patchID uuid.UUID) (*domain.PatchPreview, error) {
	s.mu.RLock()
	patch, ok := s.patches[patchID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrPatchNotFound
	}

	// Count additions and deletions
	additions := 0
	deletions := 0
	for _, line := range strings.Split(patch.Diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	// Extract context lines (first few lines of original/proposed)
	contextBefore := extractContext(patch.Original, 3)
	contextAfter := extractContext(patch.Proposed, 3)

	// Check for potential issues
	var warnings []string
	if patch.Original == "" {
		warnings = append(warnings, "This creates a new file")
	}
	if additions > 50 {
		warnings = append(warnings, "Large change: adds many lines")
	}

	// Log preview action
	if s.logger != nil {
		_ = s.logger.Log(LogActionPreviewed, patch)
	}

	return &domain.PatchPreview{
		Patch:         patch,
		ContextBefore: contextBefore,
		ContextAfter:  contextAfter,
		Additions:     additions,
		Deletions:     deletions,
		Warnings:      warnings,
	}, nil
}

// PreviewPending generates a preview for the current pending patch
func (s *Service) PreviewPending(sessionID uuid.UUID) (*domain.PatchPreview, error) {
	s.mu.RLock()
	patch := s.pending[sessionID]
	s.mu.RUnlock()

	if patch == nil {
		return nil, ErrPatchNotFound
	}

	return s.Preview(patch.ID)
}

// Approve marks a patch as approved
func (s *Service) Approve(patchID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	patch, ok := s.patches[patchID]
	if !ok {
		return ErrPatchNotFound
	}

	if patch.Status == domain.PatchStatusApplied {
		return ErrPatchApplied
	}
	if patch.Status == domain.PatchStatusRejected {
		return ErrPatchRejected
	}
	if patch.Status == domain.PatchStatusExpired {
		return ErrPatchExpired
	}

	patch.Status = domain.PatchStatusApproved
	return nil
}

// Apply marks a patch as applied and returns the proposed content
func (s *Service) Apply(patchID uuid.UUID) (file string, content string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	patch, ok := s.patches[patchID]
	if !ok {
		return "", "", ErrPatchNotFound
	}

	if !patch.CanApply() {
		switch patch.Status {
		case domain.PatchStatusApplied:
			return "", "", ErrPatchApplied
		case domain.PatchStatusRejected:
			return "", "", ErrPatchRejected
		case domain.PatchStatusExpired:
			return "", "", ErrPatchExpired
		}
	}

	now := time.Now()
	patch.Status = domain.PatchStatusApplied
	patch.AppliedAt = &now

	// Log applied action
	if s.logger != nil {
		_ = s.logger.Log(LogActionApplied, patch)
	}

	// Move to next pending patch in session
	s.advancePending(patch.SessionID, patchID)

	return patch.File, patch.Proposed, nil
}

// ApplyPending applies the current pending patch for a session
func (s *Service) ApplyPending(sessionID uuid.UUID) (file string, content string, err error) {
	s.mu.RLock()
	patch := s.pending[sessionID]
	s.mu.RUnlock()

	if patch == nil {
		return "", "", ErrPatchNotFound
	}

	return s.Apply(patch.ID)
}

// Reject marks a patch as rejected
func (s *Service) Reject(patchID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	patch, ok := s.patches[patchID]
	if !ok {
		return ErrPatchNotFound
	}

	if patch.Status == domain.PatchStatusApplied {
		return ErrPatchApplied
	}

	patch.Status = domain.PatchStatusRejected

	// Log rejected action
	if s.logger != nil {
		_ = s.logger.Log(LogActionRejected, patch)
	}

	// Move to next pending patch
	s.advancePending(patch.SessionID, patchID)

	return nil
}

// RejectPending rejects the current pending patch for a session
func (s *Service) RejectPending(sessionID uuid.UUID) error {
	s.mu.RLock()
	patch := s.pending[sessionID]
	s.mu.RUnlock()

	if patch == nil {
		return ErrPatchNotFound
	}

	return s.Reject(patch.ID)
}

// ExpireSession marks all pending patches in a session as expired
func (s *Service) ExpireSession(sessionID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	patches := s.sessions[sessionID]
	for _, p := range patches {
		if p.Status == domain.PatchStatusPending {
			p.Status = domain.PatchStatusExpired

			// Log expired action
			if s.logger != nil {
				_ = s.logger.Log(LogActionExpired, p)
			}
		}
	}
	delete(s.pending, sessionID)
}

func (s *Service) advancePending(sessionID, currentID uuid.UUID) {
	patches := s.sessions[sessionID]
	foundCurrent := false
	for _, p := range patches {
		if p.ID == currentID {
			foundCurrent = true
			continue
		}
		if foundCurrent && p.Status == domain.PatchStatusPending {
			s.pending[sessionID] = p
			return
		}
	}
	// No more pending patches
	delete(s.pending, sessionID)
}

func extractContext(content string, lines int) []string {
	if content == "" {
		return nil
	}
	allLines := strings.Split(content, "\n")
	if len(allLines) <= lines {
		return allLines
	}
	return allLines[:lines]
}
