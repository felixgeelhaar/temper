package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// UserRepository implements domain.UserRepository using the storage layer
type UserRepository struct {
	queries *storage.Queries
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(queries *storage.Queries) *UserRepository {
	return &UserRepository{queries: queries}
}

// FindByID retrieves a user by their ID
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return mapUserToDomain(user), nil
}

// FindByEmail retrieves a user by their email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return mapUserToDomain(user), nil
}

// Save persists a user (create or update)
func (r *UserRepository) Save(ctx context.Context, user *domain.User) error {
	// Try to get existing user
	_, err := r.queries.GetUserByID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create new user
			params := mapUserToStorage(user)
			_, err = r.queries.CreateUser(ctx, params)
			return err
		}
		return err
	}

	// Update existing user (note: storage only allows updating name and email)
	_, err = r.queries.UpdateUser(ctx, storage.UpdateUserParams{
		ID:    user.ID,
		Email: stringToNullString(user.Email),
		Name:  stringToNullString(user.Name),
	})
	return err
}

// Delete removes a user
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteUser(ctx, id)
}

// Ensure UserRepository implements domain.UserRepository
var _ domain.UserRepository = (*UserRepository)(nil)
