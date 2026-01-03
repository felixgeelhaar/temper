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

// ArtifactRepository implements domain.ArtifactRepository using the storage layer
type ArtifactRepository struct {
	queries *storage.Queries
}

// NewArtifactRepository creates a new ArtifactRepository
func NewArtifactRepository(queries *storage.Queries) *ArtifactRepository {
	return &ArtifactRepository{queries: queries}
}

// FindByID retrieves an artifact by ID
func (r *ArtifactRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	artifact, err := r.queries.GetArtifactByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrArtifactNotFound
		}
		return nil, err
	}
	return mapArtifactToDomain(artifact)
}

// FindByIDAndUser retrieves an artifact by ID and user
func (r *ArtifactRepository) FindByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*domain.Artifact, error) {
	artifact, err := r.queries.GetArtifactByIDAndUser(ctx, storage.GetArtifactByIDAndUserParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrArtifactNotFound
		}
		return nil, err
	}
	return mapArtifactToDomain(artifact)
}

// ListByUser retrieves artifacts for a user
func (r *ArtifactRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Artifact, error) {
	artifacts, err := r.queries.ListArtifactsByUser(ctx, storage.ListArtifactsByUserParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Artifact, 0, len(artifacts))
	for _, a := range artifacts {
		artifact, err := mapArtifactToDomain(a)
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	return result, nil
}

// ListByExercise retrieves artifacts for an exercise
// Note: Storage layer doesn't support pagination for this query
func (r *ArtifactRepository) ListByExercise(ctx context.Context, userID uuid.UUID, exerciseID string, limit, offset int) ([]*domain.Artifact, error) {
	artifacts, err := r.queries.ListArtifactsByExercise(ctx, storage.ListArtifactsByExerciseParams{
		UserID:     userID,
		ExerciseID: ptrToNullString(&exerciseID),
	})
	if err != nil {
		return nil, err
	}

	// Apply pagination in-memory (storage doesn't support it)
	start := offset
	if start >= len(artifacts) {
		return []*domain.Artifact{}, nil
	}
	end := start + limit
	if end > len(artifacts) {
		end = len(artifacts)
	}
	artifacts = artifacts[start:end]

	result := make([]*domain.Artifact, 0, len(artifacts))
	for _, a := range artifacts {
		artifact, err := mapArtifactToDomain(a)
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	return result, nil
}

// Save persists an artifact
func (r *ArtifactRepository) Save(ctx context.Context, artifact *domain.Artifact) error {
	// Try to get existing artifact
	_, err := r.queries.GetArtifactByID(ctx, artifact.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create new artifact
			params, err := mapArtifactToStorage(artifact)
			if err != nil {
				return err
			}
			_, err = r.queries.CreateArtifact(ctx, params)
			return err
		}
		return err
	}

	// Update existing artifact
	content, err := json.Marshal(artifact.Content)
	if err != nil {
		return err
	}
	_, err = r.queries.UpdateArtifactContent(ctx, storage.UpdateArtifactContentParams{
		ID:      artifact.ID,
		Content: content,
	})
	return err
}

// Delete removes an artifact
func (r *ArtifactRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.queries.DeleteArtifact(ctx, storage.DeleteArtifactParams{
		ID:     id,
		UserID: userID,
	})
}

// SaveVersion creates a new version of an artifact
func (r *ArtifactRepository) SaveVersion(ctx context.Context, version *domain.ArtifactVersion) error {
	content, err := json.Marshal(version.Content)
	if err != nil {
		return err
	}
	created, err := r.queries.CreateArtifactVersion(ctx, storage.CreateArtifactVersionParams{
		ArtifactID: version.ArtifactID,
		Version:    int32(version.Version),
		Content:    content,
	})
	if err != nil {
		return err
	}
	// Update the version with the generated ID
	version.ID = created.ID
	return nil
}

// FindLatestVersion retrieves the latest version of an artifact
func (r *ArtifactRepository) FindLatestVersion(ctx context.Context, artifactID uuid.UUID) (*domain.ArtifactVersion, error) {
	version, err := r.queries.GetLatestArtifactVersion(ctx, artifactID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrArtifactVersionNotFound
		}
		return nil, err
	}
	return mapArtifactVersionToDomain(version)
}

// ListVersions retrieves versions for an artifact
// Note: Offset not supported by storage, using limit only
func (r *ArtifactRepository) ListVersions(ctx context.Context, artifactID uuid.UUID, limit, offset int) ([]*domain.ArtifactVersion, error) {
	// Fetch more to handle offset
	fetchLimit := limit + offset
	versions, err := r.queries.ListArtifactVersions(ctx, storage.ListArtifactVersionsParams{
		ArtifactID: artifactID,
		Limit:      int32(fetchLimit),
	})
	if err != nil {
		return nil, err
	}

	// Apply offset in-memory
	if offset >= len(versions) {
		return []*domain.ArtifactVersion{}, nil
	}
	versions = versions[offset:]
	if len(versions) > limit {
		versions = versions[:limit]
	}

	result := make([]*domain.ArtifactVersion, 0, len(versions))
	for _, v := range versions {
		version, err := mapArtifactVersionToDomain(v)
		if err != nil {
			return nil, err
		}
		result = append(result, version)
	}
	return result, nil
}

// Ensure ArtifactRepository implements domain.ArtifactRepository
var _ domain.ArtifactRepository = (*ArtifactRepository)(nil)
