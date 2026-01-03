package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// InterventionRepository implements domain.InterventionRepository using the storage layer
type InterventionRepository struct {
	queries *storage.Queries
}

// NewInterventionRepository creates a new InterventionRepository
func NewInterventionRepository(queries *storage.Queries) *InterventionRepository {
	return &InterventionRepository{queries: queries}
}

// FindByID retrieves an intervention by ID
func (r *InterventionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Intervention, error) {
	intervention, err := r.queries.GetInterventionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrInterventionNotFound
		}
		return nil, err
	}
	return mapInterventionToDomain(intervention)
}

// FindLastBySessionID retrieves the most recent intervention for a session
func (r *InterventionRepository) FindLastBySessionID(ctx context.Context, sessionID uuid.UUID) (*domain.Intervention, error) {
	intervention, err := r.queries.GetLastInterventionBySession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrInterventionNotFound
		}
		return nil, err
	}
	return mapInterventionToDomain(intervention)
}

// ListBySession retrieves interventions for a session
// Note: Offset not supported by storage
func (r *InterventionRepository) ListBySession(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]*domain.Intervention, error) {
	// Fetch more to handle offset
	fetchLimit := limit + offset
	interventions, err := r.queries.ListInterventionsBySession(ctx, storage.ListInterventionsBySessionParams{
		SessionID: sessionID,
		Limit:     int32(fetchLimit),
	})
	if err != nil {
		return nil, err
	}

	// Apply offset in-memory
	if offset >= len(interventions) {
		return []*domain.Intervention{}, nil
	}
	interventions = interventions[offset:]
	if len(interventions) > limit {
		interventions = interventions[:limit]
	}

	result := make([]*domain.Intervention, 0, len(interventions))
	for _, i := range interventions {
		intervention, err := mapInterventionToDomain(i)
		if err != nil {
			return nil, err
		}
		result = append(result, intervention)
	}
	return result, nil
}

// ListByUser retrieves interventions for a user
func (r *InterventionRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Intervention, error) {
	interventions, err := r.queries.ListInterventionsByUser(ctx, storage.ListInterventionsByUserParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Intervention, 0, len(interventions))
	for _, i := range interventions {
		intervention, err := mapInterventionToDomain(i)
		if err != nil {
			return nil, err
		}
		result = append(result, intervention)
	}
	return result, nil
}

// Save persists an intervention
func (r *InterventionRepository) Save(ctx context.Context, intervention *domain.Intervention) error {
	params, err := mapInterventionToStorage(intervention)
	if err != nil {
		return err
	}
	_, err = r.queries.CreateIntervention(ctx, params)
	return err
}

// CountBySession returns the number of interventions for a session
func (r *InterventionRepository) CountBySession(ctx context.Context, sessionID uuid.UUID) (int, error) {
	count, err := r.queries.CountInterventionsBySession(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Ensure InterventionRepository implements domain.InterventionRepository
var _ domain.InterventionRepository = (*InterventionRepository)(nil)
