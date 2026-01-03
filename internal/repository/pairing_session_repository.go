package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// PairingSessionRepository implements domain.PairingSessionRepository using the storage layer
type PairingSessionRepository struct {
	queries *storage.Queries
}

// NewPairingSessionRepository creates a new PairingSessionRepository
func NewPairingSessionRepository(queries *storage.Queries) *PairingSessionRepository {
	return &PairingSessionRepository{queries: queries}
}

// FindByID retrieves a session by ID
func (r *PairingSessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.PairingSession, error) {
	session, err := r.queries.GetPairingSession(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSessionNotActive
		}
		return nil, err
	}
	return mapPairingSessionToDomain(session)
}

// FindActiveByUserID retrieves the active session for a user
func (r *PairingSessionRepository) FindActiveByUserID(ctx context.Context, userID uuid.UUID) (*domain.PairingSession, error) {
	session, err := r.queries.GetActivePairingSession(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSessionNotActive
		}
		return nil, err
	}
	return mapPairingSessionToDomain(session)
}

// Save persists a session (create or update)
func (r *PairingSessionRepository) Save(ctx context.Context, session *domain.PairingSession) error {
	// Try to get existing session
	_, err := r.queries.GetPairingSession(ctx, session.ID())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create new session
			policy, err := json.Marshal(session.Policy())
			if err != nil {
				return err
			}
			_, err = r.queries.CreatePairingSession(ctx, storage.CreatePairingSessionParams{
				UserID:     session.UserID(),
				ExerciseID: ptrToNullString(session.ExerciseID()),
				Policy:     policy,
			})
			return err
		}
		return err
	}

	// Update existing session - end it if not active
	if !session.IsActive() {
		_, err = r.queries.EndPairingSession(ctx, session.ID())
		return err
	}

	return nil
}

// mapPairingSessionToDomain converts a storage PairingSession to a domain PairingSession
// Note: This creates a minimal session since storage doesn't have all domain state
func mapPairingSessionToDomain(s storage.PairingSession) (*domain.PairingSession, error) {
	var policy domain.LearningPolicy
	if len(s.Policy) > 0 {
		if err := json.Unmarshal(s.Policy, &policy); err != nil {
			return nil, err
		}
	}

	exerciseID := nullStringToPtr(s.ExerciseID)

	// Reconstruct the session based on whether it has an exercise ID
	if exerciseID != nil {
		return domain.NewPairingSession(s.UserID, *exerciseID, nil, policy), nil
	}

	// For greenfield sessions
	return domain.NewGreenfieldPairingSession(s.UserID, nil, policy), nil
}

// Ensure PairingSessionRepository implements domain.PairingSessionRepository
var _ domain.PairingSessionRepository = (*PairingSessionRepository)(nil)
