package appreciation

import (
	"fmt"
	"math/rand"
)

// Message represents an appreciation message
type Message struct {
	Text     string     `json:"text"`
	Type     MomentType `json:"type"`
	Evidence Evidence   `json:"evidence"`
}

// Generator creates appreciation messages from moments
type Generator struct {
	templates map[MomentType][]string
}

// NewGenerator creates a new message generator with default templates
func NewGenerator() *Generator {
	return &Generator{
		templates: defaultTemplates(),
	}
}

// Generate creates an appreciation message for a moment
func (g *Generator) Generate(moment *Moment) *Message {
	if moment == nil {
		return nil
	}

	templates, ok := g.templates[moment.Type]
	if !ok || len(templates) == 0 {
		return nil
	}

	// Select a template randomly for variety
	template := templates[rand.Intn(len(templates))]

	// Format with evidence
	text := g.formatTemplate(template, moment)

	return &Message{
		Text:     text,
		Type:     moment.Type,
		Evidence: moment.Evidence,
	}
}

func (g *Generator) formatTemplate(template string, moment *Moment) string {
	e := moment.Evidence

	switch moment.Type {
	case MomentNoHintsNeeded:
		return fmt.Sprintf(template, e.RunCount)

	case MomentMinimalHints:
		if e.HintCount == 1 {
			return fmt.Sprintf(template, "just one hint")
		}
		return fmt.Sprintf(template, fmt.Sprintf("only %d hints", e.HintCount))

	case MomentNoEscalation:
		return fmt.Sprintf(template, e.HintCount)

	case MomentFirstTrySuccess:
		return template

	case MomentQuickResolution:
		return fmt.Sprintf(template, e.Duration)

	case MomentAllTestsPassing:
		return template

	case MomentReducedDependency:
		return fmt.Sprintf(template, e.ImprovementPercent)

	case MomentTopicMastery:
		return fmt.Sprintf(template, e.TopicName)

	case MomentConsistentSuccess:
		return template

	case MomentFirstInTopic:
		return fmt.Sprintf(template, e.TopicName)

	case MomentCriterionSatisfied:
		return fmt.Sprintf(template, e.CriterionID)

	case MomentSpecComplete:
		return fmt.Sprintf(template, e.SpecName)

	case MomentSessionEnd:
		return fmt.Sprintf(template, e.SessionDuration)

	default:
		return template
	}
}

func defaultTemplates() map[MomentType][]string {
	return map[MomentType][]string{
		// Session-level appreciation
		MomentNoHintsNeeded: {
			"You completed this without any hints. That shows solid understanding.",
			"No hints needed. Your independent problem-solving is growing.",
			"Completed in %d runs without assistance. Well done.",
		},

		MomentMinimalHints: {
			"You resolved this with %s. That shows growing confidence.",
			"Completed with %s. You're relying less on guidance.",
			"Only %s needed. Your self-reliance is improving.",
		},

		MomentNoEscalation: {
			"You worked through this with %d hints and never needed more detailed help. That discipline pays off.",
			"Stayed at light guidance level with %d hints. You're building genuine understanding.",
			"No escalation needed despite %d hints. That restraint builds stronger skills.",
		},

		MomentFirstTrySuccess: {
			"All tests passed on the first run. Your preparation shows.",
			"First attempt, all green. That's solid work.",
			"Tests passing on the first try. Your thinking before coding is paying off.",
		},

		MomentQuickResolution: {
			"Resolved in %s. Your pattern recognition is improving.",
			"Quick resolution in %s. You're building fluency.",
		},

		MomentAllTestsPassing: {
			"All tests passing. The code works as intended.",
			"Clean test run. Your solution is correct.",
		},

		// Progress-level appreciation
		MomentReducedDependency: {
			"Your hint dependency dropped by %.0f%%. You're solving more independently.",
			"%.0f%% less reliance on hints than before. Real progress.",
			"You've reduced your hint dependency by %.0f%%. That's meaningful growth.",
		},

		MomentTopicMastery: {
			"You've developed strong skills in %s. The fundamentals are solid.",
			"Mastery achieved in %s. You can tackle more complex challenges in this area.",
		},

		MomentConsistentSuccess: {
			"Multiple sessions with low hint usage. Your skills are becoming reliable.",
			"Consistent success without heavy guidance. You're building real competence.",
		},

		MomentFirstInTopic: {
			"First exercise completed in %s. Every journey starts with a single step.",
			"Started learning %s. The foundation is laid.",
		},

		// Spec-level appreciation
		MomentCriterionSatisfied: {
			"Acceptance criterion %s satisfied. Progress toward the goal.",
			"Criterion %s complete. One step closer to the spec.",
		},

		MomentSpecComplete: {
			"All acceptance criteria for %s satisfied. The feature is complete.",
			"Spec %s complete. You built what was needed, nothing more.",
		},

		// Session end messages
		MomentSessionEnd: {
			"Good session. %s of focused work builds lasting skill.",
			"Session complete. %s invested in your craft.",
			"Every session counts. %s of practice today.",
		},
	}
}

// ShouldAppreciate determines if we should show appreciation (avoid spam)
func ShouldAppreciate(lastAppreciationMinutes int, momentPriority int) bool {
	// Higher priority moments can be shown more frequently
	// Lower priority moments need more time between appreciations

	switch {
	case momentPriority >= 8:
		// High priority (spec complete, mastery, first try): always show
		return true
	case momentPriority >= 5:
		// Medium priority: at least 30 minutes between appreciations
		return lastAppreciationMinutes >= 30
	default:
		// Low priority: at least 60 minutes between appreciations
		return lastAppreciationMinutes >= 60
	}
}
