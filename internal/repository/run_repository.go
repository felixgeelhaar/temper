package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// RunRepository implements domain.RunRepository using the storage layer
type RunRepository struct {
	queries *storage.Queries
}

// NewRunRepository creates a new RunRepository
func NewRunRepository(queries *storage.Queries) *RunRepository {
	return &RunRepository{queries: queries}
}

// FindByID retrieves a run by ID
func (r *RunRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	run, err := r.queries.GetRunByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRunNotFound
		}
		return nil, err
	}
	return mapRunToDomain(run)
}

// FindByIDAndUser retrieves a run by ID and user
func (r *RunRepository) FindByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*domain.Run, error) {
	run, err := r.queries.GetRunByIDAndUser(ctx, storage.GetRunByIDAndUserParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRunNotFound
		}
		return nil, err
	}
	return mapRunToDomain(run)
}

// ListByUser retrieves runs for a user
func (r *RunRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Run, error) {
	runs, err := r.queries.ListRunsByUser(ctx, storage.ListRunsByUserParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Run, 0, len(runs))
	for _, run := range runs {
		domainRun, err := mapRunToDomain(run)
		if err != nil {
			return nil, err
		}
		result = append(result, domainRun)
	}
	return result, nil
}

// ListByArtifact retrieves runs for an artifact
// Note: Offset not supported by storage
func (r *RunRepository) ListByArtifact(ctx context.Context, artifactID uuid.UUID, limit, offset int) ([]*domain.Run, error) {
	// Fetch more to handle offset
	fetchLimit := limit + offset
	runs, err := r.queries.ListRunsByArtifact(ctx, storage.ListRunsByArtifactParams{
		ArtifactID: artifactID,
		Limit:      int32(fetchLimit),
	})
	if err != nil {
		return nil, err
	}

	// Apply offset in-memory
	if offset >= len(runs) {
		return []*domain.Run{}, nil
	}
	runs = runs[offset:]
	if len(runs) > limit {
		runs = runs[:limit]
	}

	result := make([]*domain.Run, 0, len(runs))
	for _, run := range runs {
		domainRun, err := mapRunToDomain(run)
		if err != nil {
			return nil, err
		}
		result = append(result, domainRun)
	}
	return result, nil
}

// ListPending retrieves pending runs
func (r *RunRepository) ListPending(ctx context.Context, limit int) ([]*domain.Run, error) {
	runs, err := r.queries.ListPendingRuns(ctx, int32(limit))
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Run, 0, len(runs))
	for _, run := range runs {
		domainRun, err := mapRunToDomain(run)
		if err != nil {
			return nil, err
		}
		result = append(result, domainRun)
	}
	return result, nil
}

// Save persists a run
func (r *RunRepository) Save(ctx context.Context, run *domain.Run) error {
	// Try to get existing run
	_, err := r.queries.GetRunByID(ctx, run.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create new run
			params, err := mapRunToStorage(run)
			if err != nil {
				return err
			}
			_, err = r.queries.CreateRun(ctx, params)
			return err
		}
		return err
	}

	// Update existing run - update status and output
	if run.Output != nil {
		output, err := json.Marshal(run.Output)
		if err != nil {
			return err
		}
		_, err = r.queries.UpdateRunOutput(ctx, storage.UpdateRunOutputParams{
			ID:     run.ID,
			Status: run.Status.String(),
			Output: pqtype.NullRawMessage{RawMessage: output, Valid: true},
		})
		return err
	}

	_, err = r.queries.UpdateRunStatus(ctx, storage.UpdateRunStatusParams{
		ID:        run.ID,
		Status:    run.Status.String(),
		StartedAt: ptrToNullTime(run.StartedAt),
	})
	return err
}

// CountByUser returns the total number of runs for a user
func (r *RunRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := r.queries.CountRunsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// CountSuccessfulByUser returns the number of successful runs for a user
func (r *RunRepository) CountSuccessfulByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := r.queries.CountSuccessfulRunsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Ensure RunRepository implements domain.RunRepository
var _ domain.RunRepository = (*RunRepository)(nil)
