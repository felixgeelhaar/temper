package session

import (
	"errors"
	"fmt"

	"github.com/felixgeelhaar/temper/internal/storage/local"
)

const (
	collectionSessions  = "sessions"
	subdirRuns          = "runs"
	subdirInterventions = "interventions"
)

var (
	ErrNotFound = errors.New("session not found")
)

// Store handles session persistence
type Store struct {
	store *local.Store
}

// NewStore creates a new session store
func NewStore(basePath string) (*Store, error) {
	store, err := local.NewStore(basePath)
	if err != nil {
		return nil, fmt.Errorf("create local store: %w", err)
	}
	return &Store{store: store}, nil
}

// Save persists a session
func (s *Store) Save(session *Session) error {
	return s.store.Save(collectionSessions, session.ID, session)
}

// Get retrieves a session by ID
func (s *Store) Get(id string) (*Session, error) {
	var session Session
	if err := s.store.Load(collectionSessions, id, &session); err != nil {
		if errors.Is(err, local.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &session, nil
}

// Delete removes a session
func (s *Store) Delete(id string) error {
	if err := s.store.Delete(collectionSessions, id); err != nil {
		if errors.Is(err, local.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// List returns all session IDs
func (s *Store) List() ([]string, error) {
	return s.store.List(collectionSessions)
}

// ListActive returns all active sessions
func (s *Store) ListActive() ([]*Session, error) {
	ids, err := s.store.List(collectionSessions)
	if err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, id := range ids {
		session, err := s.Get(id)
		if err != nil {
			continue
		}
		if session.Status == StatusActive {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// SaveRun persists a run within a session
func (s *Store) SaveRun(run *Run) error {
	return s.store.SaveDir(collectionSessions, run.SessionID, subdirRuns, run.ID, run)
}

// GetRun retrieves a run by ID
func (s *Store) GetRun(sessionID, runID string) (*Run, error) {
	var run Run
	if err := s.store.LoadDir(collectionSessions, sessionID, subdirRuns, runID, &run); err != nil {
		if errors.Is(err, local.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &run, nil
}

// ListRuns returns all run IDs for a session
func (s *Store) ListRuns(sessionID string) ([]string, error) {
	return s.store.ListDir(collectionSessions, sessionID, subdirRuns)
}

// SaveIntervention persists an intervention within a session
func (s *Store) SaveIntervention(intervention *Intervention) error {
	return s.store.SaveDir(collectionSessions, intervention.SessionID, subdirInterventions, intervention.ID, intervention)
}

// GetIntervention retrieves an intervention by ID
func (s *Store) GetIntervention(sessionID, interventionID string) (*Intervention, error) {
	var intervention Intervention
	if err := s.store.LoadDir(collectionSessions, sessionID, subdirInterventions, interventionID, &intervention); err != nil {
		if errors.Is(err, local.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &intervention, nil
}

// ListInterventions returns all intervention IDs for a session
func (s *Store) ListInterventions(sessionID string) ([]string, error) {
	return s.store.ListDir(collectionSessions, sessionID, subdirInterventions)
}

// Exists checks if a session exists
func (s *Store) Exists(id string) bool {
	return s.store.Exists(collectionSessions, id)
}
