package domain

import (
	"time"

	"github.com/google/uuid"
)

// Intervention represents an AI guidance action
type Intervention struct {
	ID          uuid.UUID
	SessionID   uuid.UUID
	UserID      uuid.UUID
	RunID       *uuid.UUID
	Intent      Intent
	Level       InterventionLevel
	Type        InterventionType
	Content     string   // the actual guidance text
	Targets     []Target // file/line targets
	Rationale   string   // internal reasoning (not shown to user)
	RequestedAt time.Time
	DeliveredAt time.Time
}

// Intent represents what the user is asking for
type Intent string

const (
	IntentHint    Intent = "hint"    // general hint request
	IntentReview  Intent = "review"  // review current code
	IntentStuck   Intent = "stuck"   // explicitly stuck
	IntentNext    Intent = "next"    // what to do next
	IntentExplain Intent = "explain" // explain concept
)

// InterventionLevel represents the depth of assistance
type InterventionLevel int

const (
	L0Clarify           InterventionLevel = 0 // Clarifying question only
	L1CategoryHint      InterventionLevel = 1 // Category hint
	L2LocationConcept   InterventionLevel = 2 // Location + concept (no code)
	L3ConstrainedSnippet InterventionLevel = 3 // Constrained snippet/outline
	L4PartialSolution   InterventionLevel = 4 // Partial solution (gated)
	L5FullSolution      InterventionLevel = 5 // Full solution (rare)
)

// String returns the human-readable name of the intervention level
func (l InterventionLevel) String() string {
	switch l {
	case L0Clarify:
		return "clarify"
	case L1CategoryHint:
		return "category"
	case L2LocationConcept:
		return "location"
	case L3ConstrainedSnippet:
		return "snippet"
	case L4PartialSolution:
		return "partial"
	case L5FullSolution:
		return "solution"
	default:
		return "unknown"
	}
}

// Description returns a description of what this level provides
func (l InterventionLevel) Description() string {
	switch l {
	case L0Clarify:
		return "Ask a clarifying question to guide your thinking"
	case L1CategoryHint:
		return "Hint at the category or direction to explore"
	case L2LocationConcept:
		return "Point to the location and explain the concept"
	case L3ConstrainedSnippet:
		return "Provide a constrained snippet or outline"
	case L4PartialSolution:
		return "Show a partial solution with explanation"
	case L5FullSolution:
		return "Provide the complete solution"
	default:
		return "Unknown level"
	}
}

// InterventionType represents the form of guidance
type InterventionType string

const (
	TypeQuestion InterventionType = "question" // asking user a question
	TypeHint     InterventionType = "hint"     // subtle direction
	TypeNudge    InterventionType = "nudge"    // gentle push
	TypeCritique InterventionType = "critique" // code review feedback
	TypeExplain  InterventionType = "explain"  // conceptual explanation
	TypeSnippet  InterventionType = "snippet"  // code snippet
)

// Target represents a specific location in the code
type Target struct {
	File      string
	StartLine int
	EndLine   int
}

// LearningPolicy defines constraints on AI intervention
type LearningPolicy struct {
	MaxLevel        InterventionLevel // maximum allowed level
	PatchingEnabled bool              // whether code patches are allowed
	CooldownSeconds int               // minimum time between L3+ interventions
	Track           string            // "practice", "interview-prep"
}

// DefaultPolicy returns the default learning policy for practice mode
func DefaultPolicy() LearningPolicy {
	return LearningPolicy{
		MaxLevel:        L3ConstrainedSnippet,
		PatchingEnabled: false,
		CooldownSeconds: 60,
		Track:           "practice",
	}
}

// InterviewPrepPolicy returns a stricter policy for interview preparation
func InterviewPrepPolicy() LearningPolicy {
	return LearningPolicy{
		MaxLevel:        L2LocationConcept,
		PatchingEnabled: false,
		CooldownSeconds: 120,
		Track:           "interview-prep",
	}
}

// ClampLevel ensures the intervention level doesn't exceed the policy maximum
func (p LearningPolicy) ClampLevel(requested InterventionLevel) InterventionLevel {
	if requested > p.MaxLevel {
		return p.MaxLevel
	}
	return requested
}
