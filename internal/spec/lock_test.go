package spec

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestGenerateLock(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Goal 1"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Feature 1",
				Description: "Test feature",
			},
			{
				ID:    "feat-2",
				Title: "Feature 2",
				API: &domain.APISpec{
					Path:   "/api/v1/test",
					Method: "GET",
				},
			},
		},
	}

	lock, err := GenerateLock(spec)
	if err != nil {
		t.Fatalf("GenerateLock() error = %v", err)
	}

	if lock.Version != spec.Version {
		t.Errorf("Version = %q; want %q", lock.Version, spec.Version)
	}

	if lock.SpecHash == "" {
		t.Error("SpecHash should not be empty")
	}

	if len(lock.Features) != 2 {
		t.Errorf("Features count = %d; want 2", len(lock.Features))
	}

	// Check feature with API
	feat2 := lock.Features["feat-2"]
	if feat2.APIPath != "/api/v1/test" {
		t.Errorf("APIPath = %q; want %q", feat2.APIPath, "/api/v1/test")
	}

	if lock.LockedAt.IsZero() {
		t.Error("LockedAt should be set")
	}
}

func TestVerifyLock_Valid(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Goal 1"},
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	lock, _ := GenerateLock(spec)

	valid, drifts := VerifyLock(spec, lock)
	if !valid {
		t.Errorf("VerifyLock() valid = false; want true; drifts = %v", drifts)
	}
	if len(drifts) != 0 {
		t.Errorf("VerifyLock() drifts = %v; want empty", drifts)
	}
}

func TestVerifyLock_VersionChanged(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Goal 1"},
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	lock, _ := GenerateLock(spec)

	// Change version
	spec.Version = "2.0.0"

	valid, drifts := VerifyLock(spec, lock)
	if valid {
		t.Error("VerifyLock() should detect version change")
	}
	if len(drifts) == 0 {
		t.Error("VerifyLock() should have drifts for version change")
	}
}

func TestVerifyLock_FeatureRemoved(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
			{ID: "feat-2", Title: "Feature 2"},
		},
	}

	lock, _ := GenerateLock(spec)

	// Remove a feature
	spec.Features = spec.Features[:1]

	valid, drifts := VerifyLock(spec, lock)
	if valid {
		t.Error("VerifyLock() should detect removed feature")
	}

	found := false
	for _, d := range drifts {
		if d == "feature removed: feat-2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Drifts should mention removed feature; got %v", drifts)
	}
}

func TestVerifyLock_FeatureAdded(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	lock, _ := GenerateLock(spec)

	// Add a feature
	spec.Features = append(spec.Features, domain.Feature{ID: "feat-new", Title: "New Feature"})

	valid, drifts := VerifyLock(spec, lock)
	if valid {
		t.Error("VerifyLock() should detect added feature")
	}

	found := false
	for _, d := range drifts {
		if d == "feature added: feat-new" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Drifts should mention added feature; got %v", drifts)
	}
}

func TestVerifyLock_FeatureModified(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	lock, _ := GenerateLock(spec)

	// Modify a feature
	spec.Features[0].Title = "Modified Feature"

	valid, drifts := VerifyLock(spec, lock)
	if valid {
		t.Error("VerifyLock() should detect modified feature")
	}

	found := false
	for _, d := range drifts {
		if d == "feature modified: feat-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Drifts should mention modified feature; got %v", drifts)
	}
}

func TestCalculateDrift_NoDrift(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	lock, _ := GenerateLock(spec)

	report := CalculateDrift(spec, lock)
	if report.HasDrift {
		t.Error("CalculateDrift() HasDrift = true; want false")
	}
	if len(report.AddedFeatures) != 0 {
		t.Errorf("AddedFeatures = %v; want empty", report.AddedFeatures)
	}
	if len(report.RemovedFeatures) != 0 {
		t.Errorf("RemovedFeatures = %v; want empty", report.RemovedFeatures)
	}
	if len(report.ModifiedFeatures) != 0 {
		t.Errorf("ModifiedFeatures = %v; want empty", report.ModifiedFeatures)
	}
}

func TestCalculateDrift_VersionChanged(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	lock, _ := GenerateLock(spec)
	spec.Version = "2.0.0"

	report := CalculateDrift(spec, lock)
	if !report.HasDrift {
		t.Error("CalculateDrift() HasDrift = false; want true")
	}
	if !report.VersionChanged {
		t.Error("VersionChanged should be true")
	}
	if report.OldVersion != "1.0.0" {
		t.Errorf("OldVersion = %q; want %q", report.OldVersion, "1.0.0")
	}
	if report.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q; want %q", report.NewVersion, "2.0.0")
	}
}

func TestCalculateDrift_AllDriftTypes(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
			{ID: "feat-2", Title: "Feature 2"},
		},
	}

	lock, _ := GenerateLock(spec)

	// Add, remove, and modify features
	spec.Features = []domain.Feature{
		{ID: "feat-1", Title: "Modified Feature 1"}, // Modified
		{ID: "feat-3", Title: "New Feature 3"},      // Added (feat-2 removed)
	}

	report := CalculateDrift(spec, lock)
	if !report.HasDrift {
		t.Error("CalculateDrift() HasDrift = false; want true")
	}

	if len(report.AddedFeatures) != 1 || report.AddedFeatures[0] != "feat-3" {
		t.Errorf("AddedFeatures = %v; want [feat-3]", report.AddedFeatures)
	}
	if len(report.RemovedFeatures) != 1 || report.RemovedFeatures[0] != "feat-2" {
		t.Errorf("RemovedFeatures = %v; want [feat-2]", report.RemovedFeatures)
	}
	if len(report.ModifiedFeatures) != 1 || report.ModifiedFeatures[0] != "feat-1" {
		t.Errorf("ModifiedFeatures = %v; want [feat-1]", report.ModifiedFeatures)
	}
}
