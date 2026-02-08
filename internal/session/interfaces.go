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

// SessionStore defines the persistence interface for sessions.
// Both the JSON file store and SQLite store implement this.
type SessionStore interface {
	Save(session *Session) error
	Get(id string) (*Session, error)
	Delete(id string) error
	List() ([]string, error)
	ListActive() ([]*Session, error)
	Exists(id string) bool

	SaveRun(run *Run) error
	GetRun(sessionID, runID string) (*Run, error)
	ListRuns(sessionID string) ([]string, error)

	SaveIntervention(intervention *Intervention) error
	GetIntervention(sessionID, interventionID string) (*Intervention, error)
	ListInterventions(sessionID string) ([]string, error)
}

// Ensure Store (JSON) implements SessionStore
var _ SessionStore = (*Store)(nil)
