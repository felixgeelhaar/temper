package spec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Validator checks spec completeness and consistency
type Validator struct{}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate performs comprehensive validation on a spec
func (v *Validator) Validate(spec *domain.ProductSpec) *domain.SpecValidation {
	validation := &domain.SpecValidation{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Check required fields
	v.checkRequired(spec, validation)

	// Check goals
	v.checkGoals(spec, validation)

	// Check features
	v.checkFeatures(spec, validation)

	// Check acceptance criteria
	v.checkAcceptanceCriteria(spec, validation)

	// Check milestones
	v.checkMilestones(spec, validation)

	// Identify potential ambiguities
	v.identifyAmbiguities(spec, validation)

	return validation
}

func (v *Validator) checkRequired(spec *domain.ProductSpec, validation *domain.SpecValidation) {
	if spec.Name == "" {
		validation.Errors = append(validation.Errors, "spec name is required")
		validation.Valid = false
	}

	if spec.Version == "" {
		validation.Warnings = append(validation.Warnings, "spec version is not set, defaulting to 1.0.0")
	}

	if len(spec.Goals) == 0 {
		validation.Errors = append(validation.Errors, "at least one goal is required")
		validation.Valid = false
	}

	if len(spec.Features) == 0 {
		validation.Errors = append(validation.Errors, "at least one feature is required")
		validation.Valid = false
	}

	if len(spec.AcceptanceCriteria) == 0 {
		validation.Errors = append(validation.Errors, "at least one acceptance criterion is required")
		validation.Valid = false
	}
}

func (v *Validator) checkGoals(spec *domain.ProductSpec, validation *domain.SpecValidation) {
	for i, goal := range spec.Goals {
		if strings.TrimSpace(goal) == "" {
			validation.Errors = append(validation.Errors, fmt.Sprintf("goal %d is empty", i+1))
			validation.Valid = false
			continue
		}

		// Warn if goal is too vague
		if len(goal) < 20 {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("goal %d may be too vague: %q", i+1, goal))
		}

		// Warn if goal contains placeholder text
		if containsPlaceholder(goal) {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("goal %d contains placeholder text", i+1))
		}
	}
}

func (v *Validator) checkFeatures(spec *domain.ProductSpec, validation *domain.SpecValidation) {
	featureIDs := make(map[string]bool)

	for i, feat := range spec.Features {
		// Check for duplicate IDs
		if featureIDs[feat.ID] {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("duplicate feature ID: %s", feat.ID))
			validation.Valid = false
		}
		featureIDs[feat.ID] = true

		// Check required fields
		if feat.ID == "" {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("feature %d is missing an ID", i+1))
			validation.Valid = false
		}

		if feat.Title == "" {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("feature %s is missing a title", feat.ID))
			validation.Valid = false
		}

		if feat.Description == "" {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("feature %s has no description", feat.ID))
		}

		// Validate priority
		switch feat.Priority {
		case domain.PriorityHigh, domain.PriorityMedium, domain.PriorityLow, "":
			// valid
		default:
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("feature %s has invalid priority: %s", feat.ID, feat.Priority))
		}

		// Check API spec if present
		if feat.API != nil {
			if feat.API.Method == "" {
				validation.Errors = append(validation.Errors,
					fmt.Sprintf("feature %s API is missing method", feat.ID))
				validation.Valid = false
			}
			if feat.API.Path == "" {
				validation.Errors = append(validation.Errors,
					fmt.Sprintf("feature %s API is missing path", feat.ID))
				validation.Valid = false
			}
		}

		// Check success criteria
		if len(feat.SuccessCriteria) == 0 {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("feature %s has no success criteria", feat.ID))
		}
	}
}

func (v *Validator) checkAcceptanceCriteria(spec *domain.ProductSpec, validation *domain.SpecValidation) {
	criteriaIDs := make(map[string]bool)

	for i, ac := range spec.AcceptanceCriteria {
		// Check for duplicate IDs
		if criteriaIDs[ac.ID] {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("duplicate acceptance criterion ID: %s", ac.ID))
			validation.Valid = false
		}
		criteriaIDs[ac.ID] = true

		// Check required fields
		if ac.ID == "" {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("acceptance criterion %d is missing an ID", i+1))
			validation.Valid = false
		}

		if ac.Description == "" {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("acceptance criterion %s is missing a description", ac.ID))
			validation.Valid = false
		}

		// Check for verifiable criteria
		if !isVerifiable(ac.Description) {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("acceptance criterion %s may not be verifiable: %q", ac.ID, ac.Description))
		}
	}
}

func (v *Validator) checkMilestones(spec *domain.ProductSpec, validation *domain.SpecValidation) {
	milestoneIDs := make(map[string]bool)
	featureIDs := make(map[string]bool)

	// Build set of valid feature IDs
	for _, feat := range spec.Features {
		featureIDs[feat.ID] = true
	}

	for i, m := range spec.Milestones {
		// Check for duplicate IDs
		if milestoneIDs[m.ID] {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("duplicate milestone ID: %s", m.ID))
			validation.Valid = false
		}
		milestoneIDs[m.ID] = true

		// Check required fields
		if m.ID == "" {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("milestone %d is missing an ID", i+1))
			validation.Valid = false
		}

		if m.Name == "" {
			validation.Errors = append(validation.Errors,
				fmt.Sprintf("milestone %s is missing a name", m.ID))
			validation.Valid = false
		}

		// Check that referenced features exist
		for _, featID := range m.Features {
			if !featureIDs[featID] {
				validation.Errors = append(validation.Errors,
					fmt.Sprintf("milestone %s references unknown feature: %s", m.ID, featID))
				validation.Valid = false
			}
		}

		// Warn if no features assigned
		if len(m.Features) == 0 {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("milestone %s has no features assigned", m.ID))
		}
	}
}

func (v *Validator) identifyAmbiguities(spec *domain.ProductSpec, validation *domain.SpecValidation) {
	// Check for ambiguous language patterns
	ambiguousPatterns := []string{
		`should\s+(?:probably|maybe|possibly)`,
		`(?:might|may)\s+need`,
		`(?:TBD|TBA|TODO|FIXME)`,
		`(?:etc\.?|and so on|and more)`,
		`(?:as needed|as required|when appropriate)`,
		`(?:or something|or similar)`,
	}

	checkText := func(text, context string) {
		for _, pattern := range ambiguousPatterns {
			re := regexp.MustCompile(`(?i)` + pattern)
			if re.MatchString(text) {
				validation.Warnings = append(validation.Warnings,
					fmt.Sprintf("ambiguous language in %s: %q", context, text))
			}
		}
	}

	for i, goal := range spec.Goals {
		checkText(goal, fmt.Sprintf("goal %d", i+1))
	}

	for _, feat := range spec.Features {
		checkText(feat.Description, fmt.Sprintf("feature %s", feat.ID))
		for j, sc := range feat.SuccessCriteria {
			checkText(sc, fmt.Sprintf("feature %s success criterion %d", feat.ID, j+1))
		}
	}

	for _, ac := range spec.AcceptanceCriteria {
		checkText(ac.Description, fmt.Sprintf("acceptance criterion %s", ac.ID))
	}
}

// Helper functions

func containsPlaceholder(text string) bool {
	placeholders := []string{
		"TODO", "FIXME", "XXX", "PLACEHOLDER",
		"[describe", "[define", "[fill in",
		"<placeholder>", "<description>",
	}

	lower := strings.ToLower(text)
	for _, p := range placeholders {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func isVerifiable(description string) bool {
	// Check for verifiable language patterns
	verifiablePatterns := []string{
		`\b(?:can|should|must|will)\b`,
		`\b(?:returns?|displays?|shows?|creates?)\b`,
		`\b(?:within|less than|more than|at least|at most)\b`,
		`\b(?:error|success|fail|pass)\b`,
		`\b(?:when|if|given|then)\b`,
	}

	lower := strings.ToLower(description)
	for _, pattern := range verifiablePatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if re.MatchString(lower) {
			return true
		}
	}

	return false
}
