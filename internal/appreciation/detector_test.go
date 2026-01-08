package appreciation

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("NewDetector() returned nil")
	}
	if d.minSessionsForTrend != 5 {
		t.Errorf("minSessionsForTrend = %d, want 5", d.minSessionsForTrend)
	}
	if d.dependencyThreshold != 0.2 {
		t.Errorf("dependencyThreshold = %f, want 0.2", d.dependencyThreshold)
	}
	if d.quickResolutionMins != 10 {
		t.Errorf("quickResolutionMins = %d, want 10", d.quickResolutionMins)
	}
	if d.minimalHintThreshold != 2 {
		t.Errorf("minimalHintThreshold = %d, want 2", d.minimalHintThreshold)
	}
}

func TestDetector_DetectSessionMoments(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name        string
		session     *session.Session
		output      *domain.RunOutput
		wantTypes   []MomentType
		wantAtLeast int
	}{
		{
			name: "no hints needed",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 0,
				RunCount:  3,
				Status:    session.StatusActive,
				CreatedAt: time.Now(),
			},
			output:      nil,
			wantTypes:   []MomentType{MomentNoHintsNeeded},
			wantAtLeast: 1,
		},
		{
			name: "minimal hints",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 2,
				RunCount:  5,
				Status:    session.StatusActive,
				CreatedAt: time.Now(),
			},
			output:      nil,
			wantTypes:   []MomentType{MomentMinimalHints, MomentNoEscalation},
			wantAtLeast: 2,
		},
		{
			name: "first try success",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 0,
				RunCount:  1,
				Status:    session.StatusActive,
				CreatedAt: time.Now(),
			},
			output: &domain.RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			wantTypes:   []MomentType{MomentNoHintsNeeded, MomentFirstTrySuccess},
			wantAtLeast: 2,
		},
		{
			name: "all tests passing on completion",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 1,
				RunCount:  3,
				Status:    session.StatusCompleted,
				CreatedAt: time.Now(),
			},
			output: &domain.RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			wantTypes:   []MomentType{MomentMinimalHints, MomentNoEscalation, MomentAllTestsPassing},
			wantAtLeast: 3,
		},
		{
			name: "quick resolution",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 0,
				RunCount:  2,
				Status:    session.StatusCompleted,
				CreatedAt: time.Now().Add(-5 * time.Minute), // Started 5 mins ago
			},
			output:      nil,
			wantTypes:   []MomentType{MomentNoHintsNeeded, MomentQuickResolution},
			wantAtLeast: 2,
		},
		{
			name: "no moments for active with hints",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 5,
				RunCount:  10,
				Status:    session.StatusActive,
				CreatedAt: time.Now().Add(-30 * time.Minute),
			},
			output:      nil,
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moments := d.DetectSessionMoments(tt.session, tt.output)

			if len(moments) < tt.wantAtLeast {
				t.Errorf("DetectSessionMoments() got %d moments, want at least %d", len(moments), tt.wantAtLeast)
			}

			for _, wantType := range tt.wantTypes {
				found := false
				for _, m := range moments {
					if m.Type == wantType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DetectSessionMoments() missing moment type %v", wantType)
				}
			}
		})
	}
}

func TestDetector_DetectProfileMoments(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name        string
		profile     *domain.LearningProfile
		previous    *domain.LearningProfile
		wantTypes   []MomentType
		wantAtLeast int
	}{
		{
			name:        "nil profile",
			profile:     nil,
			previous:    nil,
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
		{
			name: "reduced dependency",
			profile: &domain.LearningProfile{
				HintRequests: 10,
				TotalRuns:    100,
			},
			previous: &domain.LearningProfile{
				HintRequests: 30,
				TotalRuns:    100,
			},
			wantTypes:   []MomentType{MomentReducedDependency, MomentConsistentSuccess},
			wantAtLeast: 1,
		},
		{
			name: "consistent success",
			profile: &domain.LearningProfile{
				HintRequests: 1,
				TotalRuns:    10,
			},
			previous:    nil,
			wantTypes:   []MomentType{MomentConsistentSuccess},
			wantAtLeast: 1,
		},
		{
			name: "no improvement",
			profile: &domain.LearningProfile{
				HintRequests: 50,
				TotalRuns:    100,
			},
			previous: &domain.LearningProfile{
				HintRequests: 50,
				TotalRuns:    100,
			},
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moments := d.DetectProfileMoments(tt.profile, tt.previous)

			if len(moments) < tt.wantAtLeast {
				t.Errorf("DetectProfileMoments() got %d moments, want at least %d", len(moments), tt.wantAtLeast)
			}

			for _, wantType := range tt.wantTypes {
				found := false
				for _, m := range moments {
					if m.Type == wantType {
						found = true
						break
					}
				}
				if !found && len(tt.wantTypes) > 0 {
					t.Errorf("DetectProfileMoments() missing moment type %v, got %v", wantType, moments)
				}
			}
		})
	}
}

func TestDetector_DetectTopicMoments(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name        string
		skill       *Skill
		isFirst     bool
		wantTypes   []MomentType
		wantAtLeast int
	}{
		{
			name:        "nil skill",
			skill:       nil,
			isFirst:     false,
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
		{
			name: "first in topic",
			skill: &Skill{
				Topic:              "functions",
				Level:              1,
				ExercisesCompleted: 1,
			},
			isFirst:     true,
			wantTypes:   []MomentType{MomentFirstInTopic},
			wantAtLeast: 1,
		},
		{
			name: "topic mastery",
			skill: &Skill{
				Topic:              "testing",
				Level:              4,
				ExercisesCompleted: 10,
			},
			isFirst:     false,
			wantTypes:   []MomentType{MomentTopicMastery},
			wantAtLeast: 1,
		},
		{
			name: "first and mastery",
			skill: &Skill{
				Topic:              "advanced",
				Level:              5,
				ExercisesCompleted: 15,
			},
			isFirst:     true,
			wantTypes:   []MomentType{MomentFirstInTopic, MomentTopicMastery},
			wantAtLeast: 2,
		},
		{
			name: "regular progress",
			skill: &Skill{
				Topic:              "basics",
				Level:              2,
				ExercisesCompleted: 3,
			},
			isFirst:     false,
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moments := d.DetectTopicMoments(tt.skill, tt.isFirst)

			if len(moments) < tt.wantAtLeast {
				t.Errorf("DetectTopicMoments() got %d moments, want at least %d", len(moments), tt.wantAtLeast)
			}

			for _, wantType := range tt.wantTypes {
				found := false
				for _, m := range moments {
					if m.Type == wantType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DetectTopicMoments() missing moment type %v", wantType)
				}
			}
		})
	}
}

func TestDetector_DetectSpecMoments(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name        string
		spec        *domain.ProductSpec
		criterion   *domain.AcceptanceCriterion
		wantTypes   []MomentType
		wantAtLeast int
	}{
		{
			name:        "nil spec",
			spec:        nil,
			criterion:   nil,
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
		{
			name: "criterion satisfied",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: true},
					{ID: "AC-002", Satisfied: false},
				},
			},
			criterion: &domain.AcceptanceCriterion{
				ID:          "AC-001",
				Description: "User can log in",
				Satisfied:   true,
			},
			wantTypes:   []MomentType{MomentCriterionSatisfied},
			wantAtLeast: 1,
		},
		{
			name: "spec complete",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: true},
					{ID: "AC-002", Satisfied: true},
				},
			},
			criterion:   nil,
			wantTypes:   []MomentType{MomentSpecComplete},
			wantAtLeast: 1,
		},
		{
			name: "criterion and spec complete",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: true},
				},
			},
			criterion: &domain.AcceptanceCriterion{
				ID:        "AC-001",
				Satisfied: true,
			},
			wantTypes:   []MomentType{MomentCriterionSatisfied, MomentSpecComplete},
			wantAtLeast: 2,
		},
		{
			name: "spec not complete",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: true},
					{ID: "AC-002", Satisfied: false},
				},
			},
			criterion:   nil,
			wantTypes:   []MomentType{},
			wantAtLeast: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moments := d.DetectSpecMoments(tt.spec, tt.criterion)

			if len(moments) < tt.wantAtLeast {
				t.Errorf("DetectSpecMoments() got %d moments, want at least %d", len(moments), tt.wantAtLeast)
			}

			for _, wantType := range tt.wantTypes {
				found := false
				for _, m := range moments {
					if m.Type == wantType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DetectSpecMoments() missing moment type %v", wantType)
				}
			}
		})
	}
}

func TestDetector_SelectBest(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		moments  []Moment
		wantType MomentType
		wantNil  bool
	}{
		{
			name:    "empty moments",
			moments: []Moment{},
			wantNil: true,
		},
		{
			name: "single moment",
			moments: []Moment{
				{Type: MomentMinimalHints},
			},
			wantType: MomentMinimalHints,
		},
		{
			name: "spec complete highest priority",
			moments: []Moment{
				{Type: MomentMinimalHints},
				{Type: MomentSpecComplete},
				{Type: MomentFirstTrySuccess},
			},
			wantType: MomentSpecComplete,
		},
		{
			name: "topic mastery over session moments",
			moments: []Moment{
				{Type: MomentAllTestsPassing},
				{Type: MomentTopicMastery},
				{Type: MomentQuickResolution},
			},
			wantType: MomentTopicMastery,
		},
		{
			name: "first try over no hints",
			moments: []Moment{
				{Type: MomentNoHintsNeeded},
				{Type: MomentFirstTrySuccess},
			},
			wantType: MomentFirstTrySuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.SelectBest(tt.moments)

			if tt.wantNil {
				if result != nil {
					t.Errorf("SelectBest() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("SelectBest() returned nil, want moment")
			}

			if result.Type != tt.wantType {
				t.Errorf("SelectBest().Type = %v, want %v", result.Type, tt.wantType)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"under a minute", 30 * time.Second, "under a minute"},
		{"exactly one minute", time.Minute, "1 minute"},
		{"two minutes", 2 * time.Minute, "2 minutes"},
		{"five minutes", 5 * time.Minute, "5 minutes"},
		{"ten minutes", 10 * time.Minute, "10 minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestMomentType_Constants(t *testing.T) {
	// Verify all moment type constants are defined correctly
	types := []MomentType{
		MomentNoHintsNeeded,
		MomentMinimalHints,
		MomentNoEscalation,
		MomentFirstTrySuccess,
		MomentQuickResolution,
		MomentAllTestsPassing,
		MomentReducedDependency,
		MomentTopicMastery,
		MomentConsistentSuccess,
		MomentFirstInTopic,
		MomentCriterionSatisfied,
		MomentSpecComplete,
	}

	for _, mt := range types {
		if mt == "" {
			t.Error("moment type should not be empty")
		}
	}
}

func TestEvidence_Fields(t *testing.T) {
	e := Evidence{
		HintCount:          2,
		RunCount:           5,
		MaxLevelUsed:       3,
		Duration:           "5 minutes",
		PreviousDependency: 0.5,
		CurrentDependency:  0.3,
		ImprovementPercent: 40.0,
		TopicName:          "testing",
		ExercisesCompleted: 10,
		CriterionID:        "AC-001",
		CriterionDesc:      "User can log in",
		SpecName:           "user-auth",
		Progress:           "100%",
	}

	if e.HintCount != 2 {
		t.Errorf("HintCount = %d, want 2", e.HintCount)
	}
	if e.RunCount != 5 {
		t.Errorf("RunCount = %d, want 5", e.RunCount)
	}
	if e.MaxLevelUsed != 3 {
		t.Errorf("MaxLevelUsed = %d, want 3", e.MaxLevelUsed)
	}
	if e.Duration != "5 minutes" {
		t.Errorf("Duration = %q, want %q", e.Duration, "5 minutes")
	}
	if e.ImprovementPercent != 40.0 {
		t.Errorf("ImprovementPercent = %f, want 40.0", e.ImprovementPercent)
	}
	if e.TopicName != "testing" {
		t.Errorf("TopicName = %q, want %q", e.TopicName, "testing")
	}
	if e.SpecName != "user-auth" {
		t.Errorf("SpecName = %q, want %q", e.SpecName, "user-auth")
	}
}
