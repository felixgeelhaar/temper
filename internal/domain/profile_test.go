package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewLearningProfile(t *testing.T) {
	userID := uuid.New()
	profile := NewLearningProfile(userID)

	if profile == nil {
		t.Fatal("NewLearningProfile() returned nil")
	}
	if profile.ID == uuid.Nil {
		t.Error("NewLearningProfile() should generate ID")
	}
	if profile.UserID != userID {
		t.Errorf("UserID = %v, want %v", profile.UserID, userID)
	}
	if profile.TopicSkills == nil {
		t.Error("TopicSkills should be initialized")
	}
	if profile.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestLearningProfile_GetSkillLevel(t *testing.T) {
	profile := NewLearningProfile(uuid.New())

	t.Run("unknown topic returns zero", func(t *testing.T) {
		skill := profile.GetSkillLevel("unknown/topic")
		if skill.Level != 0.0 {
			t.Errorf("Level = %f, want 0.0", skill.Level)
		}
	})

	t.Run("known topic returns skill", func(t *testing.T) {
		profile.TopicSkills["go/interfaces"] = SkillLevel{
			Level:    0.7,
			Attempts: 5,
		}

		skill := profile.GetSkillLevel("go/interfaces")
		if skill.Level != 0.7 {
			t.Errorf("Level = %f, want 0.7", skill.Level)
		}
		if skill.Attempts != 5 {
			t.Errorf("Attempts = %d, want 5", skill.Attempts)
		}
	})
}

func TestLearningProfile_UpdateSkill(t *testing.T) {
	t.Run("success increases skill", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.TopicSkills["go/basics"] = SkillLevel{Level: 0.5}

		profile.UpdateSkill("go/basics", true)

		skill := profile.TopicSkills["go/basics"]
		if skill.Level != 0.55 {
			t.Errorf("Level = %f, want 0.55", skill.Level)
		}
		if skill.Attempts != 1 {
			t.Errorf("Attempts = %d, want 1", skill.Attempts)
		}
	})

	t.Run("failure decreases skill", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.TopicSkills["go/basics"] = SkillLevel{Level: 0.5}

		profile.UpdateSkill("go/basics", false)

		skill := profile.TopicSkills["go/basics"]
		if skill.Level != 0.48 {
			t.Errorf("Level = %f, want 0.48", skill.Level)
		}
	})

	t.Run("skill capped at 1.0", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.TopicSkills["go/basics"] = SkillLevel{Level: 0.99}

		profile.UpdateSkill("go/basics", true)

		skill := profile.TopicSkills["go/basics"]
		if skill.Level != 1.0 {
			t.Errorf("Level = %f, want 1.0 (capped)", skill.Level)
		}
	})

	t.Run("skill floored at 0.0", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.TopicSkills["go/basics"] = SkillLevel{Level: 0.01}

		profile.UpdateSkill("go/basics", false)

		skill := profile.TopicSkills["go/basics"]
		if skill.Level != 0.0 {
			t.Errorf("Level = %f, want 0.0 (floored)", skill.Level)
		}
	})

	t.Run("new topic starts at 0", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())

		profile.UpdateSkill("new/topic", true)

		skill := profile.TopicSkills["new/topic"]
		if skill.Level != 0.05 {
			t.Errorf("Level = %f, want 0.05", skill.Level)
		}
	})

	t.Run("updates timestamp", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		originalUpdate := profile.UpdatedAt

		time.Sleep(time.Millisecond)
		profile.UpdateSkill("go/basics", true)

		if !profile.UpdatedAt.After(originalUpdate) {
			t.Error("UpdatedAt should be updated")
		}
		if profile.TopicSkills["go/basics"].LastSeen.IsZero() {
			t.Error("LastSeen should be set")
		}
	})
}

func TestLearningProfile_RecordRun(t *testing.T) {
	t.Run("increments total runs", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.RecordRun(true, time.Second)
		profile.RecordRun(false, 0)

		if profile.TotalRuns != 2 {
			t.Errorf("TotalRuns = %d, want 2", profile.TotalRuns)
		}
	})

	t.Run("first success sets avg time", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.RecordRun(true, 5*time.Second)

		if profile.AvgTimeToGreen != 5*time.Second {
			t.Errorf("AvgTimeToGreen = %v, want 5s", profile.AvgTimeToGreen)
		}
	})

	t.Run("subsequent success updates avg", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.AvgTimeToGreen = 10 * time.Second

		profile.RecordRun(true, 20*time.Second)

		// EMA: (10*9 + 20) / 10 = 110/10 = 11
		expected := 11 * time.Second
		if profile.AvgTimeToGreen != expected {
			t.Errorf("AvgTimeToGreen = %v, want %v", profile.AvgTimeToGreen, expected)
		}
	})

	t.Run("failure does not update avg", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.AvgTimeToGreen = 10 * time.Second

		profile.RecordRun(false, 5*time.Second)

		if profile.AvgTimeToGreen != 10*time.Second {
			t.Errorf("AvgTimeToGreen = %v, want 10s (unchanged)", profile.AvgTimeToGreen)
		}
	})

	t.Run("zero duration does not update avg", func(t *testing.T) {
		profile := NewLearningProfile(uuid.New())
		profile.AvgTimeToGreen = 10 * time.Second

		profile.RecordRun(true, 0)

		if profile.AvgTimeToGreen != 10*time.Second {
			t.Errorf("AvgTimeToGreen = %v, want 10s (unchanged)", profile.AvgTimeToGreen)
		}
	})
}

func TestLearningProfile_RecordHint(t *testing.T) {
	profile := NewLearningProfile(uuid.New())
	originalUpdate := profile.UpdatedAt

	time.Sleep(time.Millisecond)
	profile.RecordHint()
	profile.RecordHint()

	if profile.HintRequests != 2 {
		t.Errorf("HintRequests = %d, want 2", profile.HintRequests)
	}
	if !profile.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated")
	}
}

func TestLearningProfile_HintDependency(t *testing.T) {
	tests := []struct {
		name         string
		hintRequests int
		totalRuns    int
		want         float64
	}{
		{"no runs", 5, 0, 0.0},
		{"no hints", 0, 10, 0.0},
		{"50% dependency", 5, 10, 0.5},
		{"100% dependency", 10, 10, 1.0},
		{"over 100% capped", 20, 10, 1.0},
		{"low dependency", 1, 10, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &LearningProfile{
				HintRequests: tt.hintRequests,
				TotalRuns:    tt.totalRuns,
			}
			if got := profile.HintDependency(); got != tt.want {
				t.Errorf("HintDependency() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestLearningProfile_SuggestMaxLevel(t *testing.T) {
	tests := []struct {
		name       string
		dependency float64
		want       InterventionLevel
	}{
		{"very dependent", 0.6, L3ConstrainedSnippet},
		{"moderately dependent", 0.4, L2LocationConcept},
		{"independent", 0.2, L1CategoryHint},
		{"very independent", 0.0, L1CategoryHint},
		{"threshold 0.5", 0.5, L2LocationConcept},
		{"threshold 0.3", 0.3, L1CategoryHint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &LearningProfile{
				HintRequests: int(tt.dependency * 100),
				TotalRuns:    100,
			}
			if got := profile.SuggestMaxLevel(); got != tt.want {
				t.Errorf("SuggestMaxLevel() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestProfileSignals_NeedsIntervention(t *testing.T) {
	tests := []struct {
		name             string
		runsThisSession  int
		timeOnExercise   time.Duration
		errorsCount      int
		want             bool
	}{
		{"no signals", 0, 0, 0, false},
		{"few runs", 3, 0, 0, false},
		{"many runs", 6, 0, 0, true},
		{"long time", 0, 11 * time.Minute, 0, true},
		{"many errors", 0, 0, 4, true},
		{"threshold runs", 5, 0, 0, false},
		{"threshold time", 0, 10 * time.Minute, 0, false},
		{"threshold errors", 0, 0, 3, false},
		{"multiple signals", 6, 15 * time.Minute, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := &ProfileSignals{
				RunsThisSession:   tt.runsThisSession,
				TimeOnExercise:    tt.timeOnExercise,
				ErrorsEncountered: make([]string, tt.errorsCount),
			}
			if got := signals.NeedsIntervention(); got != tt.want {
				t.Errorf("NeedsIntervention() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfileSignals_Struct(t *testing.T) {
	now := time.Now()
	signals := &ProfileSignals{
		RunsThisSession:    5,
		HintsThisSession:   2,
		TimeOnExercise:     5 * time.Minute,
		ErrorsEncountered:  []string{"undefined: x", "syntax error"},
		LastInterventionAt: &now,
	}

	if signals.RunsThisSession != 5 {
		t.Errorf("RunsThisSession = %d, want 5", signals.RunsThisSession)
	}
	if signals.LastInterventionAt == nil {
		t.Error("LastInterventionAt should not be nil")
	}
}

func TestSkillLevel_Struct(t *testing.T) {
	now := time.Now()
	skill := SkillLevel{
		Level:      0.75,
		Attempts:   10,
		LastSeen:   now,
		Confidence: 0.8,
	}

	if skill.Level != 0.75 {
		t.Errorf("Level = %f, want 0.75", skill.Level)
	}
	if skill.Confidence != 0.8 {
		t.Errorf("Confidence = %f, want 0.8", skill.Confidence)
	}
}
