package session

import (
	"context"
)

// SessionService defines the interface for session management operations
// used by the daemon handlers
type SessionService interface {
	// Create starts a new pairing session
	Create(ctx context.Context, req CreateRequest) (*Session, error)

	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (*Session, error)

	// Delete removes a session
	Delete(ctx context.Context, id string) error

	// RunCode executes code in a session
	RunCode(ctx context.Context, sessionID string, req RunRequest) (*Run, error)

	// UpdateCode updates the code in a session
	UpdateCode(ctx context.Context, id string, code map[string]string) (*Session, error)

	// RecordIntervention records an intervention in a session
	RecordIntervention(ctx context.Context, intervention *Intervention) error
}

// Ensure Service implements SessionService
var _ SessionService = (*Service)(nil)
