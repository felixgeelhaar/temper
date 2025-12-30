package spec

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// NewSpecTemplate creates a new spec with sensible defaults
func NewSpecTemplate(name string) *domain.ProductSpec {
	slug := slugify(name)
	now := time.Now()

	return &domain.ProductSpec{
		Name:    name,
		Version: "1.0.0",
		Goals: []string{
			"Define the primary objective of this feature",
		},
		Features: []domain.Feature{
			{
				ID:          fmt.Sprintf("feat-%s-1", slug),
				Title:       "Core Feature",
				Description: "Describe what this feature does",
				Priority:    domain.PriorityHigh,
				SuccessCriteria: []string{
					"Define measurable success criteria",
				},
			},
		},
		NonFunctional: domain.NonFunctionalReqs{
			Performance: []string{},
			Security:    []string{},
			Scalability: []string{},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{
				ID:          "ac-1",
				Description: "Define a verifiable acceptance criterion",
				Satisfied:   false,
			},
		},
		Milestones: []domain.Milestone{
			{
				ID:          "m-1",
				Name:        "MVP",
				Features:    []string{fmt.Sprintf("feat-%s-1", slug)},
				Target:      now.AddDate(0, 0, 14).Format("2006-01-02"),
				Description: "Initial implementation",
			},
		},
		FilePath:  fmt.Sprintf("%s.yaml", slug),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// GenerateSpecFromDescription creates a spec template from a natural language description
func GenerateSpecFromDescription(description string) *domain.ProductSpec {
	// Extract a title from the first sentence or first 50 chars
	title := extractTitle(description)
	spec := NewSpecTemplate(title)

	// Set the first goal to be the description
	if len(description) > 200 {
		spec.Goals[0] = description[:200] + "..."
	} else {
		spec.Goals[0] = description
	}

	return spec
}

// ValidateSpecYAML checks if the content is valid spec YAML without fully parsing
func ValidateSpecYAML(content []byte) error {
	_, err := ParseSpec(content)
	if err != nil {
		return fmt.Errorf("invalid spec YAML: %w", err)
	}
	return nil
}

// MergeSpecs combines two specs, with the overlay taking precedence
func MergeSpecs(base, overlay *domain.ProductSpec) *domain.ProductSpec {
	merged := *base // shallow copy

	if overlay.Name != "" {
		merged.Name = overlay.Name
	}
	if overlay.Version != "" {
		merged.Version = overlay.Version
	}
	if len(overlay.Goals) > 0 {
		merged.Goals = overlay.Goals
	}
	if len(overlay.Features) > 0 {
		merged.Features = overlay.Features
	}
	if len(overlay.AcceptanceCriteria) > 0 {
		merged.AcceptanceCriteria = overlay.AcceptanceCriteria
	}
	if len(overlay.Milestones) > 0 {
		merged.Milestones = overlay.Milestones
	}

	// Merge non-functional requirements
	if len(overlay.NonFunctional.Performance) > 0 {
		merged.NonFunctional.Performance = overlay.NonFunctional.Performance
	}
	if len(overlay.NonFunctional.Security) > 0 {
		merged.NonFunctional.Security = overlay.NonFunctional.Security
	}
	if len(overlay.NonFunctional.Scalability) > 0 {
		merged.NonFunctional.Scalability = overlay.NonFunctional.Scalability
	}
	if len(overlay.NonFunctional.Availability) > 0 {
		merged.NonFunctional.Availability = overlay.NonFunctional.Availability
	}

	merged.UpdatedAt = time.Now()
	return &merged
}

// AddFeature adds a new feature to the spec
func AddFeature(spec *domain.ProductSpec, title, description string, priority domain.Priority) {
	id := fmt.Sprintf("feat-%d", len(spec.Features)+1)
	spec.Features = append(spec.Features, domain.Feature{
		ID:          id,
		Title:       title,
		Description: description,
		Priority:    priority,
	})
	spec.UpdatedAt = time.Now()
}

// AddAcceptanceCriterion adds a new acceptance criterion to the spec
func AddAcceptanceCriterion(spec *domain.ProductSpec, description string) {
	id := fmt.Sprintf("ac-%d", len(spec.AcceptanceCriteria)+1)
	spec.AcceptanceCriteria = append(spec.AcceptanceCriteria, domain.AcceptanceCriterion{
		ID:          id,
		Description: description,
		Satisfied:   false,
	})
	spec.UpdatedAt = time.Now()
}

// MarkCriterionSatisfied marks an acceptance criterion as satisfied with evidence
func MarkCriterionSatisfied(spec *domain.ProductSpec, criterionID, evidence string) bool {
	for i := range spec.AcceptanceCriteria {
		if spec.AcceptanceCriteria[i].ID == criterionID {
			spec.AcceptanceCriteria[i].Satisfied = true
			spec.AcceptanceCriteria[i].Evidence = evidence
			spec.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// Helper functions

func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and special chars with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")

	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")

	// Limit length
	if len(s) > 50 {
		s = s[:50]
	}

	return s
}

func extractTitle(description string) string {
	// Try to find first sentence
	if idx := strings.Index(description, "."); idx > 0 && idx < 100 {
		return strings.TrimSpace(description[:idx])
	}

	// Otherwise take first 50 chars
	if len(description) > 50 {
		return strings.TrimSpace(description[:50])
	}

	return strings.TrimSpace(description)
}
