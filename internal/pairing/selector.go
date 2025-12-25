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
