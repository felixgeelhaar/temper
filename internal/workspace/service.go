package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("workspace not found")
	ErrAccessDenied  = errors.New("access denied")
	ErrInvalidInput  = errors.New("invalid input")
)

// Repository defines the interface for workspace data access
type Repository interface {
	Create(ctx context.Context, artifact *domain.Artifact) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error)
	GetByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*domain.Artifact, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Artifact, error)
	Update(ctx context.Context, artifact *domain.Artifact) error
	Delete(ctx context.Context, id, userID uuid.UUID) error

	CreateVersion(ctx context.Context, version *domain.ArtifactVersion) error
	ListVersions(ctx context.Context, artifactID uuid.UUID, limit int) ([]*domain.ArtifactVersion, error)
	GetVersion(ctx context.Context, artifactID uuid.UUID, version int) (*domain.ArtifactVersion, error)
	CountVersions(ctx context.Context, artifactID uuid.UUID) (int, error)
}

// Service handles workspace operations
type Service struct {
	repo Repository
}

// NewService creates a new workspace service
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateRequest contains data for creating a workspace
type CreateRequest struct {
	UserID     uuid.UUID
	Name       string
	ExerciseID *string
	Content    map[string]string
}

// Create creates a new workspace
func (s *Service) Create(ctx context.Context, req CreateRequest) (*domain.Artifact, error) {
	if req.Name == "" {
		return nil, ErrInvalidInput
	}

	artifact := &domain.Artifact{
		ID:         uuid.New(),
		UserID:     req.UserID,
		ExerciseID: req.ExerciseID,
		Name:       req.Name,
		Content:    req.Content,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if artifact.Content == nil {
		artifact.Content = make(map[string]string)
	}

	if err := s.repo.Create(ctx, artifact); err != nil {
		return nil, err
	}

	return artifact, nil
}

// Get retrieves a workspace by ID
func (s *Service) Get(ctx context.Context, id, userID uuid.UUID) (*domain.Artifact, error) {
	artifact, err := s.repo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return artifact, nil
}

// List returns all workspaces for a user
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Artifact, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListByUser(ctx, userID, limit, offset)
}

// UpdateRequest contains data for updating a workspace
type UpdateRequest struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Name    *string
	Content map[string]string
}

// Update updates a workspace
func (s *Service) Update(ctx context.Context, req UpdateRequest) (*domain.Artifact, error) {
	artifact, err := s.repo.GetByIDAndUser(ctx, req.ID, req.UserID)
	if err != nil {
		return nil, ErrNotFound
	}

	if req.Name != nil {
		artifact.Name = *req.Name
	}
	if req.Content != nil {
		artifact.Content = req.Content
	}
	artifact.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, artifact); err != nil {
		return nil, err
	}

	return artifact, nil
}

// Delete removes a workspace
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.Delete(ctx, id, userID)
}

// CreateSnapshot creates a version snapshot
func (s *Service) CreateSnapshot(ctx context.Context, artifactID, userID uuid.UUID) (*domain.ArtifactVersion, error) {
	artifact, err := s.repo.GetByIDAndUser(ctx, artifactID, userID)
	if err != nil {
		return nil, ErrNotFound
	}

	// Get next version number
	count, err := s.repo.CountVersions(ctx, artifactID)
	if err != nil {
		return nil, err
	}

	version := &domain.ArtifactVersion{
		ID:         uuid.New(),
		ArtifactID: artifactID,
		Version:    count + 1,
		Content:    artifact.Content,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.CreateVersion(ctx, version); err != nil {
		return nil, err
	}

	return version, nil
}

// ListVersions returns version history for a workspace
func (s *Service) ListVersions(ctx context.Context, artifactID, userID uuid.UUID, limit int) ([]*domain.ArtifactVersion, error) {
	// Verify ownership
	_, err := s.repo.GetByIDAndUser(ctx, artifactID, userID)
	if err != nil {
		return nil, ErrNotFound
	}

	if limit <= 0 {
		limit = 10
	}

	return s.repo.ListVersions(ctx, artifactID, limit)
}

// RestoreVersion restores a workspace to a specific version
func (s *Service) RestoreVersion(ctx context.Context, artifactID, userID uuid.UUID, versionNum int) (*domain.Artifact, error) {
	artifact, err := s.repo.GetByIDAndUser(ctx, artifactID, userID)
	if err != nil {
		return nil, ErrNotFound
	}

	version, err := s.repo.GetVersion(ctx, artifactID, versionNum)
	if err != nil {
		return nil, errors.New("version not found")
	}

	artifact.Content = version.Content
	artifact.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, artifact); err != nil {
		return nil, err
	}

	return artifact, nil
}

// CreateFromExercise creates a workspace from an exercise
func (s *Service) CreateFromExercise(ctx context.Context, userID uuid.UUID, exercise *domain.Exercise) (*domain.Artifact, error) {
	// Combine starter and test code
	content := make(map[string]string)
	for k, v := range exercise.StarterCode {
		content[k] = v
	}
	for k, v := range exercise.TestCode {
		content[k] = v
	}

	return s.Create(ctx, CreateRequest{
		UserID:     userID,
		Name:       exercise.Title,
		ExerciseID: &exercise.ID,
		Content:    content,
	})
}
