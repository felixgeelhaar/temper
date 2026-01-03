package domain

import (
	"math"
	"testing"
	"time"
)

func TestProductSpec_GetProgress(t *testing.T) {
	tests := []struct {
		name     string
		criteria []AcceptanceCriterion
		wantPct  float64
		wantSat  int
		wantPend int
	}{
		{
			name:     "no criteria",
			criteria: []AcceptanceCriterion{},
			wantPct:  0,
			wantSat:  0,
			wantPend: 0,
		},
		{
			name: "all satisfied",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: true},
				{ID: "AC2", Satisfied: true},
			},
			wantPct:  100,
			wantSat:  2,
			wantPend: 0,
		},
		{
			name: "none satisfied",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: false},
				{ID: "AC2", Satisfied: false},
			},
			wantPct:  0,
			wantSat:  0,
			wantPend: 2,
		},
		{
			name: "partial",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: true},
				{ID: "AC2", Satisfied: false},
				{ID: "AC3", Satisfied: true},
				{ID: "AC4", Satisfied: false},
			},
			wantPct:  50,
			wantSat:  2,
			wantPend: 2,
		},
		{
			name: "one of three",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: true},
				{ID: "AC2", Satisfied: false},
				{ID: "AC3", Satisfied: false},
			},
			wantPct:  100.0 / 3.0, // ~33.333...
			wantSat:  1,
			wantPend: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &ProductSpec{
				AcceptanceCriteria: tt.criteria,
			}
			progress := spec.GetProgress()

			if progress.TotalCriteria != len(tt.criteria) {
				t.Errorf("TotalCriteria = %d, want %d", progress.TotalCriteria, len(tt.criteria))
			}
			if progress.SatisfiedCriteria != tt.wantSat {
				t.Errorf("SatisfiedCriteria = %d, want %d", progress.SatisfiedCriteria, tt.wantSat)
			}
			// Use tolerance for floating point comparison
			if math.Abs(progress.PercentComplete-tt.wantPct) > 0.0001 {
				t.Errorf("PercentComplete = %f, want %f", progress.PercentComplete, tt.wantPct)
			}
			if len(progress.PendingCriteria) != tt.wantPend {
				t.Errorf("PendingCriteria len = %d, want %d", len(progress.PendingCriteria), tt.wantPend)
			}
		})
	}
}

func TestProductSpec_GetCriterion(t *testing.T) {
	spec := &ProductSpec{
		AcceptanceCriteria: []AcceptanceCriterion{
			{ID: "AC1", Description: "First"},
			{ID: "AC2", Description: "Second"},
			{ID: "AC3", Description: "Third"},
		},
	}

	t.Run("find existing", func(t *testing.T) {
		criterion := spec.GetCriterion("AC2")
		if criterion == nil {
			t.Fatal("GetCriterion(AC2) returned nil")
		}
		if criterion.Description != "Second" {
			t.Errorf("Description = %q, want Second", criterion.Description)
		}
	})

	t.Run("find non-existent", func(t *testing.T) {
		criterion := spec.GetCriterion("AC99")
		if criterion != nil {
			t.Errorf("GetCriterion(AC99) = %v, want nil", criterion)
		}
	})

	t.Run("returns pointer to actual element", func(t *testing.T) {
		criterion := spec.GetCriterion("AC1")
		criterion.Satisfied = true

		if !spec.AcceptanceCriteria[0].Satisfied {
			t.Error("Modifying returned criterion should affect original")
		}
	})
}

func TestProductSpec_GetFeature(t *testing.T) {
	spec := &ProductSpec{
		Features: []Feature{
			{ID: "F1", Title: "Feature One"},
			{ID: "F2", Title: "Feature Two"},
		},
	}

	t.Run("find existing", func(t *testing.T) {
		feature := spec.GetFeature("F2")
		if feature == nil {
			t.Fatal("GetFeature(F2) returned nil")
		}
		if feature.Title != "Feature Two" {
			t.Errorf("Title = %q, want Feature Two", feature.Title)
		}
	})

	t.Run("find non-existent", func(t *testing.T) {
		feature := spec.GetFeature("F99")
		if feature != nil {
			t.Errorf("GetFeature(F99) = %v, want nil", feature)
		}
	})

	t.Run("returns pointer to actual element", func(t *testing.T) {
		feature := spec.GetFeature("F1")
		feature.Priority = PriorityHigh

		if spec.Features[0].Priority != PriorityHigh {
			t.Error("Modifying returned feature should affect original")
		}
	})
}

func TestProductSpec_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		criteria []AcceptanceCriterion
		want     bool
	}{
		{
			name:     "no criteria",
			criteria: []AcceptanceCriterion{},
			want:     false, // empty = not complete
		},
		{
			name: "all satisfied",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: true},
				{ID: "AC2", Satisfied: true},
			},
			want: true,
		},
		{
			name: "one unsatisfied",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: true},
				{ID: "AC2", Satisfied: false},
			},
			want: false,
		},
		{
			name: "all unsatisfied",
			criteria: []AcceptanceCriterion{
				{ID: "AC1", Satisfied: false},
				{ID: "AC2", Satisfied: false},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &ProductSpec{AcceptanceCriteria: tt.criteria}
			if got := spec.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProductSpec_Struct(t *testing.T) {
	now := time.Now()
	spec := &ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Goal 1", "Goal 2"},
		Features: []Feature{
			{
				ID:          "F1",
				Title:       "Feature One",
				Description: "Description",
				Priority:    PriorityHigh,
				API: &APISpec{
					Method: "POST",
					Path:   "/api/v1/resource",
				},
				SuccessCriteria: []string{"Works"},
			},
		},
		NonFunctional: NonFunctionalReqs{
			Performance: []string{"< 100ms response"},
			Security:    []string{"HTTPS required"},
		},
		AcceptanceCriteria: []AcceptanceCriterion{
			{ID: "AC1", Description: "Test", Satisfied: false, Evidence: ""},
		},
		Milestones: []Milestone{
			{ID: "M1", Name: "MVP", Features: []string{"F1"}, Target: "Q1"},
		},
		FilePath:  "/path/to/spec.yaml",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if spec.Name != "Test Spec" {
		t.Errorf("Name = %q, want Test Spec", spec.Name)
	}
	if len(spec.Goals) != 2 {
		t.Errorf("Goals len = %d, want 2", len(spec.Goals))
	}
	if spec.Features[0].API.Method != "POST" {
		t.Errorf("API.Method = %q, want POST", spec.Features[0].API.Method)
	}
}

func TestPriority_Constants(t *testing.T) {
	tests := []struct {
		priority Priority
		want     string
	}{
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.priority) != tt.want {
				t.Errorf("Priority = %q, want %q", tt.priority, tt.want)
			}
		})
	}
}

func TestFeature_Struct(t *testing.T) {
	feature := Feature{
		ID:          "F1",
		Title:       "User Authentication",
		Description: "Allow users to log in",
		Priority:    PriorityHigh,
		API: &APISpec{
			Method:   "POST",
			Path:     "/api/v1/login",
			Request:  `{"email": "string", "password": "string"}`,
			Response: `{"token": "string"}`,
		},
		SuccessCriteria: []string{"User can log in", "JWT token returned"},
	}

	if feature.ID != "F1" {
		t.Errorf("ID = %q, want F1", feature.ID)
	}
	if feature.API.Path != "/api/v1/login" {
		t.Errorf("API.Path = %q, want /api/v1/login", feature.API.Path)
	}
}

func TestNonFunctionalReqs_Struct(t *testing.T) {
	nfr := NonFunctionalReqs{
		Performance:  []string{"99th percentile < 200ms"},
		Security:     []string{"All data encrypted at rest"},
		Scalability:  []string{"Handle 10k concurrent users"},
		Availability: []string{"99.9% uptime"},
	}

	if len(nfr.Performance) != 1 {
		t.Errorf("Performance len = %d, want 1", len(nfr.Performance))
	}
}

func TestAcceptanceCriterion_Struct(t *testing.T) {
	ac := AcceptanceCriterion{
		ID:          "AC1",
		Description: "User can register",
		Satisfied:   true,
		Evidence:    "Test passes",
	}

	if !ac.Satisfied {
		t.Error("Satisfied should be true")
	}
	if ac.Evidence != "Test passes" {
		t.Errorf("Evidence = %q, want Test passes", ac.Evidence)
	}
}

func TestMilestone_Struct(t *testing.T) {
	milestone := Milestone{
		ID:          "M1",
		Name:        "MVP",
		Features:    []string{"F1", "F2"},
		Target:      "2024-Q1",
		Description: "Minimum viable product",
	}

	if milestone.Name != "MVP" {
		t.Errorf("Name = %q, want MVP", milestone.Name)
	}
	if len(milestone.Features) != 2 {
		t.Errorf("Features len = %d, want 2", len(milestone.Features))
	}
}

func TestSpecValidation_Struct(t *testing.T) {
	validation := SpecValidation{
		Valid:    false,
		Errors:   []string{"Missing version"},
		Warnings: []string{"No milestones defined"},
	}

	if validation.Valid {
		t.Error("Valid should be false")
	}
	if len(validation.Errors) != 1 {
		t.Errorf("Errors len = %d, want 1", len(validation.Errors))
	}
}

func TestSpecProgress_Struct(t *testing.T) {
	progress := SpecProgress{
		TotalCriteria:     10,
		SatisfiedCriteria: 7,
		PercentComplete:   70.0,
		PendingCriteria: []AcceptanceCriterion{
			{ID: "AC8"},
			{ID: "AC9"},
			{ID: "AC10"},
		},
	}

	if progress.PercentComplete != 70.0 {
		t.Errorf("PercentComplete = %f, want 70.0", progress.PercentComplete)
	}
	if len(progress.PendingCriteria) != 3 {
		t.Errorf("PendingCriteria len = %d, want 3", len(progress.PendingCriteria))
	}
}

func TestSpecLock_Struct(t *testing.T) {
	now := time.Now()
	lock := SpecLock{
		Version:  "1.0.0",
		SpecHash: "abc123",
		Features: map[string]LockedFeature{
			"F1": {Hash: "hash1", APIPath: "/api/v1/users", TestFile: "users_test.go"},
		},
		LockedAt: now,
	}

	if lock.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", lock.Version)
	}
	if lock.Features["F1"].APIPath != "/api/v1/users" {
		t.Errorf("Features[F1].APIPath = %q, want /api/v1/users", lock.Features["F1"].APIPath)
	}
}

func TestLockedFeature_Struct(t *testing.T) {
	lf := LockedFeature{
		Hash:     "sha256:abc123",
		APIPath:  "/api/v1/resource",
		TestFile: "resource_test.go",
	}

	if lf.Hash != "sha256:abc123" {
		t.Errorf("Hash = %q, want sha256:abc123", lf.Hash)
	}
}
