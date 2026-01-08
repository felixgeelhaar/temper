package pairing

import (
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
)

// AuthoringContext holds context for spec authoring sessions
type AuthoringContext struct {
	Spec      *domain.ProductSpec // The spec being authored
	Section   string              // Current section: goals, features, acceptance_criteria, non_functional
	Documents []domain.Document   // Discovered project documents
	Question  string              // Optional user question for hints
}

// HasDocuments returns true if there are documents available
func (c *AuthoringContext) HasDocuments() bool {
	return len(c.Documents) > 0
}

// GetDocumentContent returns all document content formatted for LLM
func (c *AuthoringContext) GetDocumentContent(maxTokens int) string {
	if len(c.Documents) == 0 {
		return ""
	}

	// Rough estimate: 1 token ~= 4 chars
	maxChars := maxTokens * 4
	var content string

	for _, doc := range c.Documents {
		if len(content) >= maxChars {
			break
		}

		content += "---\n"
		content += "# " + doc.Title + " (" + doc.Path + ")\n\n"

		for _, section := range doc.Sections {
			if len(content) >= maxChars {
				break
			}

			for i := 0; i < section.Level; i++ {
				content += "#"
			}
			if section.Level > 0 {
				content += " "
			}
			content += section.Heading + "\n\n"

			remaining := maxChars - len(content)
			if remaining > 0 {
				sectionContent := section.Content
				if len(sectionContent) > remaining {
					sectionContent = sectionContent[:remaining-3] + "..."
				}
				content += sectionContent + "\n\n"
			}
		}
	}

	return content
}

// InterventionContext holds all context for intervention selection and generation.
// This consolidates the various signals used to determine appropriate intervention level.
type InterventionContext struct {
	// Core exercise/training context
	Exercise *domain.Exercise
	Code     map[string]string
	Profile  *domain.LearningProfile

	// Run output signals
	RunOutput *domain.RunOutput

	// Editor context
	CurrentFile string
	CursorLine  int

	// Session context
	SessionIntent session.SessionIntent

	// Spec context (for feature guidance sessions)
	Spec           *domain.ProductSpec
	FocusCriterion *domain.AcceptanceCriterion
}

// HasSpec returns true if this context has spec information
func (c *InterventionContext) HasSpec() bool {
	return c.Spec != nil
}

// IsFeatureGuidance returns true if this is a feature guidance session
func (c *InterventionContext) IsFeatureGuidance() bool {
	return c.SessionIntent == session.IntentFeatureGuidance && c.Spec != nil
}

// GetNextUnsatisfiedCriterion returns the next unsatisfied acceptance criterion
func (c *InterventionContext) GetNextUnsatisfiedCriterion() *domain.AcceptanceCriterion {
	if c.Spec == nil {
		return nil
	}

	for i := range c.Spec.AcceptanceCriteria {
		if !c.Spec.AcceptanceCriteria[i].Satisfied {
			return &c.Spec.AcceptanceCriteria[i]
		}
	}
	return nil
}

// GetCurrentFeature returns the feature associated with the focus criterion
func (c *InterventionContext) GetCurrentFeature() *domain.Feature {
	if c.Spec == nil || c.FocusCriterion == nil {
		return nil
	}

	// For now, return the first high-priority feature
	// A more sophisticated implementation would use explicit linking
	for i := range c.Spec.Features {
		if c.Spec.Features[i].Priority == domain.PriorityHigh {
			return &c.Spec.Features[i]
		}
	}

	if len(c.Spec.Features) > 0 {
		return &c.Spec.Features[0]
	}
	return nil
}

// SpecProgress returns the completion progress for the spec
func (c *InterventionContext) SpecProgress() (satisfied, total int) {
	if c.Spec == nil {
		return 0, 0
	}

	total = len(c.Spec.AcceptanceCriteria)
	for _, ac := range c.Spec.AcceptanceCriteria {
		if ac.Satisfied {
			satisfied++
		}
	}
	return satisfied, total
}

// ScopeDriftIndicators checks if the code might be drifting from spec scope
type ScopeDriftIndicators struct {
	OutOfScopeAPIs    []string // API paths in code not in spec
	MissingSpecAPIs   []string // Spec APIs not yet implemented
	UnrelatedPatterns []string // Code patterns not relevant to spec goals
}

// CheckScopeDrift analyzes code against spec for potential drift
// This is a placeholder for more sophisticated analysis
func (c *InterventionContext) CheckScopeDrift() *ScopeDriftIndicators {
	if !c.HasSpec() {
		return nil
	}

	indicators := &ScopeDriftIndicators{
		OutOfScopeAPIs:    []string{},
		MissingSpecAPIs:   []string{},
		UnrelatedPatterns: []string{},
	}

	// Collect spec API paths
	specAPIs := make(map[string]bool)
	for _, feat := range c.Spec.Features {
		if feat.API != nil {
			specAPIs[feat.API.Path] = true
		}
	}

	// This would be enhanced with actual code analysis
	// For now, just identify missing spec APIs
	for path := range specAPIs {
		indicators.MissingSpecAPIs = append(indicators.MissingSpecAPIs, path)
	}

	return indicators
}
