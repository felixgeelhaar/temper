package pairing

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestSelector_IntentToBaseLevel(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name   string
		intent domain.Intent
		want   domain.InterventionLevel
	}{
		{"hint returns L1", domain.IntentHint, domain.L1CategoryHint},
		{"review returns L2", domain.IntentReview, domain.L2LocationConcept},
		{"stuck returns L2", domain.IntentStuck, domain.L2LocationConcept},
		{"next returns L1", domain.IntentNext, domain.L1CategoryHint},
		{"explain returns L2", domain.IntentExplain, domain.L2LocationConcept},
		{"unknown returns L1", domain.Intent{}, domain.L1CategoryHint}, // Zero value for unknown
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.intentToBaseLevel(tt.intent)
			if got != tt.want {
				t.Errorf("intentToBaseLevel(%v) = %v, want %v", tt.intent, got, tt.want)
			}
		})
	}
}

func TestSelector_AdjustForContext(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name       string
		level      domain.InterventionLevel
		difficulty domain.Difficulty
		want       domain.InterventionLevel
	}{
		{
			name:       "beginner keeps level",
			level:      domain.L2LocationConcept,
			difficulty: domain.DifficultyBeginner,
			want:       domain.L2LocationConcept,
		},
		{
			name:       "intermediate keeps level",
			level:      domain.L2LocationConcept,
			difficulty: domain.DifficultyIntermediate,
			want:       domain.L2LocationConcept,
		},
		{
			name:       "advanced reduces level above L1",
			level:      domain.L2LocationConcept,
			difficulty: domain.DifficultyAdvanced,
			want:       domain.L1CategoryHint,
		},
		{
			name:       "advanced keeps L1",
			level:      domain.L1CategoryHint,
			difficulty: domain.DifficultyAdvanced,
			want:       domain.L1CategoryHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := InterventionContext{
				Exercise: &domain.Exercise{Difficulty: tt.difficulty},
			}
			got := s.adjustForContext(tt.level, ctx)
			if got != tt.want {
				t.Errorf("adjustForContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelector_AdjustForProfile(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name         string
		level        domain.InterventionLevel
		hintRequests int
		totalRuns    int
		want         domain.InterventionLevel
	}{
		{
			name:         "nil profile keeps level",
			level:        domain.L2LocationConcept,
			hintRequests: 0,
			totalRuns:    0,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "high dependency keeps level",
			level:        domain.L2LocationConcept,
			hintRequests: 8,
			totalRuns:    10,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "low dependency with experience reduces level",
			level:        domain.L2LocationConcept,
			hintRequests: 1,
			totalRuns:    20,
			want:         domain.L1CategoryHint,
		},
		{
			name:         "low dependency with low experience keeps level",
			level:        domain.L2LocationConcept,
			hintRequests: 0,
			totalRuns:    5,
			want:         domain.L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profile *domain.LearningProfile
			if tt.totalRuns > 0 || tt.hintRequests > 0 {
				profile = &domain.LearningProfile{
					HintRequests: tt.hintRequests,
					TotalRuns:    tt.totalRuns,
				}
			}
			got := s.adjustForProfile(tt.level, profile)
			if got != tt.want {
				t.Errorf("adjustForProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelector_AdjustForRunOutput(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name        string
		level       domain.InterventionLevel
		output      *domain.RunOutput
		want        domain.InterventionLevel
	}{
		{
			name:   "nil output keeps level",
			level:  domain.L2LocationConcept,
			output: nil,
			want:   domain.L2LocationConcept,
		},
		{
			name:  "all tests pass reduces to L0",
			level: domain.L2LocationConcept,
			output: &domain.RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			want: domain.L0Clarify,
		},
		{
			name:  "build errors increase to L2",
			level: domain.L1CategoryHint,
			output: &domain.RunOutput{
				BuildErrors: []domain.Diagnostic{{Severity: "error"}},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "many test failures increase to L2",
			level: domain.L1CategoryHint,
			output: &domain.RunOutput{
				TestsFailed: 5,
			},
			want: domain.L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.adjustForRunOutput(tt.level, tt.output)
			if got != tt.want {
				t.Errorf("adjustForRunOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelector_SelectType(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name   string
		intent domain.Intent
		level  domain.InterventionLevel
		want   domain.InterventionType
	}{
		{"L0 returns question", domain.IntentHint, domain.L0Clarify, domain.TypeQuestion},
		{"L1 returns hint", domain.IntentHint, domain.L1CategoryHint, domain.TypeHint},
		{"L2 review returns critique", domain.IntentReview, domain.L2LocationConcept, domain.TypeCritique},
		{"L2 non-review returns nudge", domain.IntentHint, domain.L2LocationConcept, domain.TypeNudge},
		{"L3 explain returns explain", domain.IntentExplain, domain.L3ConstrainedSnippet, domain.TypeExplain},
		{"L3 non-explain returns snippet", domain.IntentStuck, domain.L3ConstrainedSnippet, domain.TypeSnippet},
		{"L4 returns snippet", domain.IntentStuck, domain.L4PartialSolution, domain.TypeSnippet},
		{"L5 returns snippet", domain.IntentStuck, domain.L5FullSolution, domain.TypeSnippet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SelectType(tt.intent, tt.level)
			if got != tt.want {
				t.Errorf("SelectType(%v, %v) = %v, want %v", tt.intent, tt.level, got, tt.want)
			}
		})
	}
}

func TestSelector_ShouldEscalate(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name      string
		attempts  int
		lastLevel domain.InterventionLevel
		want      bool
	}{
		{"no escalate at 1 attempt", 1, domain.L1CategoryHint, false},
		{"no escalate at 2 attempts", 2, domain.L1CategoryHint, false},
		{"escalate at 3 attempts below L3", 3, domain.L1CategoryHint, true},
		{"no escalate at L3 even with 3 attempts", 3, domain.L3ConstrainedSnippet, false},
		{"no escalate at L4", 5, domain.L4PartialSolution, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.ShouldEscalate(tt.attempts, tt.lastLevel)
			if got != tt.want {
				t.Errorf("ShouldEscalate(%d, %v) = %v, want %v", tt.attempts, tt.lastLevel, got, tt.want)
			}
		})
	}
}

func TestSelector_AdjustForSpec(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name  string
		level domain.InterventionLevel
		ctx   InterventionContext
		want  domain.InterventionLevel
	}{
		{
			name:  "no spec keeps level",
			level: domain.L2LocationConcept,
			ctx:   InterventionContext{},
			want:  domain.L2LocationConcept,
		},
		{
			name:  "with focus criterion keeps level",
			level: domain.L2LocationConcept,
			ctx: InterventionContext{
				Spec:           &domain.ProductSpec{},
				FocusCriterion: &domain.AcceptanceCriterion{ID: "ac-1"},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "high progress reduces level above L1",
			level: domain.L2LocationConcept,
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: true},
						{ID: "ac-3", Satisfied: false},
					},
				},
			},
			want: domain.L1CategoryHint,
		},
		{
			name:  "low progress keeps level",
			level: domain.L2LocationConcept,
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: false},
						{ID: "ac-2", Satisfied: false},
						{ID: "ac-3", Satisfied: false},
					},
				},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "L1 keeps L1 even with progress",
			level: domain.L1CategoryHint,
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: true},
					},
				},
			},
			want: domain.L1CategoryHint,
		},
		{
			name:  "scope drift detected",
			level: domain.L2LocationConcept,
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					Features: []domain.Feature{
						{ID: "f1", API: &domain.APISpec{Path: "/api/v1"}},
					},
				},
			},
			want: domain.L2LocationConcept, // Level unchanged, but drift is detected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.adjustForSpec(tt.level, tt.ctx)
			if got != tt.want {
				t.Errorf("adjustForSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelector_SelectLevel_FullIntegration(t *testing.T) {
	s := NewSelector()
	policy := domain.LearningPolicy{}

	tests := []struct {
		name   string
		intent domain.Intent
		ctx    InterventionContext
		want   domain.InterventionLevel
	}{
		{
			name:   "hint with no context",
			intent: domain.IntentHint,
			ctx:    InterventionContext{},
			want:   domain.L1CategoryHint,
		},
		{
			name:   "stuck with passing tests gets L0",
			intent: domain.IntentStuck,
			ctx: InterventionContext{
				RunOutput: &domain.RunOutput{TestsPassed: 5, TestsFailed: 0},
			},
			want: domain.L0Clarify,
		},
		{
			name:   "review with advanced exercise",
			intent: domain.IntentReview,
			ctx: InterventionContext{
				Exercise: &domain.Exercise{Difficulty: domain.DifficultyAdvanced},
			},
			want: domain.L1CategoryHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SelectLevel(tt.intent, tt.ctx, policy)
			if got != tt.want {
				t.Errorf("SelectLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
