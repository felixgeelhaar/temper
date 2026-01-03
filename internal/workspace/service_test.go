package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// mockRepository is a test implementation of Repository
type mockRepository struct {
	artifacts map[uuid.UUID]*domain.Artifact
	versions  map[uuid.UUID][]*domain.ArtifactVersion
	createErr error
	getErr    error
	updateErr error
	deleteErr error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		artifacts: make(map[uuid.UUID]*domain.Artifact),
		versions:  make(map[uuid.UUID][]*domain.ArtifactVersion),
	}
}

func (m *mockRepository) Create(ctx context.Context, artifact *domain.Artifact) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.artifacts[artifact.ID] = artifact
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	a, ok := m.artifacts[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockRepository) GetByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*domain.Artifact, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	a, ok := m.artifacts[id]
	if !ok || a.UserID != userID {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Artifact, error) {
	var result []*domain.Artifact
	for _, a := range m.artifacts {
		if a.UserID == userID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockRepository) Update(ctx context.Context, artifact *domain.Artifact) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.artifacts[artifact.ID] = artifact
	return nil
}

func (m *mockRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.artifacts, id)
	return nil
}

func (m *mockRepository) CreateVersion(ctx context.Context, version *domain.ArtifactVersion) error {
	m.versions[version.ArtifactID] = append(m.versions[version.ArtifactID], version)
	return nil
}

func (m *mockRepository) ListVersions(ctx context.Context, artifactID uuid.UUID, limit int) ([]*domain.ArtifactVersion, error) {
	return m.versions[artifactID], nil
}

func (m *mockRepository) GetVersion(ctx context.Context, artifactID uuid.UUID, version int) (*domain.ArtifactVersion, error) {
	versions := m.versions[artifactID]
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, errors.New("version not found")
}

func (m *mockRepository) CountVersions(ctx context.Context, artifactID uuid.UUID) (int, error) {
	return len(m.versions[artifactID]), nil
}

func TestNewService(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)

	if s == nil {
		t.Fatal("NewService() returned nil")
	}
	if s.repo != repo {
		t.Error("repo not set correctly")
	}
}

func TestService_Create(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	tests := []struct {
		name    string
		req     CreateRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateRequest{
				UserID: userID,
				Name:   "Test Workspace",
				Content: map[string]string{
					"main.go": "package main",
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			req: CreateRequest{
				UserID: userID,
				Name:   "",
			},
			wantErr: true,
		},
		{
			name: "nil content initializes map",
			req: CreateRequest{
				UserID:  userID,
				Name:    "Empty Workspace",
				Content: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := s.Create(ctx, tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if artifact == nil {
					t.Fatal("Create() returned nil artifact")
				}
				if artifact.Name != tt.req.Name {
					t.Errorf("Name = %v, want %v", artifact.Name, tt.req.Name)
				}
				if artifact.UserID != tt.req.UserID {
					t.Error("UserID mismatch")
				}
				if artifact.Content == nil {
					t.Error("Content should not be nil")
				}
			}
		})
	}
}

func TestService_Create_RepositoryError(t *testing.T) {
	repo := newMockRepository()
	repo.createErr = errors.New("database error")
	s := NewService(repo)
	ctx := context.Background()

	_, err := s.Create(ctx, CreateRequest{
		UserID: uuid.New(),
		Name:   "Test",
	})

	if err == nil {
		t.Error("Create() should return error when repo fails")
	}
}

func TestService_Get(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create an artifact
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "Test",
	})

	tests := []struct {
		name    string
		id      uuid.UUID
		userID  uuid.UUID
		wantErr bool
	}{
		{
			name:    "valid get",
			id:      artifact.ID,
			userID:  userID,
			wantErr: false,
		},
		{
			name:    "wrong user",
			id:      artifact.ID,
			userID:  uuid.New(),
			wantErr: true,
		},
		{
			name:    "non-existent",
			id:      uuid.New(),
			userID:  userID,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Get(ctx, tt.id, tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_List(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create some artifacts
	for i := 0; i < 3; i++ {
		s.Create(ctx, CreateRequest{
			UserID: userID,
			Name:   "Test " + string(rune('A'+i)),
		})
	}

	tests := []struct {
		name   string
		limit  int
		offset int
		want   int
	}{
		{"default limit", 0, 0, 3},
		{"custom limit", 2, 0, 3}, // Mock returns all, limit applied elsewhere
		{"excessive limit capped", 200, 0, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.List(ctx, userID, tt.limit, tt.offset)

			if err != nil {
				t.Errorf("List() error = %v", err)
			}

			if len(result) == 0 && tt.want > 0 {
				t.Error("List() returned empty result")
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create an artifact
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID:  userID,
		Name:    "Original",
		Content: map[string]string{"file.go": "original"},
	})

	newName := "Updated"
	newContent := map[string]string{"file.go": "updated"}

	updated, err := s.Update(ctx, UpdateRequest{
		ID:      artifact.ID,
		UserID:  userID,
		Name:    &newName,
		Content: newContent,
	})

	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.Name != newName {
		t.Errorf("Name = %v, want %v", updated.Name, newName)
	}

	if updated.Content["file.go"] != "updated" {
		t.Error("Content not updated")
	}

	if updated.UpdatedAt.Before(artifact.UpdatedAt) {
		t.Error("UpdatedAt should be after creation time")
	}
}

func TestService_Update_NotFound(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()

	name := "Updated"
	_, err := s.Update(ctx, UpdateRequest{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Name:   &name,
	})

	if err != ErrNotFound {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestService_Delete(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create and delete
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "To Delete",
	})

	err := s.Delete(ctx, artifact.ID, userID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err = s.Get(ctx, artifact.ID, userID)
	if err == nil {
		t.Error("Get() should fail after deletion")
	}
}

func TestService_CreateSnapshot(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create an artifact
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID:  userID,
		Name:    "Versioned",
		Content: map[string]string{"file.go": "version 1"},
	})

	// Create snapshot
	version, err := s.CreateSnapshot(ctx, artifact.ID, userID)
	if err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}

	if version.Version != 1 {
		t.Errorf("Version = %d, want 1", version.Version)
	}

	if version.ArtifactID != artifact.ID {
		t.Error("ArtifactID mismatch")
	}

	// Create second snapshot
	version2, err := s.CreateSnapshot(ctx, artifact.ID, userID)
	if err != nil {
		t.Fatalf("CreateSnapshot() second error = %v", err)
	}

	if version2.Version != 2 {
		t.Errorf("Second version = %d, want 2", version2.Version)
	}
}

func TestService_CreateSnapshot_NotFound(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()

	_, err := s.CreateSnapshot(ctx, uuid.New(), uuid.New())
	if err != ErrNotFound {
		t.Errorf("CreateSnapshot() error = %v, want ErrNotFound", err)
	}
}

func TestService_ListVersions(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create artifact and versions
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "Versioned",
	})

	s.CreateSnapshot(ctx, artifact.ID, userID)
	s.CreateSnapshot(ctx, artifact.ID, userID)

	versions, err := s.ListVersions(ctx, artifact.ID, userID, 10)
	if err != nil {
		t.Fatalf("ListVersions() error = %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("ListVersions() returned %d, want 2", len(versions))
	}
}

func TestService_ListVersions_NotFound(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()

	_, err := s.ListVersions(ctx, uuid.New(), uuid.New(), 10)
	if err != ErrNotFound {
		t.Errorf("ListVersions() error = %v, want ErrNotFound", err)
	}
}

func TestService_RestoreVersion(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create artifact with initial content
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID:  userID,
		Name:    "Restorable",
		Content: map[string]string{"file.go": "version 1"},
	})

	// Create snapshot of version 1
	s.CreateSnapshot(ctx, artifact.ID, userID)

	// Update content
	newContent := map[string]string{"file.go": "version 2"}
	s.Update(ctx, UpdateRequest{
		ID:      artifact.ID,
		UserID:  userID,
		Content: newContent,
	})

	// Restore to version 1
	restored, err := s.RestoreVersion(ctx, artifact.ID, userID, 1)
	if err != nil {
		t.Fatalf("RestoreVersion() error = %v", err)
	}

	if restored.Content["file.go"] != "version 1" {
		t.Errorf("Content = %v, want version 1 content", restored.Content["file.go"])
	}
}

func TestService_RestoreVersion_NotFound(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()

	_, err := s.RestoreVersion(ctx, uuid.New(), uuid.New(), 1)
	if err != ErrNotFound {
		t.Errorf("RestoreVersion() error = %v, want ErrNotFound", err)
	}
}

func TestService_RestoreVersion_VersionNotFound(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	artifact, _ := s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "Test",
	})

	_, err := s.RestoreVersion(ctx, artifact.ID, userID, 999)
	if err == nil {
		t.Error("RestoreVersion() should fail for non-existent version")
	}
}

func TestService_CreateFromExercise(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	exercise := &domain.Exercise{
		ID:    "ex-1",
		Title: "Hello World",
		StarterCode: map[string]string{
			"main.go": "package main",
		},
		TestCode: map[string]string{
			"main_test.go": "package main_test",
		},
	}

	artifact, err := s.CreateFromExercise(ctx, userID, exercise)
	if err != nil {
		t.Fatalf("CreateFromExercise() error = %v", err)
	}

	if artifact.Name != exercise.Title {
		t.Errorf("Name = %v, want %v", artifact.Name, exercise.Title)
	}

	if artifact.ExerciseID == nil || *artifact.ExerciseID != exercise.ID {
		t.Error("ExerciseID not set correctly")
	}

	// Check both starter and test code are included
	if _, ok := artifact.Content["main.go"]; !ok {
		t.Error("StarterCode not included")
	}
	if _, ok := artifact.Content["main_test.go"]; !ok {
		t.Error("TestCode not included")
	}
}

func TestErrors(t *testing.T) {
	if ErrNotFound.Error() != "workspace not found" {
		t.Errorf("ErrNotFound = %v", ErrNotFound)
	}
	if ErrAccessDenied.Error() != "access denied" {
		t.Errorf("ErrAccessDenied = %v", ErrAccessDenied)
	}
	if ErrInvalidInput.Error() != "invalid input" {
		t.Errorf("ErrInvalidInput = %v", ErrInvalidInput)
	}
}

func TestCreateRequest_Fields(t *testing.T) {
	userID := uuid.New()
	exerciseID := "ex-1"

	req := CreateRequest{
		UserID:     userID,
		Name:       "Test",
		ExerciseID: &exerciseID,
		Content:    map[string]string{"file.go": "code"},
	}

	if req.UserID != userID {
		t.Error("UserID mismatch")
	}
	if req.Name != "Test" {
		t.Error("Name mismatch")
	}
	if *req.ExerciseID != exerciseID {
		t.Error("ExerciseID mismatch")
	}
}

func TestUpdateRequest_Fields(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	name := "Updated"

	req := UpdateRequest{
		ID:      id,
		UserID:  userID,
		Name:    &name,
		Content: map[string]string{"file.go": "updated"},
	}

	if req.ID != id {
		t.Error("ID mismatch")
	}
	if req.UserID != userID {
		t.Error("UserID mismatch")
	}
	if *req.Name != name {
		t.Error("Name mismatch")
	}
}

func TestService_List_DefaultLimit(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Create artifact
	s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "Test",
	})

	// Test with negative limit (should use default)
	_, err := s.List(ctx, userID, -1, 0)
	if err != nil {
		t.Errorf("List() with negative limit should not error: %v", err)
	}
}

func TestService_ListVersions_DefaultLimit(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	artifact, _ := s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "Test",
	})

	// Test with zero limit (should use default of 10)
	_, err := s.ListVersions(ctx, artifact.ID, userID, 0)
	if err != nil {
		t.Errorf("ListVersions() with zero limit should not error: %v", err)
	}
}

func TestArtifact_Timestamps(t *testing.T) {
	repo := newMockRepository()
	s := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	before := time.Now()
	artifact, _ := s.Create(ctx, CreateRequest{
		UserID: userID,
		Name:   "Test",
	})
	after := time.Now()

	if artifact.CreatedAt.Before(before) || artifact.CreatedAt.After(after) {
		t.Error("CreatedAt should be within test execution time")
	}

	if artifact.UpdatedAt.Before(before) || artifact.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be within test execution time")
	}
}
