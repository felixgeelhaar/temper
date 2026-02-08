package spec

import (
	"context"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// SpecService defines the interface for spec management operations
// used by the daemon handlers
type SpecService interface {
	// Create creates a new spec scaffold with the given name
	Create(ctx context.Context, name string) (*domain.ProductSpec, error)

	// Load reads a spec from the given path
	Load(ctx context.Context, path string) (*domain.ProductSpec, error)

	// List returns all specs in the workspace
	List(ctx context.Context) ([]*domain.ProductSpec, error)

	// Validate checks a spec for completeness and consistency
	Validate(ctx context.Context, path string) (*domain.SpecValidation, error)

	// MarkCriterionSatisfied marks an acceptance criterion as satisfied
	MarkCriterionSatisfied(ctx context.Context, path, criterionID, evidence string) error

	// Lock generates and saves a SpecLock for the spec
	Lock(ctx context.Context, path string) (*domain.SpecLock, error)

	// GetProgress returns the completion progress for a spec
	GetProgress(ctx context.Context, path string) (*domain.SpecProgress, error)

	// GetDrift returns detailed drift information
	GetDrift(ctx context.Context, path string) (*DriftReport, error)

	// Save persists changes to a spec
	Save(ctx context.Context, spec *domain.ProductSpec) error

	// GetWorkspaceRoot returns the workspace root path
	GetWorkspaceRoot() string
}

// Ensure Service implements SpecService
var _ SpecService = (*Service)(nil)
