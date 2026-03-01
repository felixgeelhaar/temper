package domain

import "testing"

func TestSelectionContext_SpecProgress(t *testing.T) {
	ctx := SelectionContext{
		Spec: &ProductSpec{
			AcceptanceCriteria: []AcceptanceCriterion{
				{ID: "a", Satisfied: true},
				{ID: "b", Satisfied: false},
			},
		},
	}

	if !ctx.HasSpec() {
		t.Error("HasSpec() should return true")
	}

	satisfied, total := ctx.SpecProgress()
	if satisfied != 1 || total != 2 {
		t.Errorf("SpecProgress() = (%d, %d); want (1, 2)", satisfied, total)
	}
}

func TestInterventionSelector_SelectLevel(t *testing.T) {
	selector := NewInterventionSelector()
	ctx := SelectionContext{
		Exercise: &Exercise{Difficulty: DifficultyAdvanced},
		Profile:  &LearningProfile{TotalRuns: 20, HintRequests: 0},
		RunOutput: &RunOutput{
			TestsPassed: 5,
			TestsFailed: 0,
		},
	}

	level := selector.SelectLevel(IntentReview, ctx, DefaultPolicy())
	if level != L0Clarify {
		t.Errorf("SelectLevel() = %v; want %v", level, L0Clarify)
	}
}

func TestInterventionSelector_AdjustForRunOutput(t *testing.T) {
	selector := NewInterventionSelector()
	output := &RunOutput{
		BuildErrors: []Diagnostic{{Message: "error"}},
	}
	level := selector.AdjustForRunOutput(L1CategoryHint, output)
	if level != L2LocationConcept {
		t.Errorf("AdjustForRunOutput() = %v; want %v", level, L2LocationConcept)
	}
}

func TestInterventionSelector_AdjustForSpec(t *testing.T) {
	selector := NewInterventionSelector()
	ctx := SelectionContext{
		Spec: &ProductSpec{
			AcceptanceCriteria: []AcceptanceCriterion{
				{ID: "a", Satisfied: true},
				{ID: "b", Satisfied: true},
				{ID: "c", Satisfied: false},
			},
		},
	}

	level := selector.AdjustForSpec(L2LocationConcept, ctx)
	if level != L1CategoryHint {
		t.Errorf("AdjustForSpec() = %v; want %v", level, L1CategoryHint)
	}
}

func TestInterventionSelector_SelectType(t *testing.T) {
	selector := NewInterventionSelector()

	if selector.SelectType(IntentReview, L2LocationConcept) != TypeCritique {
		t.Error("SelectType() should return critique for review at L2")
	}
	if selector.SelectType(IntentExplain, L3ConstrainedSnippet) != TypeExplain {
		t.Error("SelectType() should return explain for L3 explain intent")
	}
	if selector.SelectType(IntentHint, L0Clarify) != TypeQuestion {
		t.Error("SelectType() should return question for L0")
	}
}

func TestInterventionSelector_Escalation(t *testing.T) {
	selector := NewInterventionSelector()

	if !selector.ShouldEscalate(3, L2LocationConcept) {
		t.Error("ShouldEscalate() should return true for 3 attempts at L2")
	}
	if selector.ShouldEscalate(2, L2LocationConcept) {
		t.Error("ShouldEscalate() should return false for 2 attempts")
	}

	if selector.EscalateLevel(L1CategoryHint) != L2LocationConcept {
		t.Error("EscalateLevel() should increment level")
	}
	if selector.EscalateLevel(L3ConstrainedSnippet) != L3ConstrainedSnippet {
		t.Error("EscalateLevel() should not exceed L3")
	}
}
