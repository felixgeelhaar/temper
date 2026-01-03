package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSkillEvaluator_EvaluateSkill_NilProfile(t *testing.T) {
	e := NewSkillEvaluator()
	assessment := e.EvaluateSkill(nil)

	if assessment.RecommendedLevel != L2LocationConcept {
		t.Errorf("nil profile should recommend L2, got %v", assessment.RecommendedLevel)
	}
}

func TestSkillEvaluator_EvaluateSkill_EmptyProfile(t *testing.T) {
	e := NewSkillEvaluator()
	profile := NewLearningProfile(uuid.New())

	assessment := e.EvaluateSkill(profile)

	if assessment.OverallLevel != 0.0 {
		t.Errorf("empty profile should have 0 overall level, got %v", assessment.OverallLevel)
	}
	if assessment.GrowthRate != GrowthSteady {
		t.Errorf("empty profile should have steady growth, got %v", assessment.GrowthRate)
	}
}

func TestSkillEvaluator_EvaluateSkill_WithSkills(t *testing.T) {
	e := NewSkillEvaluator()
	profile := &LearningProfile{
		ID:     uuid.New(),
		UserID: uuid.New(),
		TopicSkills: map[string]SkillLevel{
			"go/interfaces": {Level: 0.8, Attempts: 5, LastSeen: time.Now()},
			"go/goroutines": {Level: 0.6, Attempts: 3, LastSeen: time.Now()},
			"go/channels":   {Level: 0.3, Attempts: 2, LastSeen: time.Now()},
		},
		TotalRuns:    20,
		HintRequests: 4,
	}

	assessment := e.EvaluateSkill(profile)

	if assessment.OverallLevel <= 0 {
		t.Error("profile with skills should have positive overall level")
	}
	if len(assessment.StrongestTopics) == 0 {
		t.Error("should identify strongest topics")
	}
	if len(assessment.WeakestTopics) == 0 {
		t.Error("should identify weakest topics")
	}
}

func TestSkillEvaluator_CalculateOverallLevel(t *testing.T) {
	e := NewSkillEvaluator()

	tests := []struct {
		name    string
		profile *LearningProfile
		wantMin float64
		wantMax float64
	}{
		{
			name: "empty topics",
			profile: &LearningProfile{
				TopicSkills: map[string]SkillLevel{},
			},
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name: "single high skill",
			profile: &LearningProfile{
				TopicSkills: map[string]SkillLevel{
					"go/basics": {Level: 0.9, Attempts: 10, LastSeen: time.Now()},
				},
			},
			wantMin: 0.8,
			wantMax: 1.0,
		},
		{
			name: "mixed skills",
			profile: &LearningProfile{
				TopicSkills: map[string]SkillLevel{
					"go/basics":   {Level: 0.8, Attempts: 5, LastSeen: time.Now()},
					"go/advanced": {Level: 0.2, Attempts: 2, LastSeen: time.Now()},
				},
			},
			wantMin: 0.3,
			wantMax: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.calculateOverallLevel(tt.profile)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateOverallLevel() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSkillEvaluator_DetermineGrowthRate(t *testing.T) {
	e := NewSkillEvaluator()

	tests := []struct {
		name         string
		profile      *LearningProfile
		want         SkillGrowthRate
	}{
		{
			name: "not enough data",
			profile: &LearningProfile{
				TotalRuns: 5,
			},
			want: GrowthSteady,
		},
		{
			name: "rapid growth",
			profile: &LearningProfile{
				TopicSkills: map[string]SkillLevel{
					"go/basics": {Level: 0.85, Attempts: 10, LastSeen: time.Now()},
				},
				TotalRuns:    20,
				HintRequests: 2,
			},
			want: GrowthRapid,
		},
		{
			name: "slow growth - high dependency",
			profile: &LearningProfile{
				TopicSkills: map[string]SkillLevel{
					"go/basics": {Level: 0.2, Attempts: 5, LastSeen: time.Now()},
				},
				TotalRuns:    20,
				HintRequests: 15,
			},
			want: GrowthSlow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.determineGrowthRate(tt.profile)
			if got != tt.want {
				t.Errorf("determineGrowthRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSkillEvaluator_AnalyzeTopics(t *testing.T) {
	e := NewSkillEvaluator()

	profile := &LearningProfile{
		TopicSkills: map[string]SkillLevel{
			"go/interfaces": {Level: 0.9, Attempts: 5, LastSeen: time.Now()},
			"go/goroutines": {Level: 0.7, Attempts: 3, LastSeen: time.Now()},
			"go/channels":   {Level: 0.5, Attempts: 4, LastSeen: time.Now()},
			"go/generics":   {Level: 0.3, Attempts: 2, LastSeen: time.Now()},
			"go/testing":    {Level: 0.2, Attempts: 1, LastSeen: time.Now()},
		},
	}

	strongest, weakest := e.analyzeTopics(profile)

	if len(strongest) != 3 {
		t.Errorf("expected 3 strongest topics, got %d", len(strongest))
	}
	if len(weakest) != 3 {
		t.Errorf("expected 3 weakest topics, got %d", len(weakest))
	}

	// Verify strongest contains high-skill topics
	if strongest[0] != "go/interfaces" {
		t.Errorf("expected go/interfaces as top, got %s", strongest[0])
	}
}

func TestSkillEvaluator_CalculateSkillDelta(t *testing.T) {
	e := NewSkillEvaluator()

	tests := []struct {
		name       string
		level      float64
		success    bool
		difficulty Difficulty
		wantSign   int // 1 for positive, -1 for negative
	}{
		{"success beginner", 0.5, true, DifficultyBeginner, 1},
		{"success advanced", 0.5, true, DifficultyAdvanced, 1},
		{"failure beginner", 0.5, false, DifficultyBeginner, -1},
		{"failure advanced", 0.5, false, DifficultyAdvanced, -1},
		{"success at high level", 0.9, true, DifficultyIntermediate, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.CalculateSkillDelta(tt.level, tt.success, tt.difficulty)

			if tt.wantSign > 0 && got <= 0 {
				t.Errorf("expected positive delta, got %v", got)
			}
			if tt.wantSign < 0 && got >= 0 {
				t.Errorf("expected negative delta, got %v", got)
			}
		})
	}

	// Verify diminishing returns at high skill
	highDelta := e.CalculateSkillDelta(0.9, true, DifficultyIntermediate)
	lowDelta := e.CalculateSkillDelta(0.5, true, DifficultyIntermediate)
	if highDelta >= lowDelta {
		t.Error("expected smaller delta at high skill level")
	}

	// Verify advanced gives more delta
	advDelta := e.CalculateSkillDelta(0.5, true, DifficultyAdvanced)
	begDelta := e.CalculateSkillDelta(0.5, true, DifficultyBeginner)
	if advDelta <= begDelta {
		t.Error("expected larger delta for advanced difficulty")
	}
}

func TestSkillEvaluator_ShouldSuggestBreak(t *testing.T) {
	e := NewSkillEvaluator()

	tests := []struct {
		name    string
		signals *ProfileSignals
		want    bool
	}{
		{"nil signals", nil, false},
		{"fresh session", &ProfileSignals{RunsThisSession: 2}, false},
		{"many runs", &ProfileSignals{RunsThisSession: 15}, true},
		{"long session", &ProfileSignals{TimeOnExercise: 45 * time.Minute}, true},
		{"frustrated", &ProfileSignals{
			RunsThisSession:   7,
			ErrorsEncountered: []string{"e1", "e2", "e3", "e4", "e5", "e6"},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.ShouldSuggestBreak(tt.signals)
			if got != tt.want {
				t.Errorf("ShouldSuggestBreak() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSkillEvaluator_EscalationThreshold(t *testing.T) {
	e := NewSkillEvaluator()

	tests := []struct {
		name       string
		profile    *LearningProfile
		difficulty Difficulty
		want       int
	}{
		{"nil profile beginner", nil, DifficultyBeginner, 4},
		{"nil profile advanced", nil, DifficultyAdvanced, 2},
		{"nil profile intermediate", nil, DifficultyIntermediate, 3},
		{
			"high dependency",
			&LearningProfile{TotalRuns: 10, HintRequests: 8},
			DifficultyIntermediate,
			2, // threshold reduced
		},
		{
			"low dependency",
			&LearningProfile{TotalRuns: 10, HintRequests: 1},
			DifficultyIntermediate,
			3, // standard threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.EscalationThreshold(tt.profile, tt.difficulty)
			if got != tt.want {
				t.Errorf("EscalationThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSkillEvaluator_IsReadyForAdvancement(t *testing.T) {
	e := NewSkillEvaluator()

	tests := []struct {
		name       string
		assessment *SkillAssessment
		want       bool
	}{
		{
			name: "ready",
			assessment: &SkillAssessment{
				OverallLevel:   0.75,
				HintDependency: 0.2,
				GrowthRate:     GrowthRapid,
			},
			want: true,
		},
		{
			name: "low skill",
			assessment: &SkillAssessment{
				OverallLevel:   0.4,
				HintDependency: 0.1,
				GrowthRate:     GrowthSteady,
			},
			want: false,
		},
		{
			name: "high dependency",
			assessment: &SkillAssessment{
				OverallLevel:   0.8,
				HintDependency: 0.5,
				GrowthRate:     GrowthSteady,
			},
			want: false,
		},
		{
			name: "plateaued",
			assessment: &SkillAssessment{
				OverallLevel:   0.7,
				HintDependency: 0.2,
				GrowthRate:     GrowthPlateaued,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.isReadyForAdvancement(tt.assessment)
			if got != tt.want {
				t.Errorf("isReadyForAdvancement() = %v, want %v", got, tt.want)
			}
		})
	}
}
