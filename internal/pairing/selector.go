package pairing

import (
	"github.com/felixgeelhaar/temper/internal/domain"
)

// Selector determines intervention level and type
type Selector struct{}

// NewSelector creates a new selector
func NewSelector() *Selector {
	return &Selector{}
}

// SelectLevel determines the appropriate intervention level
func (s *Selector) SelectLevel(intent domain.Intent, ctx InterventionContext, policy domain.LearningPolicy) domain.InterventionLevel {
	// Base level selection based on intent
	baseLevel := s.intentToBaseLevel(intent)

	// Adjust based on context signals
	level := s.adjustForContext(baseLevel, ctx)

	// Adjust based on profile signals
	level = s.adjustForProfile(level, ctx.Profile)

	// Adjust based on run output
	level = s.adjustForRunOutput(level, ctx.RunOutput)

	// Adjust based on spec context (feature guidance sessions)
	level = s.adjustForSpec(level, ctx)

	return level
}

// intentToBaseLevel maps intent to base intervention level
func (s *Selector) intentToBaseLevel(intent domain.Intent) domain.InterventionLevel {
	switch intent {
	case domain.IntentHint:
		return domain.L1CategoryHint
	case domain.IntentReview:
		return domain.L2LocationConcept
	case domain.IntentStuck:
		return domain.L2LocationConcept
	case domain.IntentNext:
		return domain.L1CategoryHint
	case domain.IntentExplain:
		return domain.L2LocationConcept
	default:
		return domain.L1CategoryHint
	}
}

// adjustForContext adjusts level based on exercise context
func (s *Selector) adjustForContext(level domain.InterventionLevel, ctx InterventionContext) domain.InterventionLevel {
	if ctx.Exercise == nil {
		return level
	}

	// Harder exercises might warrant slightly more help
	switch ctx.Exercise.Difficulty {
	case domain.DifficultyBeginner:
		// No adjustment for beginners
	case domain.DifficultyIntermediate:
		// Keep as is
	case domain.DifficultyAdvanced:
		// Maybe slightly more restrictive for advanced
		if level > domain.L1CategoryHint {
			level = domain.InterventionLevel(int(level) - 1)
		}
	}

	return level
}

// adjustForProfile adjusts level based on learning profile
func (s *Selector) adjustForProfile(level domain.InterventionLevel, profile *domain.LearningProfile) domain.InterventionLevel {
	if profile == nil {
		return level
	}

	// If user is very dependent on hints, don't increase level
	dependency := profile.HintDependency()
	if dependency > 0.5 {
		// Keep level as is - user needs support
		return level
	}

	// If user is very independent, consider reducing level
	if dependency < 0.2 && profile.TotalRuns > 10 {
		if level > domain.L0Clarify {
			return domain.InterventionLevel(int(level) - 1)
		}
	}

	return level
}

// adjustForRunOutput adjusts level based on run results
func (s *Selector) adjustForRunOutput(level domain.InterventionLevel, output *domain.RunOutput) domain.InterventionLevel {
	if output == nil {
		return level
	}

	// If all tests pass, user might just need a nudge
	if output.AllTestsPassed() {
		if level > domain.L0Clarify {
			return domain.L0Clarify
		}
	}

	// If there are build errors, might need more specific help
	if output.HasErrors() && level < domain.L2LocationConcept {
		return domain.L2LocationConcept
	}

	// If many tests failing, provide more guidance
	if output.TestsFailed > 3 && level < domain.L2LocationConcept {
		return domain.L2LocationConcept
	}

	return level
}

// adjustForSpec adjusts level based on spec context for feature guidance sessions
func (s *Selector) adjustForSpec(level domain.InterventionLevel, ctx InterventionContext) domain.InterventionLevel {
	if !ctx.HasSpec() {
		return level
	}

	// Feature guidance sessions are more structured
	// We anchor feedback to acceptance criteria

	// If there's a focus criterion, keep level focused
	if ctx.FocusCriterion != nil {
		// If working on a specific criterion, provide targeted help
		// but maintain learning-first approach
		return level
	}

	// Check spec progress - if many criteria satisfied, user is doing well
	satisfied, total := ctx.SpecProgress()
	if total > 0 {
		progress := float64(satisfied) / float64(total)

		// If significant progress, trust the user more
		if progress > 0.5 && level > domain.L1CategoryHint {
			return domain.InterventionLevel(int(level) - 1)
		}
	}

	// Detect potential scope drift - if code is going off-spec,
	// guide back gently. This info will be in the prompt
	// to help anchor feedback to spec scope.
	// (Scope drift detection happens upstream, level adjustment not needed here)

	return level
}

// SelectType determines the intervention type based on intent and level
func (s *Selector) SelectType(intent domain.Intent, level domain.InterventionLevel) domain.InterventionType {
	// Map level to primary type
	switch level {
	case domain.L0Clarify:
		return domain.TypeQuestion
	case domain.L1CategoryHint:
		return domain.TypeHint
	case domain.L2LocationConcept:
		if intent == domain.IntentReview {
			return domain.TypeCritique
		}
		return domain.TypeNudge
	case domain.L3ConstrainedSnippet:
		if intent == domain.IntentExplain {
			return domain.TypeExplain
		}
		return domain.TypeSnippet
	case domain.L4PartialSolution, domain.L5FullSolution:
		return domain.TypeSnippet
	default:
		return domain.TypeHint
	}
}

// ShouldEscalate determines if we should escalate to a higher level
func (s *Selector) ShouldEscalate(attempts int, lastLevel domain.InterventionLevel) bool {
	// Escalate after multiple attempts at the same level
	if attempts >= 3 && lastLevel < domain.L3ConstrainedSnippet {
		return true
	}
	return false
}
