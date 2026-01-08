package pairing

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

func TestNewSelector(t *testing.T) {
	s := NewSelector()
	if s == nil {
		t.Fatal("NewSelector() returned nil")
	}
}

func TestSelector_intentToBaseLevel(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name   string
		intent domain.Intent
		want   domain.InterventionLevel
	}{
		{
			name:   "IntentHint maps to L1",
			intent: domain.IntentHint,
			want:   domain.L1CategoryHint,
		},
		{
			name:   "IntentReview maps to L2",
			intent: domain.IntentReview,
			want:   domain.L2LocationConcept,
		},
		{
			name:   "IntentStuck maps to L2",
			intent: domain.IntentStuck,
			want:   domain.L2LocationConcept,
		},
		{
			name:   "IntentNext maps to L1",
			intent: domain.IntentNext,
			want:   domain.L1CategoryHint,
		},
		{
			name:   "IntentExplain maps to L2",
			intent: domain.IntentExplain,
			want:   domain.L2LocationConcept,
		},
		{
			name:   "unknown intent defaults to L1",
			intent: domain.Intent("unknown"),
			want:   domain.L1CategoryHint,
		},
		{
			name:   "empty intent defaults to L1",
			intent: domain.Intent(""),
			want:   domain.L1CategoryHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.intentToBaseLevel(tt.intent)
			if got != tt.want {
				t.Errorf("intentToBaseLevel(%q) = %v, want %v", tt.intent, got, tt.want)
			}
		})
	}
}

func TestSelector_adjustForContext(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name       string
		level      domain.InterventionLevel
		difficulty domain.Difficulty
		want       domain.InterventionLevel
	}{
		{
			name:       "nil exercise returns unchanged level",
			level:      domain.L2LocationConcept,
			difficulty: "", // will use nil exercise
			want:       domain.L2LocationConcept,
		},
		{
			name:       "beginner difficulty unchanged",
			level:      domain.L2LocationConcept,
			difficulty: domain.DifficultyBeginner,
			want:       domain.L2LocationConcept,
		},
		{
			name:       "intermediate difficulty unchanged",
			level:      domain.L2LocationConcept,
			difficulty: domain.DifficultyIntermediate,
			want:       domain.L2LocationConcept,
		},
		{
			name:       "advanced difficulty decrements L2 to L1",
			level:      domain.L2LocationConcept,
			difficulty: domain.DifficultyAdvanced,
			want:       domain.L1CategoryHint,
		},
		{
			name:       "advanced difficulty decrements L3 to L2",
			level:      domain.L3ConstrainedSnippet,
			difficulty: domain.DifficultyAdvanced,
			want:       domain.L2LocationConcept,
		},
		{
			name:       "advanced with L1 stays at L1",
			level:      domain.L1CategoryHint,
			difficulty: domain.DifficultyAdvanced,
			want:       domain.L1CategoryHint,
		},
		{
			name:       "advanced with L0 stays at L0",
			level:      domain.L0Clarify,
			difficulty: domain.DifficultyAdvanced,
			want:       domain.L0Clarify,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx InterventionContext
			if tt.difficulty != "" {
				ctx.Exercise = &domain.Exercise{
					Difficulty: tt.difficulty,
				}
			}

			got := s.adjustForContext(tt.level, ctx)
			if got != tt.want {
				t.Errorf("adjustForContext(%v, difficulty=%q) = %v, want %v",
					tt.level, tt.difficulty, got, tt.want)
			}
		})
	}
}

func TestSelector_adjustForProfile(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name         string
		level        domain.InterventionLevel
		hintRequests int
		totalRuns    int
		want         domain.InterventionLevel
	}{
		{
			name:         "nil profile returns unchanged level",
			level:        domain.L2LocationConcept,
			hintRequests: -1, // signals nil profile
			totalRuns:    -1,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "high dependency (>0.5) keeps level unchanged",
			level:        domain.L2LocationConcept,
			hintRequests: 60,
			totalRuns:    100,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "exactly 0.5 dependency keeps level unchanged",
			level:        domain.L2LocationConcept,
			hintRequests: 50,
			totalRuns:    100,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "low dependency (<0.2) with many runs (>10) decrements level",
			level:        domain.L2LocationConcept,
			hintRequests: 1,
			totalRuns:    15,
			want:         domain.L1CategoryHint,
		},
		{
			name:         "low dependency but few runs keeps level",
			level:        domain.L2LocationConcept,
			hintRequests: 1,
			totalRuns:    8,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "exactly 10 runs with low dependency keeps level",
			level:        domain.L2LocationConcept,
			hintRequests: 1,
			totalRuns:    10,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "L0 with low dependency stays at L0",
			level:        domain.L0Clarify,
			hintRequests: 1,
			totalRuns:    20,
			want:         domain.L0Clarify,
		},
		{
			name:         "zero runs returns level unchanged",
			level:        domain.L2LocationConcept,
			hintRequests: 0,
			totalRuns:    0,
			want:         domain.L2LocationConcept,
		},
		{
			name:         "moderate dependency (0.3) keeps level",
			level:        domain.L2LocationConcept,
			hintRequests: 30,
			totalRuns:    100,
			want:         domain.L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profile *domain.LearningProfile
			if tt.hintRequests >= 0 {
				profile = &domain.LearningProfile{
					ID:           uuid.New(),
					HintRequests: tt.hintRequests,
					TotalRuns:    tt.totalRuns,
				}
			}

			got := s.adjustForProfile(tt.level, profile)
			if got != tt.want {
				t.Errorf("adjustForProfile(%v, hints=%d, runs=%d) = %v, want %v",
					tt.level, tt.hintRequests, tt.totalRuns, got, tt.want)
			}
		})
	}
}

func TestSelector_adjustForRunOutput(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name        string
		level       domain.InterventionLevel
		output      *domain.RunOutput
		want        domain.InterventionLevel
	}{
		{
			name:   "nil output returns unchanged level",
			level:  domain.L2LocationConcept,
			output: nil,
			want:   domain.L2LocationConcept,
		},
		{
			name:  "all tests passed drops to L0",
			level: domain.L2LocationConcept,
			output: &domain.RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			want: domain.L0Clarify,
		},
		{
			name:  "all tests passed from L3 drops to L0",
			level: domain.L3ConstrainedSnippet,
			output: &domain.RunOutput{
				TestsPassed: 3,
				TestsFailed: 0,
			},
			want: domain.L0Clarify,
		},
		{
			name:  "L0 with all tests passed stays at L0",
			level: domain.L0Clarify,
			output: &domain.RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			want: domain.L0Clarify,
		},
		{
			name:  "build errors with L1 escalates to L2",
			level: domain.L1CategoryHint,
			output: &domain.RunOutput{
				BuildErrors: []domain.Diagnostic{
					{Message: "undefined: foo"},
				},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "build errors with L0 escalates to L2",
			level: domain.L0Clarify,
			output: &domain.RunOutput{
				BuildErrors: []domain.Diagnostic{
					{Message: "syntax error"},
				},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "build errors with L2 stays at L2",
			level: domain.L2LocationConcept,
			output: &domain.RunOutput{
				BuildErrors: []domain.Diagnostic{
					{Message: "type mismatch"},
				},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "build errors with L3 stays at L3",
			level: domain.L3ConstrainedSnippet,
			output: &domain.RunOutput{
				BuildErrors: []domain.Diagnostic{
					{Message: "undefined"},
				},
			},
			want: domain.L3ConstrainedSnippet,
		},
		{
			name:  "many tests failed (>3) with L1 escalates to L2",
			level: domain.L1CategoryHint,
			output: &domain.RunOutput{
				TestsFailed: 4,
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "exactly 3 tests failed keeps level",
			level: domain.L1CategoryHint,
			output: &domain.RunOutput{
				TestsFailed: 3,
			},
			want: domain.L1CategoryHint,
		},
		{
			name:  "many tests failed but already L2 stays",
			level: domain.L2LocationConcept,
			output: &domain.RunOutput{
				TestsFailed: 5,
			},
			want: domain.L2LocationConcept,
		},
		{
			name:  "some tests failed without build errors",
			level: domain.L1CategoryHint,
			output: &domain.RunOutput{
				TestsPassed: 2,
				TestsFailed: 2,
			},
			want: domain.L1CategoryHint,
		},
		{
			name:  "no tests run keeps level",
			level: domain.L2LocationConcept,
			output: &domain.RunOutput{
				TestsPassed: 0,
				TestsFailed: 0,
			},
			want: domain.L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.adjustForRunOutput(tt.level, tt.output)
			if got != tt.want {
				t.Errorf("adjustForRunOutput(%v, output) = %v, want %v",
					tt.level, got, tt.want)
			}
		})
	}
}

func TestSelector_adjustForSpec(t *testing.T) {
	s := NewSelector()

	tests := []struct {
		name           string
		level          domain.InterventionLevel
		hasSpec        bool
		focusCriterion bool
		satisfied      int
		total          int
		want           domain.InterventionLevel
	}{
		{
			name:    "no spec returns unchanged level",
			level:   domain.L2LocationConcept,
			hasSpec: false,
			want:    domain.L2LocationConcept,
		},
		{
			name:           "with focus criterion keeps level unchanged",
			level:          domain.L2LocationConcept,
			hasSpec:        true,
			focusCriterion: true,
			satisfied:      3,
			total:          5,
			want:           domain.L2LocationConcept,
		},
		{
			name:      "high progress (>50%) decrements level",
			level:     domain.L2LocationConcept,
			hasSpec:   true,
			satisfied: 4,
			total:     6,
			want:      domain.L1CategoryHint,
		},
		{
			name:      "exactly 50% progress keeps level",
			level:     domain.L2LocationConcept,
			hasSpec:   true,
			satisfied: 3,
			total:     6,
			want:      domain.L2LocationConcept,
		},
		{
			name:      "low progress keeps level",
			level:     domain.L2LocationConcept,
			hasSpec:   true,
			satisfied: 1,
			total:     6,
			want:      domain.L2LocationConcept,
		},
		{
			name:      "high progress but L1 stays at L1",
			level:     domain.L1CategoryHint,
			hasSpec:   true,
			satisfied: 5,
			total:     6,
			want:      domain.L1CategoryHint,
		},
		{
			name:      "high progress with L0 stays at L0",
			level:     domain.L0Clarify,
			hasSpec:   true,
			satisfied: 5,
			total:     6,
			want:      domain.L0Clarify,
		},
		{
			name:      "no criteria returns unchanged",
			level:     domain.L2LocationConcept,
			hasSpec:   true,
			satisfied: 0,
			total:     0,
			want:      domain.L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx InterventionContext

			if tt.hasSpec {
				ctx.Spec = &domain.ProductSpec{
					AcceptanceCriteria: make([]domain.AcceptanceCriterion, tt.total),
				}
				for i := 0; i < tt.satisfied && i < tt.total; i++ {
					ctx.Spec.AcceptanceCriteria[i].Satisfied = true
				}
				if tt.focusCriterion && tt.total > 0 {
					ctx.FocusCriterion = &ctx.Spec.AcceptanceCriteria[0]
				}
			}

			got := s.adjustForSpec(tt.level, ctx)
			if got != tt.want {
				t.Errorf("adjustForSpec(%v, spec=%v, focus=%v, progress=%d/%d) = %v, want %v",
					tt.level, tt.hasSpec, tt.focusCriterion, tt.satisfied, tt.total, got, tt.want)
			}
		})
	}
}

func TestSelector_SelectLevel_Integration(t *testing.T) {
	s := NewSelector()
	policy := domain.DefaultPolicy()

	tests := []struct {
		name    string
		intent  domain.Intent
		ctx     InterventionContext
		want    domain.InterventionLevel
	}{
		{
			name:   "basic hint intent with empty context",
			intent: domain.IntentHint,
			ctx:    InterventionContext{},
			want:   domain.L1CategoryHint,
		},
		{
			name:   "review intent with empty context",
			intent: domain.IntentReview,
			ctx:    InterventionContext{},
			want:   domain.L2LocationConcept,
		},
		{
			name:   "stuck intent with advanced exercise reduces level",
			intent: domain.IntentStuck,
			ctx: InterventionContext{
				Exercise: &domain.Exercise{
					Difficulty: domain.DifficultyAdvanced,
				},
			},
			want: domain.L1CategoryHint,
		},
		{
			name:   "hint with all tests passed drops to L0",
			intent: domain.IntentHint,
			ctx: InterventionContext{
				RunOutput: &domain.RunOutput{
					TestsPassed: 5,
					TestsFailed: 0,
				},
			},
			want: domain.L0Clarify,
		},
		{
			name:   "hint with build errors escalates to L2",
			intent: domain.IntentHint,
			ctx: InterventionContext{
				RunOutput: &domain.RunOutput{
					BuildErrors: []domain.Diagnostic{{Message: "error"}},
				},
			},
			want: domain.L2LocationConcept,
		},
		{
			name:   "independent user with many runs gets reduced level",
			intent: domain.IntentReview,
			ctx: InterventionContext{
				Profile: &domain.LearningProfile{
					HintRequests: 1,
					TotalRuns:    20,
				},
			},
			want: domain.L1CategoryHint,
		},
		{
			name:   "spec with high progress reduces level",
			intent: domain.IntentReview,
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{Satisfied: true},
						{Satisfied: true},
						{Satisfied: true},
						{Satisfied: false},
					},
				},
			},
			want: domain.L1CategoryHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SelectLevel(tt.intent, tt.ctx, policy)
			if got != tt.want {
				t.Errorf("SelectLevel(%q, ctx) = %v, want %v", tt.intent, got, tt.want)
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
		{
			name:   "L0 returns Question",
			intent: domain.IntentHint,
			level:  domain.L0Clarify,
			want:   domain.TypeQuestion,
		},
		{
			name:   "L1 returns Hint",
			intent: domain.IntentHint,
			level:  domain.L1CategoryHint,
			want:   domain.TypeHint,
		},
		{
			name:   "L2 with Review returns Critique",
			intent: domain.IntentReview,
			level:  domain.L2LocationConcept,
			want:   domain.TypeCritique,
		},
		{
			name:   "L2 with Hint returns Nudge",
			intent: domain.IntentHint,
			level:  domain.L2LocationConcept,
			want:   domain.TypeNudge,
		},
		{
			name:   "L2 with Stuck returns Nudge",
			intent: domain.IntentStuck,
			level:  domain.L2LocationConcept,
			want:   domain.TypeNudge,
		},
		{
			name:   "L3 with Explain returns Explain",
			intent: domain.IntentExplain,
			level:  domain.L3ConstrainedSnippet,
			want:   domain.TypeExplain,
		},
		{
			name:   "L3 with Hint returns Snippet",
			intent: domain.IntentHint,
			level:  domain.L3ConstrainedSnippet,
			want:   domain.TypeSnippet,
		},
		{
			name:   "L4 returns Snippet",
			intent: domain.IntentHint,
			level:  domain.L4PartialSolution,
			want:   domain.TypeSnippet,
		},
		{
			name:   "L5 returns Snippet",
			intent: domain.IntentStuck,
			level:  domain.L5FullSolution,
			want:   domain.TypeSnippet,
		},
		{
			name:   "unknown level returns Hint",
			intent: domain.IntentHint,
			level:  domain.InterventionLevel(99),
			want:   domain.TypeHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SelectType(tt.intent, tt.level)
			if got != tt.want {
				t.Errorf("SelectType(%q, %v) = %v, want %v", tt.intent, tt.level, got, tt.want)
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
		{
			name:      "0 attempts should not escalate",
			attempts:  0,
			lastLevel: domain.L1CategoryHint,
			want:      false,
		},
		{
			name:      "1 attempt should not escalate",
			attempts:  1,
			lastLevel: domain.L1CategoryHint,
			want:      false,
		},
		{
			name:      "2 attempts should not escalate",
			attempts:  2,
			lastLevel: domain.L1CategoryHint,
			want:      false,
		},
		{
			name:      "3 attempts at L0 should escalate",
			attempts:  3,
			lastLevel: domain.L0Clarify,
			want:      true,
		},
		{
			name:      "3 attempts at L1 should escalate",
			attempts:  3,
			lastLevel: domain.L1CategoryHint,
			want:      true,
		},
		{
			name:      "3 attempts at L2 should escalate",
			attempts:  3,
			lastLevel: domain.L2LocationConcept,
			want:      true,
		},
		{
			name:      "3 attempts at L3 should NOT escalate",
			attempts:  3,
			lastLevel: domain.L3ConstrainedSnippet,
			want:      false,
		},
		{
			name:      "3 attempts at L4 should NOT escalate",
			attempts:  3,
			lastLevel: domain.L4PartialSolution,
			want:      false,
		},
		{
			name:      "3 attempts at L5 should NOT escalate",
			attempts:  3,
			lastLevel: domain.L5FullSolution,
			want:      false,
		},
		{
			name:      "5 attempts at L2 should escalate",
			attempts:  5,
			lastLevel: domain.L2LocationConcept,
			want:      true,
		},
		{
			name:      "10 attempts at L3 should NOT escalate",
			attempts:  10,
			lastLevel: domain.L3ConstrainedSnippet,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.ShouldEscalate(tt.attempts, tt.lastLevel)
			if got != tt.want {
				t.Errorf("ShouldEscalate(%d, %v) = %v, want %v",
					tt.attempts, tt.lastLevel, got, tt.want)
			}
		})
	}
}
