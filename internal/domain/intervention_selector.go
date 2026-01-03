package domain

// InterventionSelector is a domain service that determines intervention level and type.
// This encapsulates the core business rules for the Learning Contract.
type InterventionSelector struct{}

// NewInterventionSelector creates a new intervention selector
func NewInterventionSelector() *InterventionSelector {
	return &InterventionSelector{}
}

// SelectionContext contains the domain-level context for intervention selection.
// This is a pure domain type with no dependencies on application layer.
type SelectionContext struct {
	Exercise  *Exercise
	Profile   *LearningProfile
	RunOutput *RunOutput
	Spec      *ProductSpec
	// FocusCriterion is the current acceptance criterion being worked on
	FocusCriterion *AcceptanceCriterion
}

// HasSpec returns true if this context has spec information
func (c *SelectionContext) HasSpec() bool {
	return c.Spec != nil
}

// SpecProgress returns the number of satisfied and total acceptance criteria
func (c *SelectionContext) SpecProgress() (satisfied, total int) {
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

// SelectLevel determines the appropriate intervention level based on intent and context.
// This implements the core Learning Contract business rules.
func (s *InterventionSelector) SelectLevel(intent Intent, ctx SelectionContext, policy LearningPolicy) InterventionLevel {
	// Base level selection based on intent
	baseLevel := s.IntentToBaseLevel(intent)

	// Adjust based on context signals
	level := s.AdjustForContext(baseLevel, ctx)

	// Adjust based on profile signals
	level = s.AdjustForProfile(level, ctx.Profile)

	// Adjust based on run output
	level = s.AdjustForRunOutput(level, ctx.RunOutput)

	// Adjust based on spec context (feature guidance sessions)
	level = s.AdjustForSpec(level, ctx)

	return level
}

// IntentToBaseLevel maps an intent to its base intervention level.
// This establishes the default level before contextual adjustments.
func (s *InterventionSelector) IntentToBaseLevel(intent Intent) InterventionLevel {
	switch intent {
	case IntentHint:
		return L1CategoryHint
	case IntentReview:
		return L2LocationConcept
	case IntentStuck:
		return L2LocationConcept
	case IntentNext:
		return L1CategoryHint
	case IntentExplain:
		return L2LocationConcept
	default:
		return L1CategoryHint
	}
}

// AdjustForContext adjusts level based on exercise context.
// Advanced exercises warrant slightly less help to maintain challenge.
func (s *InterventionSelector) AdjustForContext(level InterventionLevel, ctx SelectionContext) InterventionLevel {
	if ctx.Exercise == nil {
		return level
	}

	// Harder exercises might warrant slightly less help
	switch ctx.Exercise.Difficulty {
	case DifficultyBeginner:
		// No adjustment for beginners
	case DifficultyIntermediate:
		// Keep as is
	case DifficultyAdvanced:
		// Slightly more restrictive for advanced
		if level > L1CategoryHint {
			level = InterventionLevel(int(level) - 1)
		}
	}

	return level
}

// AdjustForProfile adjusts level based on learning profile.
// Users with lower hint dependency get less direct help.
func (s *InterventionSelector) AdjustForProfile(level InterventionLevel, profile *LearningProfile) InterventionLevel {
	if profile == nil {
		return level
	}

	// If user is very dependent on hints, don't reduce level
	dependency := profile.HintDependency()
	if dependency > 0.5 {
		// Keep level as is - user needs support
		return level
	}

	// If user is very independent, consider reducing level
	if dependency < 0.2 && profile.TotalRuns > 10 {
		if level > L0Clarify {
			return InterventionLevel(int(level) - 1)
		}
	}

	return level
}

// AdjustForRunOutput adjusts level based on run results.
// Passing tests indicate less help needed; build errors indicate more help needed.
func (s *InterventionSelector) AdjustForRunOutput(level InterventionLevel, output *RunOutput) InterventionLevel {
	if output == nil {
		return level
	}

	// If all tests pass, user might just need a nudge
	if output.AllTestsPassed() {
		if level > L0Clarify {
			return L0Clarify
		}
	}

	// If there are build errors, might need more specific help
	if output.HasErrors() && level < L2LocationConcept {
		return L2LocationConcept
	}

	// If many tests failing, provide more guidance
	if output.TestsFailed > 3 && level < L2LocationConcept {
		return L2LocationConcept
	}

	return level
}

// AdjustForSpec adjusts level based on spec context for feature guidance sessions.
// High progress on acceptance criteria indicates user competence.
func (s *InterventionSelector) AdjustForSpec(level InterventionLevel, ctx SelectionContext) InterventionLevel {
	if !ctx.HasSpec() {
		return level
	}

	// If there's a focus criterion, provide targeted help
	if ctx.FocusCriterion != nil {
		// If working on a specific criterion, maintain level
		// The prompt will anchor feedback to this criterion
		return level
	}

	// Check spec progress - if many criteria satisfied, user is doing well
	satisfied, total := ctx.SpecProgress()
	if total > 0 {
		progress := float64(satisfied) / float64(total)

		// If significant progress, trust the user more
		if progress > 0.5 && level > L1CategoryHint {
			return InterventionLevel(int(level) - 1)
		}
	}

	return level
}

// SelectType determines the intervention type based on intent and level.
// This maps the selected level to an appropriate response format.
func (s *InterventionSelector) SelectType(intent Intent, level InterventionLevel) InterventionType {
	switch level {
	case L0Clarify:
		return TypeQuestion
	case L1CategoryHint:
		return TypeHint
	case L2LocationConcept:
		if intent == IntentReview {
			return TypeCritique
		}
		return TypeNudge
	case L3ConstrainedSnippet:
		if intent == IntentExplain {
			return TypeExplain
		}
		return TypeSnippet
	case L4PartialSolution, L5FullSolution:
		return TypeSnippet
	default:
		return TypeHint
	}
}

// ShouldEscalate determines if we should escalate to a higher intervention level.
// Escalation occurs after multiple unsuccessful attempts at the same level.
func (s *InterventionSelector) ShouldEscalate(attempts int, lastLevel InterventionLevel) bool {
	// Escalate after multiple attempts at the same level
	// But not beyond L3 (gated escalation for L4/L5)
	if attempts >= 3 && lastLevel < L3ConstrainedSnippet {
		return true
	}
	return false
}

// EscalateLevel returns the next higher intervention level.
// Returns the same level if already at maximum.
func (s *InterventionSelector) EscalateLevel(current InterventionLevel) InterventionLevel {
	if current < L3ConstrainedSnippet {
		return InterventionLevel(int(current) + 1)
	}
	return current
}
