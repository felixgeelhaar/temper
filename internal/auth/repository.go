package auth

import (
	"context"
	"errors"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// CreateUser inserts a new user
func (r *PostgresRepository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, name, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		user.ID, user.Email, user.Name, user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetUserByEmail retrieves a user by email
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users WHERE email = $1
	`
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users WHERE id = $1
	`
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CreateSession inserts a new session
func (r *PostgresRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query,
		session.ID, session.UserID, session.Token, session.ExpiresAt, session.CreatedAt,
	)
	return err
}

// GetSessionByToken retrieves a session by token
func (r *PostgresRepository) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions WHERE token = $1
	`
	session := &domain.Session{}
	err := r.pool.QueryRow(ctx, query, token).Scan(
		&session.ID, &session.UserID, &session.Token, &session.ExpiresAt, &session.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("session not found")
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSession removes a session
func (r *PostgresRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// DeleteUserSessions removes all sessions for a user
func (r *PostgresRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// DeleteExpiredSessions removes all expired sessions
func (r *PostgresRepository) DeleteExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	_, err := r.pool.Exec(ctx, query)
	return err
}
