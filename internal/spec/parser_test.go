package spec

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestNewSpecTemplate(t *testing.T) {
	spec := NewSpecTemplate("User Authentication")

	if spec.Name != "User Authentication" {
		t.Errorf("Name = %v, want User Authentication", spec.Name)
	}

	if spec.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", spec.Version)
	}

	if len(spec.Goals) == 0 {
		t.Error("Goals should not be empty")
	}

	if len(spec.Features) == 0 {
		t.Error("Features should not be empty")
	}

	if len(spec.AcceptanceCriteria) == 0 {
		t.Error("AcceptanceCriteria should not be empty")
	}

	if len(spec.Milestones) == 0 {
		t.Error("Milestones should not be empty")
	}

	if spec.FilePath == "" {
		t.Error("FilePath should not be empty")
	}

	if spec.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestGenerateSpecFromDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantGoal    string
	}{
		{
			name:        "short description",
			description: "Build a login system",
			wantGoal:    "Build a login system",
		},
		{
			name:        "long description truncated",
			description: "This is a very long description that exceeds two hundred characters and should be truncated appropriately. It contains many words and details about what the feature should do and how it should work in various scenarios.",
			wantGoal:    "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := GenerateSpecFromDescription(tt.description)

			if spec == nil {
				t.Fatal("GenerateSpecFromDescription() returned nil")
			}

			if len(spec.Goals) == 0 {
				t.Error("Goals should not be empty")
			}

			// Check truncation for long description
			if tt.name == "long description truncated" {
				if len(spec.Goals[0]) > 203 { // 200 + "..."
					t.Errorf("Goal should be truncated, got %d chars", len(spec.Goals[0]))
				}
			}
		})
	}
}

func TestMergeSpecs(t *testing.T) {
	base := &domain.ProductSpec{
		Name:    "Base Spec",
		Version: "1.0.0",
		Goals:   []string{"Base goal"},
		Features: []domain.Feature{
			{ID: "base-1", Title: "Base Feature"},
		},
	}

	overlay := &domain.ProductSpec{
		Name:    "Overlay Spec",
		Version: "2.0.0",
		Goals:   []string{"Overlay goal"},
	}

	merged := MergeSpecs(base, overlay)

	if merged.Name != "Overlay Spec" {
		t.Errorf("Name = %v, want Overlay Spec", merged.Name)
	}

	if merged.Version != "2.0.0" {
		t.Errorf("Version = %v, want 2.0.0", merged.Version)
	}

	if len(merged.Goals) != 1 || merged.Goals[0] != "Overlay goal" {
		t.Errorf("Goals = %v, want [Overlay goal]", merged.Goals)
	}

	// Base features should remain if overlay doesn't override
	if len(overlay.Features) == 0 && len(merged.Features) != 1 {
		t.Error("Features should be preserved from base")
	}
}

func TestAddFeature(t *testing.T) {
	spec := NewSpecTemplate("Test")
	initialCount := len(spec.Features)

	AddFeature(spec, "New Feature", "A new feature description", domain.PriorityHigh)

	if len(spec.Features) != initialCount+1 {
		t.Errorf("Features count = %d, want %d", len(spec.Features), initialCount+1)
	}

	last := spec.Features[len(spec.Features)-1]
	if last.Title != "New Feature" {
		t.Errorf("Feature.Title = %v, want New Feature", last.Title)
	}
	if last.Description != "A new feature description" {
		t.Errorf("Feature.Description = %v, want description", last.Description)
	}
	if last.Priority != domain.PriorityHigh {
		t.Errorf("Feature.Priority = %v, want high", last.Priority)
	}
	if last.ID == "" {
		t.Error("Feature.ID should not be empty")
	}
}

func TestAddAcceptanceCriterion(t *testing.T) {
	spec := NewSpecTemplate("Test")
	initialCount := len(spec.AcceptanceCriteria)

	AddAcceptanceCriterion(spec, "User can log in")

	if len(spec.AcceptanceCriteria) != initialCount+1 {
		t.Errorf("AcceptanceCriteria count = %d, want %d", len(spec.AcceptanceCriteria), initialCount+1)
	}

	last := spec.AcceptanceCriteria[len(spec.AcceptanceCriteria)-1]
	if last.Description != "User can log in" {
		t.Errorf("Criterion.Description = %v, want description", last.Description)
	}
	if last.Satisfied {
		t.Error("New criterion should not be satisfied")
	}
	if last.ID == "" {
		t.Error("Criterion.ID should not be empty")
	}
}

func TestMarkCriterionSatisfied(t *testing.T) {
	spec := NewSpecTemplate("Test")

	// Get first criterion ID
	criterionID := spec.AcceptanceCriteria[0].ID

	// Mark as satisfied
	result := MarkCriterionSatisfied(spec, criterionID, "Tests passing")

	if !result {
		t.Error("MarkCriterionSatisfied() should return true")
	}

	if !spec.AcceptanceCriteria[0].Satisfied {
		t.Error("Criterion should be marked as satisfied")
	}

	if spec.AcceptanceCriteria[0].Evidence != "Tests passing" {
		t.Errorf("Evidence = %v, want Tests passing", spec.AcceptanceCriteria[0].Evidence)
	}

	// Try to mark non-existent criterion
	result = MarkCriterionSatisfied(spec, "non-existent", "evidence")
	if result {
		t.Error("MarkCriterionSatisfied() should return false for non-existent ID")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"User Authentication", "user-authentication"},
		{"API v2.0", "api-v2-0"},
		{"My App!", "my-app"},
		{"  spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
		{"a-b-c", "a-b-c"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugify_LongInput(t *testing.T) {
	longInput := "This is a very long name that exceeds fifty characters and should be truncated"
	result := slugify(longInput)

	if len(result) > 50 {
		t.Errorf("slugify() result too long: %d chars", len(result))
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "first sentence",
			input: "Build a login system. It should support OAuth.",
			want:  "Build a login system",
		},
		{
			name:  "no sentence",
			input: "Build a login system",
			want:  "Build a login system",
		},
		{
			name:  "long text truncated",
			input: "This is a very long description without any period that goes on and on and on",
			want:  "This is a very long description without any period",
		},
		{
			name:  "short text",
			input: "Short",
			want:  "Short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.input)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateSpecYAML(t *testing.T) {
	validYAML := []byte(`
name: Test Spec
version: "1.0.0"
goals:
  - Build a feature
features:
  - id: feat-1
    title: Feature One
acceptance_criteria:
  - id: ac-1
    description: It works
`)

	err := ValidateSpecYAML(validYAML)
	if err != nil {
		t.Errorf("ValidateSpecYAML() error = %v for valid YAML", err)
	}

	invalidYAML := []byte(`
this is: not valid: yaml: at: all
`)

	err = ValidateSpecYAML(invalidYAML)
	if err == nil {
		t.Error("ValidateSpecYAML() should error for invalid YAML")
	}
}

func TestMergeSpecs_NonFunctional(t *testing.T) {
	base := &domain.ProductSpec{
		Name: "Base",
		NonFunctional: domain.NonFunctionalReqs{
			Performance: []string{"Fast"},
			Security:    []string{"Secure"},
		},
	}

	overlay := &domain.ProductSpec{
		NonFunctional: domain.NonFunctionalReqs{
			Security:    []string{"Very Secure"},
			Scalability: []string{"Scale to 1M users"},
		},
	}

	merged := MergeSpecs(base, overlay)

	// Security should be overridden
	if len(merged.NonFunctional.Security) != 1 || merged.NonFunctional.Security[0] != "Very Secure" {
		t.Errorf("Security = %v, want [Very Secure]", merged.NonFunctional.Security)
	}

	// Scalability should be set
	if len(merged.NonFunctional.Scalability) != 1 {
		t.Errorf("Scalability = %v, want [Scale to 1M users]", merged.NonFunctional.Scalability)
	}

	// Performance should remain from base (overlay is empty)
	if len(merged.NonFunctional.Performance) != 1 {
		t.Errorf("Performance = %v, want [Fast]", merged.NonFunctional.Performance)
	}
}
