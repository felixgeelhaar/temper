package spec

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator() returned nil")
	}
}

func TestValidator_Validate_Valid(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Build a login system"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Login",
				Description: "User can log in",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"User sees success message",
				},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{
				ID:          "ac-1",
				Description: "User can log in with valid credentials",
			},
		},
		Milestones: []domain.Milestone{
			{
				ID:       "m-1",
				Name:     "MVP",
				Features: []string{"feat-1"},
			},
		},
	}

	result := v.Validate(spec)

	if !result.Valid {
		t.Errorf("Validate() = invalid, want valid. Errors: %v", result.Errors)
	}
}

func TestValidator_Validate_MissingName(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Goals:    []string{"Goal"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for missing name")
	}

	found := false
	for _, err := range result.Errors {
		if err == "spec name is required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error about missing name")
	}
}

func TestValidator_Validate_MissingGoals(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for missing goals")
	}
}

func TestValidator_Validate_MissingFeatures(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Goal"},
		Features: []domain.Feature{},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for missing features")
	}
}

func TestValidator_Validate_MissingAcceptanceCriteria(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:               "Test",
		Goals:              []string{"Goal"},
		Features:           []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for missing acceptance criteria")
	}
}

func TestValidator_Validate_DuplicateFeatureID(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:  "Test",
		Goals: []string{"Goal"},
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
			{ID: "feat-1", Title: "Feature 2"}, // Duplicate
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for duplicate feature IDs")
	}
}

func TestValidator_Validate_DuplicateCriterionID(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Goal"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion 1"},
			{ID: "ac-1", Description: "Criterion 2"}, // Duplicate
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for duplicate criterion IDs")
	}
}

func TestValidator_Validate_MilestoneUnknownFeature(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Goal"},
		Features: []domain.Feature{{ID: "feat-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
		Milestones: []domain.Milestone{
			{ID: "m-1", Name: "MVP", Features: []string{"unknown-feature"}},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for milestone referencing unknown feature")
	}
}

func TestValidator_Validate_VagueGoal(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Do stuff"}, // Too vague (< 20 chars)
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Should work"},
		},
	}

	result := v.Validate(spec)

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about vague goal")
	}
}

func TestValidator_Validate_PlaceholderGoal(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"TODO: Define the goal here"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
	}

	result := v.Validate(spec)

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about placeholder text")
	}
}

func TestValidator_Validate_FeatureMissingID(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for feature without ID")
	}
}

func TestValidator_Validate_FeatureMissingTitle(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "f-1", Title: ""}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Criterion"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for feature without title")
	}
}

func TestValidator_Validate_FeatureWithAPI(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:  "Test",
		Goals: []string{"Build an API"},
		Features: []domain.Feature{{
			ID:    "f-1",
			Title: "API Feature",
			API:   &domain.APISpec{Method: "", Path: ""}, // Missing method and path
		}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "API should work"},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for API without method/path")
	}
}

func TestValidator_Validate_CriterionMissingDescription(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: ""},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for criterion without description")
	}
}

func TestValidator_Validate_AmbiguousLanguage(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Should probably do something TBD"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
	}

	result := v.Validate(spec)

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about ambiguous language")
	}
}

func TestContainsPlaceholder(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"TODO: Implement this", true},
		{"FIXME: Bug here", true},
		{"[describe the feature]", true},
		{"<placeholder>", true},
		{"A normal description", false},
		{"XXX needs review", true},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := containsPlaceholder(tt.text)
			if got != tt.want {
				t.Errorf("containsPlaceholder(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestIsVerifiable(t *testing.T) {
	tests := []struct {
		description string
		want        bool
	}{
		{"User can log in", true},
		{"System should return 200", true},
		{"Response within 100ms", true},
		{"Given valid input, then success", true},
		{"Good feature", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			got := isVerifiable(tt.description)
			if got != tt.want {
				t.Errorf("isVerifiable(%q) = %v, want %v", tt.description, got, tt.want)
			}
		})
	}
}

func TestValidator_Validate_MilestoneDuplicateID(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
		Milestones: []domain.Milestone{
			{ID: "m-1", Name: "Phase 1", Features: []string{"f-1"}},
			{ID: "m-1", Name: "Phase 2", Features: []string{"f-1"}}, // Duplicate
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for duplicate milestone IDs")
	}
}

func TestValidator_Validate_MilestoneMissingName(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
		Milestones: []domain.Milestone{
			{ID: "m-1", Name: "", Features: []string{"f-1"}},
		},
	}

	result := v.Validate(spec)

	if result.Valid {
		t.Error("Validate() should be invalid for milestone without name")
	}
}

func TestValidator_Validate_MilestoneNoFeatures(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
		Milestones: []domain.Milestone{
			{ID: "m-1", Name: "Empty Milestone", Features: []string{}},
		},
	}

	result := v.Validate(spec)

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about milestone with no features")
	}
}

func TestValidator_Validate_FeatureNoSuccessCriteria(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:  "Test",
		Goals: []string{"Build a feature"},
		Features: []domain.Feature{{
			ID:              "f-1",
			Title:           "Feature",
			Description:     "A feature",
			SuccessCriteria: []string{}, // Empty
		}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
	}

	result := v.Validate(spec)

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about feature with no success criteria")
	}
}

func TestValidator_Validate_VersionWarning(t *testing.T) {
	v := NewValidator()
	spec := &domain.ProductSpec{
		Name:     "Test",
		Version:  "", // Empty version
		Goals:    []string{"Build a feature"},
		Features: []domain.Feature{{ID: "f-1", Title: "Feature"}},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "User can log in"},
		},
	}

	result := v.Validate(spec)

	found := false
	for _, w := range result.Warnings {
		if w == "spec version is not set, defaulting to 1.0.0" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected warning about missing version")
	}
}
