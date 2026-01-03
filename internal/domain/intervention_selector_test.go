package domain

import (
	"testing"
)

func TestInterventionSelector_IntentToBaseLevel(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name   string
		intent Intent
		want   InterventionLevel
	}{
		{"hint returns L1", IntentHint, L1CategoryHint},
		{"review returns L2", IntentReview, L2LocationConcept},
		{"stuck returns L2", IntentStuck, L2LocationConcept},
		{"next returns L1", IntentNext, L1CategoryHint},
		{"explain returns L2", IntentExplain, L2LocationConcept},
		{"unknown returns L1", Intent{}, L1CategoryHint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.IntentToBaseLevel(tt.intent)
			if got != tt.want {
				t.Errorf("IntentToBaseLevel(%v) = %v, want %v", tt.intent, got, tt.want)
			}
		})
	}
}

func TestInterventionSelector_AdjustForContext(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name       string
		level      InterventionLevel
		difficulty Difficulty
		want       InterventionLevel
	}{
		{
			name:       "beginner keeps level",
			level:      L2LocationConcept,
			difficulty: DifficultyBeginner,
			want:       L2LocationConcept,
		},
		{
			name:       "intermediate keeps level",
			level:      L2LocationConcept,
			difficulty: DifficultyIntermediate,
			want:       L2LocationConcept,
		},
		{
			name:       "advanced reduces level above L1",
			level:      L2LocationConcept,
			difficulty: DifficultyAdvanced,
			want:       L1CategoryHint,
		},
		{
			name:       "advanced keeps L1",
			level:      L1CategoryHint,
			difficulty: DifficultyAdvanced,
			want:       L1CategoryHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SelectionContext{
				Exercise: &Exercise{Difficulty: tt.difficulty},
			}
			got := s.AdjustForContext(tt.level, ctx)
			if got != tt.want {
				t.Errorf("AdjustForContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterventionSelector_AdjustForProfile(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name         string
		level        InterventionLevel
		hintRequests int
		totalRuns    int
		want         InterventionLevel
	}{
		{
			name:         "nil profile keeps level",
			level:        L2LocationConcept,
			hintRequests: 0,
			totalRuns:    0,
			want:         L2LocationConcept,
		},
		{
			name:         "high dependency keeps level",
			level:        L2LocationConcept,
			hintRequests: 8,
			totalRuns:    10,
			want:         L2LocationConcept,
		},
		{
			name:         "low dependency with experience reduces level",
			level:        L2LocationConcept,
			hintRequests: 1,
			totalRuns:    20,
			want:         L1CategoryHint,
		},
		{
			name:         "low dependency with low experience keeps level",
			level:        L2LocationConcept,
			hintRequests: 0,
			totalRuns:    5,
			want:         L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profile *LearningProfile
			if tt.totalRuns > 0 || tt.hintRequests > 0 {
				profile = &LearningProfile{
					HintRequests: tt.hintRequests,
					TotalRuns:    tt.totalRuns,
				}
			}
			got := s.AdjustForProfile(tt.level, profile)
			if got != tt.want {
				t.Errorf("AdjustForProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterventionSelector_AdjustForRunOutput(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name   string
		level  InterventionLevel
		output *RunOutput
		want   InterventionLevel
	}{
		{
			name:   "nil output keeps level",
			level:  L2LocationConcept,
			output: nil,
			want:   L2LocationConcept,
		},
		{
			name:  "all tests pass reduces to L0",
			level: L2LocationConcept,
			output: &RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			want: L0Clarify,
		},
		{
			name:  "build errors increase to L2",
			level: L1CategoryHint,
			output: &RunOutput{
				BuildErrors: []Diagnostic{{Severity: "error"}},
			},
			want: L2LocationConcept,
		},
		{
			name:  "many test failures increase to L2",
			level: L1CategoryHint,
			output: &RunOutput{
				TestsFailed: 5,
			},
			want: L2LocationConcept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.AdjustForRunOutput(tt.level, tt.output)
			if got != tt.want {
				t.Errorf("AdjustForRunOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterventionSelector_SelectType(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name   string
		intent Intent
		level  InterventionLevel
		want   InterventionType
	}{
		{"L0 returns question", IntentHint, L0Clarify, TypeQuestion},
		{"L1 returns hint", IntentHint, L1CategoryHint, TypeHint},
		{"L2 review returns critique", IntentReview, L2LocationConcept, TypeCritique},
		{"L2 non-review returns nudge", IntentHint, L2LocationConcept, TypeNudge},
		{"L3 explain returns explain", IntentExplain, L3ConstrainedSnippet, TypeExplain},
		{"L3 non-explain returns snippet", IntentStuck, L3ConstrainedSnippet, TypeSnippet},
		{"L4 returns snippet", IntentStuck, L4PartialSolution, TypeSnippet},
		{"L5 returns snippet", IntentStuck, L5FullSolution, TypeSnippet},
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

func TestInterventionSelector_ShouldEscalate(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name      string
		attempts  int
		lastLevel InterventionLevel
		want      bool
	}{
		{"no escalate at 1 attempt", 1, L1CategoryHint, false},
		{"no escalate at 2 attempts", 2, L1CategoryHint, false},
		{"escalate at 3 attempts below L3", 3, L1CategoryHint, true},
		{"no escalate at L3 even with 3 attempts", 3, L3ConstrainedSnippet, false},
		{"no escalate at L4", 5, L4PartialSolution, false},
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

func TestInterventionSelector_AdjustForSpec(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name  string
		level InterventionLevel
		ctx   SelectionContext
		want  InterventionLevel
	}{
		{
			name:  "no spec keeps level",
			level: L2LocationConcept,
			ctx:   SelectionContext{},
			want:  L2LocationConcept,
		},
		{
			name:  "with focus criterion keeps level",
			level: L2LocationConcept,
			ctx: SelectionContext{
				Spec:           &ProductSpec{},
				FocusCriterion: &AcceptanceCriterion{ID: "ac-1"},
			},
			want: L2LocationConcept,
		},
		{
			name:  "high progress reduces level above L1",
			level: L2LocationConcept,
			ctx: SelectionContext{
				Spec: &ProductSpec{
					AcceptanceCriteria: []AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: true},
						{ID: "ac-3", Satisfied: false},
					},
				},
			},
			want: L1CategoryHint,
		},
		{
			name:  "low progress keeps level",
			level: L2LocationConcept,
			ctx: SelectionContext{
				Spec: &ProductSpec{
					AcceptanceCriteria: []AcceptanceCriterion{
						{ID: "ac-1", Satisfied: false},
						{ID: "ac-2", Satisfied: false},
						{ID: "ac-3", Satisfied: false},
					},
				},
			},
			want: L2LocationConcept,
		},
		{
			name:  "L1 keeps L1 even with progress",
			level: L1CategoryHint,
			ctx: SelectionContext{
				Spec: &ProductSpec{
					AcceptanceCriteria: []AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: true},
					},
				},
			},
			want: L1CategoryHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.AdjustForSpec(tt.level, tt.ctx)
			if got != tt.want {
				t.Errorf("AdjustForSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterventionSelector_SelectLevel_Integration(t *testing.T) {
	s := NewInterventionSelector()
	policy := LearningPolicy{}

	tests := []struct {
		name   string
		intent Intent
		ctx    SelectionContext
		want   InterventionLevel
	}{
		{
			name:   "hint with no context",
			intent: IntentHint,
			ctx:    SelectionContext{},
			want:   L1CategoryHint,
		},
		{
			name:   "stuck with passing tests gets L0",
			intent: IntentStuck,
			ctx: SelectionContext{
				RunOutput: &RunOutput{TestsPassed: 5, TestsFailed: 0},
			},
			want: L0Clarify,
		},
		{
			name:   "review with advanced exercise",
			intent: IntentReview,
			ctx: SelectionContext{
				Exercise: &Exercise{Difficulty: DifficultyAdvanced},
			},
			want: L1CategoryHint,
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

func TestInterventionSelector_EscalateLevel(t *testing.T) {
	s := NewInterventionSelector()

	tests := []struct {
		name    string
		current InterventionLevel
		want    InterventionLevel
	}{
		{"L0 escalates to L1", L0Clarify, L1CategoryHint},
		{"L1 escalates to L2", L1CategoryHint, L2LocationConcept},
		{"L2 escalates to L3", L2LocationConcept, L3ConstrainedSnippet},
		{"L3 stays at L3", L3ConstrainedSnippet, L3ConstrainedSnippet},
		{"L4 stays at L4", L4PartialSolution, L4PartialSolution},
		{"L5 stays at L5", L5FullSolution, L5FullSolution},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.EscalateLevel(tt.current)
			if got != tt.want {
				t.Errorf("EscalateLevel(%v) = %v, want %v", tt.current, got, tt.want)
			}
		})
	}
}

func TestSelectionContext_HasSpec(t *testing.T) {
	tests := []struct {
		name string
		ctx  SelectionContext
		want bool
	}{
		{"nil spec", SelectionContext{}, false},
		{"with spec", SelectionContext{Spec: &ProductSpec{}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.HasSpec(); got != tt.want {
				t.Errorf("HasSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectionContext_SpecProgress(t *testing.T) {
	tests := []struct {
		name          string
		ctx           SelectionContext
		wantSatisfied int
		wantTotal     int
	}{
		{
			name:          "nil spec",
			ctx:           SelectionContext{},
			wantSatisfied: 0,
			wantTotal:     0,
		},
		{
			name: "all satisfied",
			ctx: SelectionContext{
				Spec: &ProductSpec{
					AcceptanceCriteria: []AcceptanceCriterion{
						{Satisfied: true},
						{Satisfied: true},
					},
				},
			},
			wantSatisfied: 2,
			wantTotal:     2,
		},
		{
			name: "partial",
			ctx: SelectionContext{
				Spec: &ProductSpec{
					AcceptanceCriteria: []AcceptanceCriterion{
						{Satisfied: true},
						{Satisfied: false},
						{Satisfied: false},
					},
				},
			},
			wantSatisfied: 1,
			wantTotal:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, total := tt.ctx.SpecProgress()
			if s != tt.wantSatisfied || total != tt.wantTotal {
				t.Errorf("SpecProgress() = (%d, %d), want (%d, %d)", s, total, tt.wantSatisfied, tt.wantTotal)
			}
		})
	}
}
