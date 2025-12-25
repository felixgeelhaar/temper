package workspace

import (
	"context"
	"encoding/json"
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

// Create inserts a new artifact
func (r *PostgresRepository) Create(ctx context.Context, artifact *domain.Artifact) error {
	contentJSON, err := json.Marshal(artifact.Content)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO artifacts (id, user_id, exercise_id, name, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.pool.Exec(ctx, query,
		artifact.ID, artifact.UserID, artifact.ExerciseID, artifact.Name,
		contentJSON, artifact.CreatedAt, artifact.UpdatedAt,
	)
	return err
}

// GetByID retrieves an artifact by ID
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	query := `
		SELECT id, user_id, exercise_id, name, content, created_at, updated_at
		FROM artifacts WHERE id = $1
	`
	return r.scanArtifact(r.pool.QueryRow(ctx, query, id))
}

// GetByIDAndUser retrieves an artifact by ID and user
func (r *PostgresRepository) GetByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*domain.Artifact, error) {
	query := `
		SELECT id, user_id, exercise_id, name, content, created_at, updated_at
		FROM artifacts WHERE id = $1 AND user_id = $2
	`
	return r.scanArtifact(r.pool.QueryRow(ctx, query, id, userID))
}

// ListByUser retrieves all artifacts for a user
func (r *PostgresRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Artifact, error) {
	query := `
		SELECT id, user_id, exercise_id, name, content, created_at, updated_at
		FROM artifacts WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*domain.Artifact
	for rows.Next() {
		artifact, err := r.scanArtifactRow(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}

	return artifacts, rows.Err()
}

// Update updates an artifact
func (r *PostgresRepository) Update(ctx context.Context, artifact *domain.Artifact) error {
	contentJSON, err := json.Marshal(artifact.Content)
	if err != nil {
		return err
	}

	query := `
		UPDATE artifacts SET name = $2, content = $3, updated_at = $4
		WHERE id = $1
	`
	_, err = r.pool.Exec(ctx, query,
		artifact.ID, artifact.Name, contentJSON, artifact.UpdatedAt,
	)
	return err
}

// Delete removes an artifact
func (r *PostgresRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM artifacts WHERE id = $1 AND user_id = $2`
	result, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("artifact not found")
	}
	return nil
}

// CreateVersion inserts a new version
func (r *PostgresRepository) CreateVersion(ctx context.Context, version *domain.ArtifactVersion) error {
	contentJSON, err := json.Marshal(version.Content)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO artifact_versions (id, artifact_id, version, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = r.pool.Exec(ctx, query,
		version.ID, version.ArtifactID, version.Version, contentJSON, version.CreatedAt,
	)
	return err
}

// ListVersions retrieves versions for an artifact
func (r *PostgresRepository) ListVersions(ctx context.Context, artifactID uuid.UUID, limit int) ([]*domain.ArtifactVersion, error) {
	query := `
		SELECT id, artifact_id, version, content, created_at
		FROM artifact_versions WHERE artifact_id = $1
		ORDER BY version DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, artifactID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.ArtifactVersion
	for rows.Next() {
		version, err := r.scanVersionRow(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// GetVersion retrieves a specific version
func (r *PostgresRepository) GetVersion(ctx context.Context, artifactID uuid.UUID, version int) (*domain.ArtifactVersion, error) {
	query := `
		SELECT id, artifact_id, version, content, created_at
		FROM artifact_versions WHERE artifact_id = $1 AND version = $2
	`
	row := r.pool.QueryRow(ctx, query, artifactID, version)

	var v domain.ArtifactVersion
	var contentJSON []byte
	err := row.Scan(&v.ID, &v.ArtifactID, &v.Version, &contentJSON, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
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

// CountVersions counts versions for an artifact
func (r *PostgresRepository) CountVersions(ctx context.Context, artifactID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM artifact_versions WHERE artifact_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, query, artifactID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) scanArtifact(row pgx.Row) (*domain.Artifact, error) {
	var artifact domain.Artifact
	var contentJSON []byte

	err := row.Scan(
		&artifact.ID, &artifact.UserID, &artifact.ExerciseID, &artifact.Name,
		&contentJSON, &artifact.CreatedAt, &artifact.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
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

func (r *PostgresRepository) scanArtifactRow(rows pgx.Rows) (*domain.Artifact, error) {
	var artifact domain.Artifact
	var contentJSON []byte

	err := rows.Scan(
		&artifact.ID, &artifact.UserID, &artifact.ExerciseID, &artifact.Name,
		&contentJSON, &artifact.CreatedAt, &artifact.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &artifact.Content); err != nil {
		return nil, err
	}

	return &artifact, nil
}

func (r *PostgresRepository) scanVersionRow(rows pgx.Rows) (*domain.ArtifactVersion, error) {
	var version domain.ArtifactVersion
	var contentJSON []byte

	err := rows.Scan(&version.ID, &version.ArtifactID, &version.Version, &contentJSON, &version.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &version.Content); err != nil {
		return nil, err
	}

	return &version, nil
}
