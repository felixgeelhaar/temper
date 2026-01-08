package appreciation

import (
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
)

// Skill represents skill information for appreciation (simplified from profile)
type Skill struct {
	Topic              string
	Level              int
	ExercisesCompleted int
}

// Moment represents an appreciation-worthy event
type Moment struct {
	Type      MomentType
	Evidence  Evidence
	Triggered time.Time
}

// MomentType categorizes appreciation moments
type MomentType string

const (
	// Session-level moments
	MomentNoHintsNeeded   MomentType = "no_hints_needed"
	MomentMinimalHints    MomentType = "minimal_hints"
	MomentNoEscalation    MomentType = "no_escalation"
	MomentFirstTrySuccess MomentType = "first_try_success"
	MomentQuickResolution MomentType = "quick_resolution"
	MomentAllTestsPassing MomentType = "all_tests_passing"

	// Progress-level moments
	MomentReducedDependency MomentType = "reduced_dependency"
	MomentTopicMastery      MomentType = "topic_mastery"
	MomentConsistentSuccess MomentType = "consistent_success"
	MomentFirstInTopic      MomentType = "first_in_topic"

	// Spec-level moments
	MomentCriterionSatisfied MomentType = "criterion_satisfied"
	MomentSpecComplete       MomentType = "spec_complete"

	// Session end moment
	MomentSessionEnd MomentType = "session_end"
)

// Evidence provides the data backing an appreciation moment
type Evidence struct {
	// Session metrics
	HintCount    int    `json:"hint_count,omitempty"`
	RunCount     int    `json:"run_count,omitempty"`
	MaxLevelUsed int    `json:"max_level_used,omitempty"`
	Duration     string `json:"duration,omitempty"`

	// Progress metrics
	PreviousDependency float64 `json:"previous_dependency,omitempty"`
	CurrentDependency  float64 `json:"current_dependency,omitempty"`
	ImprovementPercent float64 `json:"improvement_percent,omitempty"`

	// Topic/skill metrics
	TopicName          string `json:"topic_name,omitempty"`
	ExercisesCompleted int    `json:"exercises_completed,omitempty"`

	// Spec metrics
	CriterionID   string `json:"criterion_id,omitempty"`
	CriterionDesc string `json:"criterion_desc,omitempty"`
	SpecName      string `json:"spec_name,omitempty"`
	Progress      string `json:"progress,omitempty"`

	// Session end metrics
	SessionDuration string `json:"session_duration,omitempty"`
	SpecProgress    string `json:"spec_progress,omitempty"`
}

// SessionSummary provides a motivational summary when a session ends
type SessionSummary struct {
	Duration       string    `json:"duration"`
	RunCount       int       `json:"run_count"`
	HintCount      int       `json:"hint_count"`
	Intent         string    `json:"intent"`
	SpecPath       string    `json:"spec_path,omitempty"`
	SpecProgress   string    `json:"spec_progress,omitempty"`
	Message        string    `json:"message"`
	Accomplishment string    `json:"accomplishment,omitempty"`
	Evidence       *Evidence `json:"evidence,omitempty"`
}

// Detector identifies appreciation-worthy moments
type Detector struct {
	// Configuration
	minSessionsForTrend  int
	dependencyThreshold  float64
	quickResolutionMins  int
	minimalHintThreshold int
}

// NewDetector creates a new appreciation detector
func NewDetector() *Detector {
	return &Detector{
		minSessionsForTrend:  5,
		dependencyThreshold:  0.2, // 20% improvement triggers appreciation
		quickResolutionMins:  10,
		minimalHintThreshold: 2,
	}
}

// DetectSessionMoments finds appreciation moments from a completed session
func (d *Detector) DetectSessionMoments(sess *session.Session, output *domain.RunOutput) []Moment {
	var moments []Moment
	now := time.Now()

	// No hints needed
	if sess.HintCount == 0 && sess.RunCount > 0 {
		moments = append(moments, Moment{
			Type:      MomentNoHintsNeeded,
			Triggered: now,
			Evidence: Evidence{
				HintCount: 0,
				RunCount:  sess.RunCount,
			},
		})
	}

	// Minimal hints (1-2 hints for completion)
	if sess.HintCount > 0 && sess.HintCount <= d.minimalHintThreshold {
		moments = append(moments, Moment{
			Type:      MomentMinimalHints,
			Triggered: now,
			Evidence: Evidence{
				HintCount: sess.HintCount,
				RunCount:  sess.RunCount,
			},
		})
	}

	// No escalation detection based on hint count and policy
	// If only a few hints and stayed within reasonable bounds
	if sess.HintCount > 0 && sess.HintCount <= 3 {
		moments = append(moments, Moment{
			Type:      MomentNoEscalation,
			Triggered: now,
			Evidence: Evidence{
				MaxLevelUsed: int(sess.Policy.MaxLevel),
				HintCount:    sess.HintCount,
			},
		})
	}

	// First try success (all tests passed on first run)
	if output != nil && sess.RunCount == 1 && output.AllTestsPassed() {
		moments = append(moments, Moment{
			Type:      MomentFirstTrySuccess,
			Triggered: now,
			Evidence: Evidence{
				RunCount: 1,
			},
		})
	}

	// All tests passing (on any run)
	if output != nil && output.AllTestsPassed() && sess.Status == session.StatusCompleted {
		moments = append(moments, Moment{
			Type:      MomentAllTestsPassing,
			Triggered: now,
			Evidence: Evidence{
				RunCount:  sess.RunCount,
				HintCount: sess.HintCount,
			},
		})
	}

	// Quick resolution (completed within threshold)
	duration := time.Since(sess.CreatedAt)
	if duration.Minutes() < float64(d.quickResolutionMins) && sess.Status == session.StatusCompleted {
		moments = append(moments, Moment{
			Type:      MomentQuickResolution,
			Triggered: now,
			Evidence: Evidence{
				Duration: formatDuration(duration),
			},
		})
	}

	return moments
}

// DetectProfileMoments finds appreciation moments from profile changes
func (d *Detector) DetectProfileMoments(profile *domain.LearningProfile, previous *domain.LearningProfile) []Moment {
	var moments []Moment
	now := time.Now()

	if profile == nil {
		return moments
	}

	// Reduced dependency
	if previous != nil {
		currentDep := profile.HintDependency()
		previousDep := previous.HintDependency()

		if previousDep > 0 && currentDep < previousDep {
			improvement := (previousDep - currentDep) / previousDep
			if improvement >= d.dependencyThreshold {
				moments = append(moments, Moment{
					Type:      MomentReducedDependency,
					Triggered: now,
					Evidence: Evidence{
						PreviousDependency: previousDep,
						CurrentDependency:  currentDep,
						ImprovementPercent: improvement * 100,
					},
				})
			}
		}
	}

	// Consistent success (multiple runs without heavy hints)
	if profile.TotalRuns >= d.minSessionsForTrend {
		recentDependency := profile.HintDependency()
		if recentDependency < 0.3 { // Less than 30% dependency
			moments = append(moments, Moment{
				Type:      MomentConsistentSuccess,
				Triggered: now,
				Evidence: Evidence{
					CurrentDependency: recentDependency,
				},
			})
		}
	}

	return moments
}

// DetectTopicMoments finds appreciation moments for topic/skill achievements
func (d *Detector) DetectTopicMoments(skill *Skill, isFirst bool) []Moment {
	var moments []Moment
	now := time.Now()

	if skill == nil {
		return moments
	}

	// First exercise in a topic
	if isFirst {
		moments = append(moments, Moment{
			Type:      MomentFirstInTopic,
			Triggered: now,
			Evidence: Evidence{
				TopicName:          skill.Topic,
				ExercisesCompleted: 1,
			},
		})
	}

	// Topic mastery (all exercises in topic completed)
	if skill.Level >= 4 { // High skill level
		moments = append(moments, Moment{
			Type:      MomentTopicMastery,
			Triggered: now,
			Evidence: Evidence{
				TopicName:          skill.Topic,
				ExercisesCompleted: skill.ExercisesCompleted,
			},
		})
	}

	return moments
}

// DetectSpecMoments finds appreciation moments for spec progress
func (d *Detector) DetectSpecMoments(spec *domain.ProductSpec, criterion *domain.AcceptanceCriterion) []Moment {
	var moments []Moment
	now := time.Now()

	if spec == nil {
		return moments
	}

	// Criterion satisfied
	if criterion != nil && criterion.Satisfied {
		moments = append(moments, Moment{
			Type:      MomentCriterionSatisfied,
			Triggered: now,
			Evidence: Evidence{
				SpecName:      spec.Name,
				CriterionID:   criterion.ID,
				CriterionDesc: criterion.Description,
			},
		})
	}

	// Spec complete (all criteria satisfied)
	allSatisfied := true
	for _, ac := range spec.AcceptanceCriteria {
		if !ac.Satisfied {
			allSatisfied = false
			break
		}
	}

	if allSatisfied && len(spec.AcceptanceCriteria) > 0 {
		moments = append(moments, Moment{
			Type:      MomentSpecComplete,
			Triggered: now,
			Evidence: Evidence{
				SpecName: spec.Name,
				Progress: "100%",
			},
		})
	}

	return moments
}

// SelectBest picks the most significant moment to appreciate (avoid overwhelming)
func (d *Detector) SelectBest(moments []Moment) *Moment {
	if len(moments) == 0 {
		return nil
	}

	// Priority order (higher = more significant)
	priority := map[MomentType]int{
		MomentSpecComplete:       10,
		MomentTopicMastery:       9,
		MomentFirstTrySuccess:    8,
		MomentNoHintsNeeded:      7,
		MomentReducedDependency:  6,
		MomentCriterionSatisfied: 5,
		MomentNoEscalation:       4,
		MomentMinimalHints:       3,
		MomentConsistentSuccess:  3,
		MomentQuickResolution:    2,
		MomentAllTestsPassing:    1,
		MomentFirstInTopic:       1,
	}

	best := moments[0]
	bestPriority := priority[best.Type]

	for _, m := range moments[1:] {
		p := priority[m.Type]
		if p > bestPriority {
			best = m
			bestPriority = p
		}
	}

	return &best
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "under a minute"
	}
	mins := int(d.Minutes())
	if mins == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", mins)
}
