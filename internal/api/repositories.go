package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/felixgeelhaar/temper/internal/auth"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/workspace"
	"github.com/google/uuid"
)

// AuthRepository implements auth.Repository using SQL
type AuthRepository struct {
	db *sql.DB
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

var _ auth.Repository = (*AuthRepository)(nil)

func (r *AuthRepository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, name, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Email, user.Name, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, email, name, password_hash, created_at, updated_at FROM users WHERE email = $1`
	row := r.db.QueryRowContext(ctx, query, email)

	var user domain.User
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, email, name, password_hash, created_at, updated_at FROM users WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var user domain.User
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.Token, session.ExpiresAt, session.CreatedAt)
	return err
}

func (r *AuthRepository) GetSessionByToken(ctx context.Context, token string) (*domain.Session, error) {
	query := `SELECT id, user_id, token, expires_at, created_at FROM sessions WHERE token = $1`
	row := r.db.QueryRowContext(ctx, query, token)

	var session domain.Session
	err := row.Scan(&session.ID, &session.UserID, &session.Token, &session.ExpiresAt, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("session not found")
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *AuthRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *AuthRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *AuthRepository) DeleteExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < $1`
	_, err := r.db.ExecContext(ctx, query, time.Now())
	return err
}

// WorkspaceRepository implements workspace.Repository using SQL
type WorkspaceRepository struct {
	db *sql.DB
}

// NewWorkspaceRepository creates a new workspace repository
func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

var _ workspace.Repository = (*WorkspaceRepository)(nil)

func (r *WorkspaceRepository) Create(ctx context.Context, artifact *domain.Artifact) error {
	contentJSON, err := json.Marshal(artifact.Content)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO artifacts (id, user_id, exercise_id, name, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.db.ExecContext(ctx, query,
		artifact.ID, artifact.UserID, artifact.ExerciseID, artifact.Name,
		contentJSON, artifact.CreatedAt, artifact.UpdatedAt)
	return err
}

func (r *WorkspaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	query := `
		SELECT id, user_id, exercise_id, name, content, created_at, updated_at
		FROM artifacts WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var artifact domain.Artifact
	var contentJSON []byte
	err := row.Scan(
		&artifact.ID, &artifact.UserID, &artifact.ExerciseID, &artifact.Name,
		&contentJSON, &artifact.CreatedAt, &artifact.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("artifact not found")
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &artifact.Content); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *WorkspaceRepository) GetByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*domain.Artifact, error) {
	query := `
		SELECT id, user_id, exercise_id, name, content, created_at, updated_at
		FROM artifacts WHERE id = $1 AND user_id = $2
	`
	row := r.db.QueryRowContext(ctx, query, id, userID)

	var artifact domain.Artifact
	var contentJSON []byte
	err := row.Scan(
		&artifact.ID, &artifact.UserID, &artifact.ExerciseID, &artifact.Name,
		&contentJSON, &artifact.CreatedAt, &artifact.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("artifact not found")
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &artifact.Content); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (r *WorkspaceRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Artifact, error) {
	query := `
		SELECT id, user_id, exercise_id, name, content, created_at, updated_at
		FROM artifacts WHERE user_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*domain.Artifact
	for rows.Next() {
		var artifact domain.Artifact
		var contentJSON []byte
		if err := rows.Scan(
			&artifact.ID, &artifact.UserID, &artifact.ExerciseID, &artifact.Name,
			&contentJSON, &artifact.CreatedAt, &artifact.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(contentJSON, &artifact.Content); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, &artifact)
	}
	return artifacts, rows.Err()
}

func (r *WorkspaceRepository) Update(ctx context.Context, artifact *domain.Artifact) error {
	contentJSON, err := json.Marshal(artifact.Content)
	if err != nil {
		return err
	}

	query := `
		UPDATE artifacts
		SET name = $1, content = $2, updated_at = $3
		WHERE id = $4
	`
	result, err := r.db.ExecContext(ctx, query,
		artifact.Name, contentJSON, artifact.UpdatedAt, artifact.ID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("artifact not found")
	}
	return nil
}

func (r *WorkspaceRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM artifacts WHERE id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, id, userID)
	return err
}

func (r *WorkspaceRepository) CreateVersion(ctx context.Context, version *domain.ArtifactVersion) error {
	contentJSON, err := json.Marshal(version.Content)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO artifact_versions (id, artifact_id, version, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = r.db.ExecContext(ctx, query,
		version.ID, version.ArtifactID, version.Version, contentJSON, version.CreatedAt)
	return err
}

func (r *WorkspaceRepository) ListVersions(ctx context.Context, artifactID uuid.UUID, limit int) ([]*domain.ArtifactVersion, error) {
	query := `
		SELECT id, artifact_id, version, content, created_at
		FROM artifact_versions WHERE artifact_id = $1 ORDER BY version DESC LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, artifactID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.ArtifactVersion
	for rows.Next() {
		var version domain.ArtifactVersion
		var contentJSON []byte
		if err := rows.Scan(
			&version.ID, &version.ArtifactID, &version.Version,
			&contentJSON, &version.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(contentJSON, &version.Content); err != nil {
			return nil, err
		}
		versions = append(versions, &version)
	}
	return versions, rows.Err()
}

func (r *WorkspaceRepository) GetVersion(ctx context.Context, artifactID uuid.UUID, version int) (*domain.ArtifactVersion, error) {
	query := `
		SELECT id, artifact_id, version, content, created_at
		FROM artifact_versions WHERE artifact_id = $1 AND version = $2
	`
	row := r.db.QueryRowContext(ctx, query, artifactID, version)

	var v domain.ArtifactVersion
	var contentJSON []byte
	err := row.Scan(&v.ID, &v.ArtifactID, &v.Version, &contentJSON, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("version not found")
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &v.Content); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *WorkspaceRepository) CountVersions(ctx context.Context, artifactID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM artifact_versions WHERE artifact_id = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, artifactID).Scan(&count)
	return count, err
}
