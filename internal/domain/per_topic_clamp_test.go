package domain

import "testing"

func TestClampForTopic_HighSkillTightensCeiling(t *testing.T) {
	policy := LearningPolicy{MaxLevel: L3ConstrainedSnippet}

	// Confident learner: ceiling drops from L3 to L2.
	got := policy.ClampForTopic(L3ConstrainedSnippet, 0.85)
	if got != L2LocationConcept {
		t.Errorf("high skill at L3 → got %d, want L2", got)
	}
	got = policy.ClampForTopic(L1CategoryHint, 0.85)
	if got != L1CategoryHint {
		t.Errorf("requested below ceiling stays unchanged, got %d, want L1", got)
	}
}

func TestClampForTopic_MidSkillKeepsCeiling(t *testing.T) {
	policy := LearningPolicy{MaxLevel: L3ConstrainedSnippet}
	got := policy.ClampForTopic(L3ConstrainedSnippet, 0.5)
	if got != L3ConstrainedSnippet {
		t.Errorf("mid skill at L3 → got %d, want L3 (no tighten)", got)
	}
}

func TestClampForTopic_LowSkillKeepsCeiling(t *testing.T) {
	policy := LearningPolicy{MaxLevel: L3ConstrainedSnippet}
	got := policy.ClampForTopic(L3ConstrainedSnippet, 0.1)
	if got != L3ConstrainedSnippet {
		t.Errorf("low skill at L3 → got %d, want L3", got)
	}
}

func TestClampForTopic_HighSkillAtL1FloorsAtL0(t *testing.T) {
	// MaxLevel L1 + high skill → ceiling decrements to L0.
	policy := LearningPolicy{MaxLevel: L1CategoryHint}
	got := policy.ClampForTopic(L1CategoryHint, 0.9)
	if got != L0Clarify {
		t.Errorf("high skill at L1 ceiling → got %d, want L0", got)
	}
}

func TestClampForTopic_NeverUnderflows(t *testing.T) {
	policy := LearningPolicy{MaxLevel: L0Clarify}
	got := policy.ClampForTopic(L0Clarify, 0.95)
	if got != L0Clarify {
		t.Errorf("L0 with high skill must not underflow, got %d", got)
	}
}
