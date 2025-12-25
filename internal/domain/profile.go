package domain

import (
	"time"

	"github.com/google/uuid"
)

// LearningProfile tracks a user's learning progress and patterns
type LearningProfile struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	TopicSkills    map[string]SkillLevel // "go/interfaces" -> level
	TotalExercises int                   // exercises attempted
	TotalRuns      int                   // total code runs
	HintRequests   int                   // total hints requested
	AvgTimeToGreen time.Duration         // average time to pass tests
	CommonErrors   []string              // frequently seen error patterns
	UpdatedAt      time.Time
}

// SkillLevel represents proficiency in a specific topic
type SkillLevel struct {
	Level      float64   // 0.0 - 1.0
	Attempts   int       // number of exercises attempted
	LastSeen   time.Time // last activity in this topic
	Confidence float64   // self-reported confidence
}

// NewLearningProfile creates a new learning profile for a user
func NewLearningProfile(userID uuid.UUID) *LearningProfile {
	return &LearningProfile{
		ID:          uuid.New(),
		UserID:      userID,
		TopicSkills: make(map[string]SkillLevel),
		UpdatedAt:   time.Now(),
	}
}

// GetSkillLevel returns the skill level for a topic
func (p *LearningProfile) GetSkillLevel(topic string) SkillLevel {
	if skill, ok := p.TopicSkills[topic]; ok {
		return skill
	}
	return SkillLevel{Level: 0.0}
}

// UpdateSkill updates the skill level for a topic
func (p *LearningProfile) UpdateSkill(topic string, success bool) {
	skill := p.GetSkillLevel(topic)
	skill.Attempts++
	skill.LastSeen = time.Now()

	// Simple skill adjustment
	if success {
		skill.Level = min(1.0, skill.Level+0.05)
	} else {
		skill.Level = max(0.0, skill.Level-0.02)
	}

	p.TopicSkills[topic] = skill
	p.UpdatedAt = time.Now()
}

// RecordRun updates statistics for a run
func (p *LearningProfile) RecordRun(success bool, duration time.Duration) {
	p.TotalRuns++

	// Update average time to green
	if success && duration > 0 {
		if p.AvgTimeToGreen == 0 {
			p.AvgTimeToGreen = duration
		} else {
			// Exponential moving average
			p.AvgTimeToGreen = (p.AvgTimeToGreen*9 + duration) / 10
		}
	}

	p.UpdatedAt = time.Now()
}

// RecordHint records that a hint was requested
func (p *LearningProfile) RecordHint() {
	p.HintRequests++
	p.UpdatedAt = time.Now()
}

// HintDependency calculates a metric for hint dependency (0-1)
// Higher values indicate more reliance on hints
func (p *LearningProfile) HintDependency() float64 {
	if p.TotalRuns == 0 {
		return 0.0
	}
	// Ratio of hints to runs, capped at 1.0
	return min(1.0, float64(p.HintRequests)/float64(p.TotalRuns))
}

// SuggestMaxLevel suggests an appropriate max intervention level based on profile
func (p *LearningProfile) SuggestMaxLevel() InterventionLevel {
	dependency := p.HintDependency()

	// If very dependent on hints, don't lower the cap
	if dependency > 0.5 {
		return L3ConstrainedSnippet
	}

	// If moderately dependent, cap at L2
	if dependency > 0.3 {
		return L2LocationConcept
	}

	// If independent, suggest lower levels
	return L1CategoryHint
}

// ProfileSignals represents real-time signals about learning behavior
type ProfileSignals struct {
	RunsThisSession    int
	HintsThisSession   int
	TimeOnExercise     time.Duration
	ErrorsEncountered  []string
	LastInterventionAt *time.Time
}

// NeedsIntervention suggests if user might benefit from intervention
func (s *ProfileSignals) NeedsIntervention() bool {
	// Suggest intervention if:
	// - Many runs without success
	// - Long time on exercise
	// - Repeated same errors
	return s.RunsThisSession > 5 ||
		s.TimeOnExercise > 10*time.Minute ||
		len(s.ErrorsEncountered) > 3
}
