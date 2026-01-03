package spec

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func setupTestService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	return NewService(tmpDir)
}

func TestNewService(t *testing.T) {
	service := setupTestService(t)
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_Create(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec, err := service.Create(ctx, "Test Spec")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if spec.Name != "Test Spec" {
		t.Errorf("Name = %q; want %q", spec.Name, "Test Spec")
	}

	if spec.FilePath == "" {
		t.Error("FilePath should be set")
	}
}

func TestService_Load(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create first
	created, _ := service.Create(ctx, "Test Spec")

	// Load it
	loaded, err := service.Load(ctx, created.FilePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != created.Name {
		t.Errorf("Name = %q; want %q", loaded.Name, created.Name)
	}
}

func TestService_Load_NotFound(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	_, err := service.Load(ctx, "nonexistent.yaml")
	if err == nil {
		t.Error("Load() should error for nonexistent spec")
	}
}

func TestService_Validate(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create and save a valid spec
	spec := &domain.ProductSpec{
		Name:     "Valid Spec",
		Version:  "1.0.0",
		FilePath: "valid.yaml",
		Goals:    []string{"Implement user authentication"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Login",
				Description: "User can log in",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"User can log in",
				},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in with valid credentials"},
		},
	}
	service.Save(ctx, spec)

	validation, err := service.Validate(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !validation.Valid {
		t.Errorf("Validation.Valid = false; want true; errors = %v", validation.Errors)
	}
}

func TestService_ValidateSpec(t *testing.T) {
	service := setupTestService(t)

	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Implement user authentication"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Login",
				Description: "User can log in",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"User can log in",
				},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in with valid credentials"},
		},
	}

	validation := service.ValidateSpec(spec)
	if !validation.Valid {
		t.Errorf("ValidateSpec() = invalid; want valid; errors = %v", validation.Errors)
	}
}

func TestService_List(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create some specs
	service.Create(ctx, "Spec 1")
	service.Create(ctx, "Spec 2")

	specs, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(specs) != 2 {
		t.Errorf("List() returned %d specs; want 2", len(specs))
	}
}

func TestService_Save(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "test-save.yaml",
		Goals:    []string{"Goal 1"},
	}

	err := service.Save(ctx, spec)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify by loading
	loaded, err := service.Load(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}

	if loaded.Name != spec.Name {
		t.Errorf("Name = %q; want %q", loaded.Name, spec.Name)
	}
}

func TestService_Delete(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create first
	created, _ := service.Create(ctx, "Test Spec")

	// Delete it
	err := service.Delete(ctx, created.FilePath)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err = service.Load(ctx, created.FilePath)
	if err == nil {
		t.Error("Load() should error after Delete()")
	}
}

func TestService_MarkCriterionSatisfied(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "criteria.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
		},
	}
	service.Save(ctx, spec)

	err := service.MarkCriterionSatisfied(ctx, spec.FilePath, "ac-1", "Tests pass")
	if err != nil {
		t.Fatalf("MarkCriterionSatisfied() error = %v", err)
	}

	// Verify it's marked
	loaded, _ := service.Load(ctx, spec.FilePath)
	if !loaded.AcceptanceCriteria[0].Satisfied {
		t.Error("Criterion should be marked as satisfied")
	}
}

func TestService_MarkCriterionSatisfied_NotFound(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "criteria.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
		},
	}
	service.Save(ctx, spec)

	err := service.MarkCriterionSatisfied(ctx, spec.FilePath, "nonexistent", "Evidence")
	if err != ErrCriterionNotFound {
		t.Errorf("MarkCriterionSatisfied() error = %v; want ErrCriterionNotFound", err)
	}
}

func TestService_GetProgress(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "progress.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
			{ID: "ac-2", Description: "Criterion 2", Satisfied: true},
		},
	}
	service.Save(ctx, spec)

	progress, err := service.GetProgress(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}

	if progress.TotalCriteria != 2 {
		t.Errorf("TotalCriteria = %d; want 2", progress.TotalCriteria)
	}
	if progress.SatisfiedCriteria != 1 {
		t.Errorf("SatisfiedCriteria = %d; want 1", progress.SatisfiedCriteria)
	}
}

func TestService_Lock(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewService(tmpDir)
	ctx := context.Background()

	// Create a valid spec
	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "lockable.yaml",
		Goals:    []string{"Implement user authentication"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Login",
				Description: "User can log in",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"User can log in",
				},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in with valid credentials"},
		},
	}
	service.Save(ctx, spec)

	lock, err := service.Lock(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	if lock.Version != spec.Version {
		t.Errorf("Lock Version = %q; want %q", lock.Version, spec.Version)
	}

	// Verify lock file exists
	lockPath := filepath.Join(tmpDir, SpecDir, LockFile)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("Lock file should be created")
	}
}

func TestService_Lock_InvalidSpec(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create an invalid spec (no goals)
	spec := &domain.ProductSpec{
		Name:     "Invalid Spec",
		Version:  "1.0.0",
		FilePath: "invalid.yaml",
	}
	service.Save(ctx, spec)

	_, err := service.Lock(ctx, spec.FilePath)
	if err == nil {
		t.Error("Lock() should fail for invalid spec")
	}
}

func TestService_VerifyLock(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create and lock a spec
	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "verifiable.yaml",
		Goals:    []string{"Implement user authentication"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Login",
				Description: "User can log in",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"User can log in",
				},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in with valid credentials"},
		},
	}
	service.Save(ctx, spec)
	service.Lock(ctx, spec.FilePath)

	valid, drifts, err := service.VerifyLock(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("VerifyLock() error = %v", err)
	}

	if !valid {
		t.Errorf("VerifyLock() valid = false; want true; drifts = %v", drifts)
	}
}

func TestService_VerifyLock_NoLock(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "no-lock.yaml",
		Goals:    []string{"Goal 1"},
	}
	service.Save(ctx, spec)

	valid, drifts, err := service.VerifyLock(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("VerifyLock() error = %v", err)
	}

	if valid {
		t.Error("VerifyLock() should be invalid when no lock exists")
	}
	if len(drifts) == 0 {
		t.Error("Should have drift message about missing lock")
	}
}

func TestService_GetDrift(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	// Create and lock a spec
	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "drift.yaml",
		Goals:    []string{"Implement user authentication"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Login",
				Description: "User can log in",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"User can log in",
				},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in with valid credentials"},
		},
	}
	service.Save(ctx, spec)
	service.Lock(ctx, spec.FilePath)

	// Modify spec
	spec.Features[0].Title = "Modified Login"
	service.Save(ctx, spec)

	report, err := service.GetDrift(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("GetDrift() error = %v", err)
	}

	if !report.HasDrift {
		t.Error("GetDrift() HasDrift = false; want true")
	}
	if len(report.ModifiedFeatures) == 0 {
		t.Error("Should have modified features")
	}
}

func TestService_GetDrift_NoLock(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "no-lock-drift.yaml",
		Goals:    []string{"Goal 1"},
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}
	service.Save(ctx, spec)

	report, err := service.GetDrift(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("GetDrift() error = %v", err)
	}

	if !report.HasDrift {
		t.Error("GetDrift() should show drift when no lock exists")
	}
	if len(report.AddedFeatures) != 1 {
		t.Errorf("AddedFeatures = %v; want all features listed", report.AddedFeatures)
	}
}

func TestService_AddFeature(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "add-feature.yaml",
		Goals:    []string{"Goal 1"},
	}
	service.Save(ctx, spec)

	err := service.AddFeature(ctx, spec.FilePath, "New Feature", "Description", domain.PriorityHigh)
	if err != nil {
		t.Fatalf("AddFeature() error = %v", err)
	}

	loaded, _ := service.Load(ctx, spec.FilePath)
	if len(loaded.Features) != 1 {
		t.Errorf("Features count = %d; want 1", len(loaded.Features))
	}
	if loaded.Features[0].Title != "New Feature" {
		t.Errorf("Feature title = %q; want %q", loaded.Features[0].Title, "New Feature")
	}
}

func TestService_AddAcceptanceCriterion(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "add-criterion.yaml",
		Goals:    []string{"Goal 1"},
	}
	service.Save(ctx, spec)

	err := service.AddAcceptanceCriterion(ctx, spec.FilePath, "User can log in")
	if err != nil {
		t.Fatalf("AddAcceptanceCriterion() error = %v", err)
	}

	loaded, _ := service.Load(ctx, spec.FilePath)
	if len(loaded.AcceptanceCriteria) != 1 {
		t.Errorf("AcceptanceCriteria count = %d; want 1", len(loaded.AcceptanceCriteria))
	}
}

func TestService_GetNextCriterion(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "next-criterion.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1", Satisfied: true},
			{ID: "ac-2", Description: "Criterion 2"},
		},
	}
	service.Save(ctx, spec)

	criterion, err := service.GetNextCriterion(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("GetNextCriterion() error = %v", err)
	}

	if criterion.ID != "ac-2" {
		t.Errorf("NextCriterion ID = %q; want %q", criterion.ID, "ac-2")
	}
}

func TestService_GetNextCriterion_AllSatisfied(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "all-satisfied.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1", Satisfied: true},
		},
	}
	service.Save(ctx, spec)

	criterion, err := service.GetNextCriterion(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("GetNextCriterion() error = %v", err)
	}

	if criterion != nil {
		t.Errorf("GetNextCriterion() = %v; want nil when all satisfied", criterion)
	}
}

func TestService_IsComplete(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "complete.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1", Satisfied: true},
		},
	}
	service.Save(ctx, spec)

	complete, err := service.IsComplete(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("IsComplete() error = %v", err)
	}

	if !complete {
		t.Error("IsComplete() = false; want true")
	}
}

func TestService_IsComplete_NotComplete(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "incomplete.yaml",
		Goals:    []string{"Goal 1"},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
		},
	}
	service.Save(ctx, spec)

	complete, err := service.IsComplete(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("IsComplete() error = %v", err)
	}

	if complete {
		t.Error("IsComplete() = true; want false")
	}
}

func TestService_GetFeatureForCriterion(t *testing.T) {
	service := setupTestService(t)

	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Low Priority", Priority: domain.PriorityLow},
			{ID: "feat-2", Title: "High Priority", Priority: domain.PriorityHigh},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
		},
	}

	feature := service.GetFeatureForCriterion(spec, "ac-1")
	if feature == nil {
		t.Fatal("GetFeatureForCriterion() returned nil")
	}

	// Should return first high-priority feature
	if feature.ID != "feat-2" {
		t.Errorf("Feature ID = %q; want %q (first high-priority)", feature.ID, "feat-2")
	}
}

func TestService_GetFeatureForCriterion_NoHighPriority(t *testing.T) {
	service := setupTestService(t)

	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Low Priority", Priority: domain.PriorityLow},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
		},
	}

	feature := service.GetFeatureForCriterion(spec, "ac-1")
	if feature == nil {
		t.Fatal("GetFeatureForCriterion() returned nil")
	}

	// Should return first feature when no high-priority
	if feature.ID != "feat-1" {
		t.Errorf("Feature ID = %q; want %q", feature.ID, "feat-1")
	}
}

func TestService_GetFeatureForCriterion_CriterionNotFound(t *testing.T) {
	service := setupTestService(t)

	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
		},
	}

	feature := service.GetFeatureForCriterion(spec, "nonexistent")
	if feature != nil {
		t.Errorf("GetFeatureForCriterion() = %v; want nil for nonexistent criterion", feature)
	}
}

func TestService_UpdateFromLock(t *testing.T) {
	service := setupTestService(t)
	ctx := context.Background()

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "update-lock.yaml",
		Goals:    []string{"Goal 1"},
	}
	service.Save(ctx, spec)

	updated, err := service.UpdateFromLock(ctx, spec.FilePath)
	if err != nil {
		t.Fatalf("UpdateFromLock() error = %v", err)
	}

	if updated.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}
