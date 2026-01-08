package spec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

var (
	ErrSpecInvalid       = errors.New("spec validation failed")
	ErrCriterionNotFound = errors.New("criterion not found")
)

// Service manages spec lifecycle and operations
type Service struct {
	store     *FileStore
	validator *Validator
}

// NewService creates a new spec service
func NewService(basePath string) *Service {
	return &Service{
		store:     NewFileStore(basePath),
		validator: NewValidator(),
	}
}

// GetWorkspaceRoot returns the workspace root path
func (s *Service) GetWorkspaceRoot() string {
	return s.store.BasePath()
}

// Create creates a new spec scaffold with the given name
func (s *Service) Create(ctx context.Context, name string) (*domain.ProductSpec, error) {
	spec := NewSpecTemplate(name)

	// Ensure .specs/ directory exists
	if err := s.store.EnsureSpecDir(); err != nil {
		return nil, fmt.Errorf("create spec directory: %w", err)
	}

	// Save the spec
	if err := s.store.Save(spec); err != nil {
		return nil, fmt.Errorf("save spec: %w", err)
	}

	return spec, nil
}

// Load reads a spec from the given path
func (s *Service) Load(ctx context.Context, path string) (*domain.ProductSpec, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

// Validate checks a spec for completeness and consistency
func (s *Service) Validate(ctx context.Context, path string) (*domain.SpecValidation, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}

	validation := s.validator.Validate(spec)
	return validation, nil
}

// ValidateSpec validates an in-memory spec
func (s *Service) ValidateSpec(spec *domain.ProductSpec) *domain.SpecValidation {
	return s.validator.Validate(spec)
}

// List returns all specs in the workspace
func (s *Service) List(ctx context.Context) ([]*domain.ProductSpec, error) {
	return s.store.List()
}

// Save persists changes to a spec
func (s *Service) Save(ctx context.Context, spec *domain.ProductSpec) error {
	return s.store.Save(spec)
}

// Delete removes a spec
func (s *Service) Delete(ctx context.Context, path string) error {
	return s.store.Delete(path)
}

// MarkCriterionSatisfied marks an acceptance criterion as satisfied
func (s *Service) MarkCriterionSatisfied(ctx context.Context, path, criterionID, evidence string) error {
	spec, err := s.store.Load(path)
	if err != nil {
		return err
	}

	if !MarkCriterionSatisfied(spec, criterionID, evidence) {
		return ErrCriterionNotFound
	}

	return s.store.Save(spec)
}

// GetProgress returns the completion progress for a spec
func (s *Service) GetProgress(ctx context.Context, path string) (*domain.SpecProgress, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}

	progress := spec.GetProgress()
	return &progress, nil
}

// Lock generates and saves a SpecLock for the spec
func (s *Service) Lock(ctx context.Context, path string) (*domain.SpecLock, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}

	// Validate before locking
	validation := s.validator.Validate(spec)
	if !validation.Valid {
		return nil, fmt.Errorf("%w: %v", ErrSpecInvalid, validation.Errors)
	}

	lock, err := GenerateLock(spec)
	if err != nil {
		return nil, fmt.Errorf("generate lock: %w", err)
	}

	if err := s.store.SaveLock(lock); err != nil {
		return nil, fmt.Errorf("save lock: %w", err)
	}

	return lock, nil
}

// VerifyLock checks if the spec matches its lock
func (s *Service) VerifyLock(ctx context.Context, path string) (bool, []string, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return false, nil, err
	}

	lock, err := s.store.LoadLock()
	if err != nil {
		if errors.Is(err, ErrSpecNotFound) {
			return false, []string{"no lock file found"}, nil
		}
		return false, nil, err
	}

	valid, drifts := VerifyLock(spec, lock)
	return valid, drifts, nil
}

// GetDrift returns detailed drift information
func (s *Service) GetDrift(ctx context.Context, path string) (*DriftReport, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}

	lock, err := s.store.LoadLock()
	if err != nil {
		if errors.Is(err, ErrSpecNotFound) {
			return &DriftReport{
				HasDrift: true,
				AddedFeatures: func() []string {
					ids := make([]string, len(spec.Features))
					for i, f := range spec.Features {
						ids[i] = f.ID
					}
					return ids
				}(),
			}, nil
		}
		return nil, err
	}

	return CalculateDrift(spec, lock), nil
}

// AddFeature adds a new feature to a spec
func (s *Service) AddFeature(ctx context.Context, path, title, description string, priority domain.Priority) error {
	spec, err := s.store.Load(path)
	if err != nil {
		return err
	}

	AddFeature(spec, title, description, priority)
	return s.store.Save(spec)
}

// AddAcceptanceCriterion adds a new acceptance criterion to a spec
func (s *Service) AddAcceptanceCriterion(ctx context.Context, path, description string) error {
	spec, err := s.store.Load(path)
	if err != nil {
		return err
	}

	AddAcceptanceCriterion(spec, description)
	return s.store.Save(spec)
}

// GetNextCriterion returns the next unsatisfied acceptance criterion
func (s *Service) GetNextCriterion(ctx context.Context, path string) (*domain.AcceptanceCriterion, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}

	for i := range spec.AcceptanceCriteria {
		if !spec.AcceptanceCriteria[i].Satisfied {
			return &spec.AcceptanceCriteria[i], nil
		}
	}

	return nil, nil // All criteria satisfied
}

// IsComplete checks if all acceptance criteria are satisfied
func (s *Service) IsComplete(ctx context.Context, path string) (bool, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return false, err
	}

	return spec.IsComplete(), nil
}

// GetFeatureForCriterion attempts to find the feature related to a criterion
func (s *Service) GetFeatureForCriterion(spec *domain.ProductSpec, criterionID string) *domain.Feature {
	// This is a heuristic - we look for features that might be related
	// based on ID prefixes or success criteria
	criterion := spec.GetCriterion(criterionID)
	if criterion == nil {
		return nil
	}

	// For now, return the first high-priority feature
	// In a more sophisticated implementation, this would use NLP or explicit linking
	for i := range spec.Features {
		if spec.Features[i].Priority == domain.PriorityHigh {
			return &spec.Features[i]
		}
	}

	if len(spec.Features) > 0 {
		return &spec.Features[0]
	}

	return nil
}

// UpdateFromLock updates the spec's locked_at timestamp
func (s *Service) UpdateFromLock(ctx context.Context, path string) (*domain.ProductSpec, error) {
	spec, err := s.store.Load(path)
	if err != nil {
		return nil, err
	}

	spec.UpdatedAt = time.Now()
	if err := s.store.Save(spec); err != nil {
		return nil, err
	}

	return spec, nil
}
