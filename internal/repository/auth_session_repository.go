package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// AuthSessionRepository implements domain.AuthSessionRepository using the storage layer
type AuthSessionRepository struct {
	queries *storage.Queries
}

// NewAuthSessionRepository creates a new AuthSessionRepository
func NewAuthSessionRepository(queries *storage.Queries) *AuthSessionRepository {
	return &AuthSessionRepository{queries: queries}
}

// FindByToken retrieves a session by token
func (r *AuthSessionRepository) FindByToken(ctx context.Context, token string) (*domain.AuthSession, error) {
	session, err := r.queries.GetSessionByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAuthSessionNotFound
		}
		return nil, err
	}
	return mapAuthSessionToDomain(session), nil
}

// Save persists a session
func (r *AuthSessionRepository) Save(ctx context.Context, session *domain.AuthSession) error {
	params := mapAuthSessionToStorage(session)
	_, err := r.queries.CreateSession(ctx, params)
	return err
}

// Delete removes a session
func (r *AuthSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteSession(ctx, id)
}

// DeleteByUserID removes all sessions for a user
func (r *AuthSessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.queries.DeleteUserSessions(ctx, userID)
}

// DeleteExpired removes all expired sessions
func (r *AuthSessionRepository) DeleteExpired(ctx context.Context) error {
	return r.queries.DeleteExpiredSessions(ctx)
}

// Ensure AuthSessionRepository implements domain.AuthSessionRepository
var _ domain.AuthSessionRepository = (*AuthSessionRepository)(nil)
