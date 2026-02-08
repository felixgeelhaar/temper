package domain

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Track represents a learning contract preset that controls intervention behavior.
// Tracks are configurable policy presets like "beginner", "standard", "advanced".
type Track struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description" yaml:"description"`
	Preset      string    `json:"preset" yaml:"preset"` // "beginner", "standard", "advanced", "custom"
	CreatedAt   time.Time `json:"created_at" yaml:"-"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"-"`

	// Policy controls
	MaxLevel        InterventionLevel `json:"max_level" yaml:"max_level"`
	CooldownSeconds int               `json:"cooldown_seconds" yaml:"cooldown_seconds"`
	PatchingEnabled bool              `json:"patching_enabled" yaml:"patching_enabled"`

	// Auto-progress rules
	AutoProgress AutoProgressRules `json:"auto_progress" yaml:"auto_progress"`
}

// AutoProgressRules define when a track should automatically adjust difficulty.
type AutoProgressRules struct {
	Enabled             bool    `json:"enabled" yaml:"enabled"`
	PromoteAfterStreak  int     `json:"promote_after_streak" yaml:"promote_after_streak"`   // Consecutive successes to reduce max_level
	DemoteAfterFailures int     `json:"demote_after_failures" yaml:"demote_after_failures"` // Consecutive failures to increase max_level
	MinSkillForPromote  float64 `json:"min_skill_for_promote" yaml:"min_skill_for_promote"` // Minimum skill level to allow promotion
}

// ToPolicy converts a Track to a LearningPolicy for use in sessions.
func (t *Track) ToPolicy() LearningPolicy {
	return LearningPolicy{
		MaxLevel:        t.MaxLevel,
		CooldownSeconds: t.CooldownSeconds,
		PatchingEnabled: t.PatchingEnabled,
		Track:           t.ID,
	}
}

// Validate checks track fields for correctness.
func (t *Track) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("track id is required")
	}
	if t.Name == "" {
		return fmt.Errorf("track name is required")
	}
	if t.MaxLevel < L0Clarify || t.MaxLevel > L5FullSolution {
		return fmt.Errorf("max_level must be between 0 and 5")
	}
	if t.CooldownSeconds < 0 {
		return fmt.Errorf("cooldown_seconds must be non-negative")
	}
	return nil
}

// MarshalYAML exports a track to YAML format for sharing.
func (t *Track) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(t)
}

// UnmarshalTrackYAML imports a track from YAML data.
func UnmarshalTrackYAML(data []byte) (*Track, error) {
	var track Track
	if err := yaml.Unmarshal(data, &track); err != nil {
		return nil, fmt.Errorf("parse track yaml: %w", err)
	}
	if err := track.Validate(); err != nil {
		return nil, err
	}
	return &track, nil
}

// BuiltinTracks returns the built-in track presets.
func BuiltinTracks() []*Track {
	now := time.Now()
	return []*Track{
		{
			ID:              "beginner",
			Name:            "Beginner",
			Description:     "Maximum guidance. Generous hints and code snippets with short cooldowns.",
			Preset:          "beginner",
			MaxLevel:        L4PartialSolution,
			CooldownSeconds: 30,
			PatchingEnabled: true,
			AutoProgress: AutoProgressRules{
				Enabled:             true,
				PromoteAfterStreak:  5,
				DemoteAfterFailures: 3,
				MinSkillForPromote:  0.3,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:              "standard",
			Name:            "Standard",
			Description:     "Balanced learning. Hints up to constrained snippets with moderate cooldowns.",
			Preset:          "standard",
			MaxLevel:        L3ConstrainedSnippet,
			CooldownSeconds: 60,
			PatchingEnabled: false,
			AutoProgress: AutoProgressRules{
				Enabled:             true,
				PromoteAfterStreak:  7,
				DemoteAfterFailures: 5,
				MinSkillForPromote:  0.5,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:              "advanced",
			Name:            "Advanced",
			Description:     "Minimal guidance. Only clarifying questions and category hints with long cooldowns.",
			Preset:          "advanced",
			MaxLevel:        L1CategoryHint,
			CooldownSeconds: 120,
			PatchingEnabled: false,
			AutoProgress: AutoProgressRules{
				Enabled:            false,
				PromoteAfterStreak: 0,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:              "interview-prep",
			Name:            "Interview Prep",
			Description:     "Strict mode for interview practice. Location hints only, long cooldowns.",
			Preset:          "interview-prep",
			MaxLevel:        L2LocationConcept,
			CooldownSeconds: 120,
			PatchingEnabled: false,
			AutoProgress: AutoProgressRules{
				Enabled: false,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// ShouldEvaluateAutoProgress evaluates whether a track's auto-progress rules
// suggest changing the max level based on recent session performance.
func (t *Track) ShouldEvaluateAutoProgress(consecutiveSuccesses, consecutiveFailures int, avgSkill float64) *InterventionLevel {
	if !t.AutoProgress.Enabled {
		return nil
	}

	// Check for promotion (lower the max level)
	if t.AutoProgress.PromoteAfterStreak > 0 &&
		consecutiveSuccesses >= t.AutoProgress.PromoteAfterStreak &&
		avgSkill >= t.AutoProgress.MinSkillForPromote &&
		t.MaxLevel > L0Clarify {
		newLevel := t.MaxLevel - 1
		return &newLevel
	}

	// Check for demotion (raise the max level)
	if t.AutoProgress.DemoteAfterFailures > 0 &&
		consecutiveFailures >= t.AutoProgress.DemoteAfterFailures &&
		t.MaxLevel < L5FullSolution {
		newLevel := t.MaxLevel + 1
		return &newLevel
	}

	return nil
}
