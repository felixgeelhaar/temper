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

// LearningProfileRepository implements domain.LearningProfileRepository using the storage layer
type LearningProfileRepository struct {
	queries *storage.Queries
}

// NewLearningProfileRepository creates a new LearningProfileRepository
func NewLearningProfileRepository(queries *storage.Queries) *LearningProfileRepository {
	return &LearningProfileRepository{queries: queries}
}

// FindByUserID retrieves a profile by user ID
func (r *LearningProfileRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*domain.LearningProfile, error) {
	profile, err := r.queries.GetLearningProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrLearningProfileNotFound
		}
		return nil, err
	}
	return mapLearningProfileToDomain(profile)
}

// Save persists a learning profile (create or update)
func (r *LearningProfileRepository) Save(ctx context.Context, profile *domain.LearningProfile) error {
	topicSkills, err := json.Marshal(profile.TopicSkills)
	if err != nil {
		return err
	}

	_, err = r.queries.UpsertLearningProfile(ctx, storage.UpsertLearningProfileParams{
		UserID:         profile.UserID,
		TopicSkills:    topicSkills,
		TotalExercises: int32(profile.TotalExercises),
		TotalRuns:      int32(profile.TotalRuns),
		HintRequests:   int32(profile.HintRequests),
	})
	return err
}

// Ensure LearningProfileRepository implements domain.LearningProfileRepository
var _ domain.LearningProfileRepository = (*LearningProfileRepository)(nil)
