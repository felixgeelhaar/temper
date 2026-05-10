package pairing

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestApplyPolicyClamp_NoExerciseOrProfile_FallsBackToClampLevel(t *testing.T) {
	policy := domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet}
	got := applyPolicyClamp(domain.L5FullSolution, policy, InterventionContext{})
	if got != domain.L3ConstrainedSnippet {
		t.Errorf("got %d, want L3 (static clamp)", got)
	}
}

func TestApplyPolicyClamp_ConfidentTopic_TightensCeiling(t *testing.T) {
	policy := domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet}
	ctx := InterventionContext{
		Exercise: &domain.Exercise{
			ID: "go-v1/concurrency/channels",
		},
		Profile: &domain.LearningProfile{
			TopicSkills: map[string]domain.SkillLevel{
				// ExtractTopic("go-v1/concurrency/channels") → "go/concurrency"
				"go/concurrency": {Level: 0.85},
			},
		},
	}

	got := applyPolicyClamp(domain.L3ConstrainedSnippet, policy, ctx)
	if got != domain.L2LocationConcept {
		t.Errorf("confident topic at L3 → got %d, want L2", got)
	}
}

func TestApplyPolicyClamp_StrugglingTopic_KeepsCeiling(t *testing.T) {
	policy := domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet}
	ctx := InterventionContext{
		Exercise: &domain.Exercise{ID: "go-v1/basics/hello-world"},
		Profile: &domain.LearningProfile{
			TopicSkills: map[string]domain.SkillLevel{
				"go/basics": {Level: 0.1},
			},
		},
	}

	got := applyPolicyClamp(domain.L3ConstrainedSnippet, policy, ctx)
	if got != domain.L3ConstrainedSnippet {
		t.Errorf("struggling topic at L3 → got %d, want L3", got)
	}
}

func TestApplyPolicyClamp_UnknownTopic_FallsBackToZeroSkill(t *testing.T) {
	// Empty TopicSkills map → GetSkillLevel returns Level: 0.0 →
	// no decrement, ceiling unchanged.
	policy := domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet}
	ctx := InterventionContext{
		Exercise: &domain.Exercise{ID: "go-v1/basics/hello-world"},
		Profile: &domain.LearningProfile{
			TopicSkills: map[string]domain.SkillLevel{},
		},
	}

	got := applyPolicyClamp(domain.L3ConstrainedSnippet, policy, ctx)
	if got != domain.L3ConstrainedSnippet {
		t.Errorf("unknown topic must not tighten, got %d", got)
	}
}
