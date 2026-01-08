package patch

import (
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// PatchService defines the interface for patch management operations
// used by the daemon handlers
type PatchService interface {
	// ExtractFromIntervention extracts patches from an intervention and stores them
	ExtractFromIntervention(intervention *domain.Intervention, sessionID uuid.UUID, currentCode map[string]string) []*domain.Patch

	// PreviewPending generates a preview for the current pending patch
	PreviewPending(sessionID uuid.UUID) (*domain.PatchPreview, error)

	// ApplyPending applies the current pending patch for a session
	ApplyPending(sessionID uuid.UUID) (file string, content string, err error)

	// RejectPending rejects the current pending patch for a session
	RejectPending(sessionID uuid.UUID) error

	// GetSessionPatches returns all patches for a session
	GetSessionPatches(sessionID uuid.UUID) []*domain.Patch

	// GetLogger returns the logger (may be nil if logging is disabled)
	GetLogger() *Logger
}

// Ensure Service implements PatchService
var _ PatchService = (*Service)(nil)
