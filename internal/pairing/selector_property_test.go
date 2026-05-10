package pairing

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// drawIntent generates one of the five supported intents.
func drawIntent(t *rapid.T) domain.Intent {
	options := []domain.Intent{
		domain.IntentHint,
		domain.IntentReview,
		domain.IntentStuck,
		domain.IntentNext,
		domain.IntentExplain,
	}
	return options[rapid.IntRange(0, len(options)-1).Draw(t, "intent")]
}

// drawDifficulty generates one of three difficulties.
func drawDifficulty(t *rapid.T) domain.Difficulty {
	options := []domain.Difficulty{
		domain.DifficultyBeginner,
		domain.DifficultyIntermediate,
		domain.DifficultyAdvanced,
	}
	return options[rapid.IntRange(0, len(options)-1).Draw(t, "difficulty")]
}

// drawContext synthesizes a context with optional exercise/profile/output.
func drawContext(t *rapid.T) InterventionContext {
	ctx := InterventionContext{}

	if rapid.Bool().Draw(t, "has_exercise") {
		ctx.Exercise = &domain.Exercise{
			ID:         "test/ex",
			Title:      "Test",
			Difficulty: drawDifficulty(t),
		}
	}

	if rapid.Bool().Draw(t, "has_profile") {
		runs := rapid.IntRange(0, 200).Draw(t, "total_runs")
		hints := rapid.IntRange(0, runs).Draw(t, "hint_requests")
		ctx.Profile = &domain.LearningProfile{
			TotalRuns:    runs,
			HintRequests: hints,
		}
	}

	if rapid.Bool().Draw(t, "has_output") {
		ctx.RunOutput = &domain.RunOutput{
			BuildOK:     rapid.Bool().Draw(t, "build_ok"),
			TestsPassed: rapid.IntRange(0, 50).Draw(t, "passed"),
			TestsFailed: rapid.IntRange(0, 50).Draw(t, "failed"),
		}
	}

	return ctx
}

// drawPolicy generates a learning policy with valid MaxLevel.
func drawPolicy(t *rapid.T) domain.LearningPolicy {
	max := domain.InterventionLevel(rapid.IntRange(0, 5).Draw(t, "max_level"))
	return domain.LearningPolicy{
		MaxLevel:        max,
		CooldownSeconds: 60,
		Track:           "practice",
	}
}

// TestSelector_LevelInRange asserts the foundational invariant: the
// selector's output, after caller-side ClampLevel, is always within
// [L0, policy.MaxLevel]. Underflow at L0 must never occur regardless of
// how many decrement-firing adjustments compose.
func TestSelector_LevelInRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewSelector()
		intent := drawIntent(t)
		ctx := drawContext(t)
		policy := drawPolicy(t)

		got := s.SelectLevel(intent, ctx, policy)
		got = policy.ClampLevel(got)

		if got < domain.L0Clarify {
			t.Fatalf("level %d below floor L0 (intent=%v ctx=%+v policy=%+v)",
				got, intent, ctx, policy)
		}
		if got > policy.MaxLevel {
			t.Fatalf("level %d above policy clamp %d", got, policy.MaxLevel)
		}
		if got > domain.L5FullSolution {
			t.Fatalf("level %d above L5 ceiling", got)
		}
	})
}

// TestSelector_Idempotent asserts that selecting twice with the same
// inputs yields the same output. Selection must be a pure function of
// (intent, context, policy).
func TestSelector_Idempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewSelector()
		intent := drawIntent(t)
		ctx := drawContext(t)
		policy := drawPolicy(t)

		first := s.SelectLevel(intent, ctx, policy)
		second := s.SelectLevel(intent, ctx, policy)

		if first != second {
			t.Fatalf("not idempotent: first=%d second=%d", first, second)
		}
	})
}

// TestSelector_HighDependencyDoesNotDecrement asserts the documented rule
// in adjustForProfile: when HintDependency > 0.5, the profile branch must
// not decrement the level. (Other adjusters may still lower it, e.g.
// AllTestsPassed → L0 — but the profile branch alone must not.)
func TestSelector_HighDependencyDoesNotDecrement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewSelector()

		// Build a context that pins all *other* adjusters to no-op so we
		// can isolate the profile branch:
		//   - no Exercise         → adjustForContext is no-op
		//   - no RunOutput        → adjustForRunOutput is no-op
		//   - no Spec             → adjustForSpec is no-op
		runs := rapid.IntRange(20, 200).Draw(t, "runs")
		// dependency >= 0.6 → ensures HintDependency() > 0.5 strictly.
		hints := runs * 6 / 10
		if hints < runs*6/10+1 {
			hints = runs*6/10 + 1
		}
		if hints > runs {
			hints = runs
		}

		ctx := InterventionContext{
			Profile: &domain.LearningProfile{
				TotalRuns:    runs,
				HintRequests: hints,
			},
		}
		policy := domain.LearningPolicy{
			MaxLevel:        domain.L5FullSolution,
			CooldownSeconds: 60,
			Track:           "practice",
		}
		intent := drawIntent(t)

		got := s.SelectLevel(intent, ctx, policy)
		want := s.intentToBaseLevel(intent)

		if got != want {
			t.Fatalf("high dependency must not change level: base=%d got=%d (runs=%d hints=%d dep=%.3f)",
				want, got, runs, hints, ctx.Profile.HintDependency())
		}
	})
}

// TestSelector_PolicyClampUpperBound asserts that the selector output
// clamped by policy never exceeds MaxLevel, for every MaxLevel in [L0..L5].
func TestSelector_PolicyClampUpperBound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewSelector()
		intent := drawIntent(t)
		ctx := drawContext(t)
		max := domain.InterventionLevel(rapid.IntRange(0, 5).Draw(t, "max"))
		policy := domain.LearningPolicy{
			MaxLevel:        max,
			CooldownSeconds: 60,
			Track:           "practice",
		}

		got := policy.ClampLevel(s.SelectLevel(intent, ctx, policy))
		if got > max {
			t.Fatalf("clamped level %d exceeds MaxLevel %d", got, max)
		}
	})
}

// TestSelector_AllTestsPassedForcesL0 asserts the AllTestsPassed branch
// of adjustForRunOutput: when all tests pass, the selector returns L0
// regardless of any other adjustment that might raise the level.
func TestSelector_AllTestsPassedForcesL0(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewSelector()
		intent := drawIntent(t)
		passed := rapid.IntRange(1, 50).Draw(t, "passed")

		ctx := InterventionContext{
			RunOutput: &domain.RunOutput{
				BuildOK:     true,
				TestsPassed: passed,
				TestsFailed: 0,
			},
		}
		policy := domain.LearningPolicy{MaxLevel: domain.L5FullSolution}

		got := s.SelectLevel(intent, ctx, policy)
		if got != domain.L0Clarify {
			t.Fatalf("all-tests-pass must yield L0, got %d (intent=%v)", got, intent)
		}
	})
}

// TestSelector_BuildErrorRaisesToL2 asserts that build errors ratchet the
// level UP to at least L2 (since concrete location help is required), and
// the ratchet survives whatever the profile branch may have done.
func TestSelector_BuildErrorRaisesToL2(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewSelector()
		intent := drawIntent(t)

		ctx := InterventionContext{
			RunOutput: &domain.RunOutput{
				BuildOK: false,
				BuildErrors: []domain.Diagnostic{
					{File: "main.go", Line: 1, Message: "syntax"},
				},
			},
			// Even an "independent learner" decrement must not undo the build-error ratchet.
			Profile: &domain.LearningProfile{
				TotalRuns:    100,
				HintRequests: 5, // dependency 0.05 < 0.2 → would normally decrement
			},
		}
		policy := domain.LearningPolicy{MaxLevel: domain.L5FullSolution}

		got := s.SelectLevel(intent, ctx, policy)
		if got < domain.L2LocationConcept {
			t.Fatalf("build error must raise level to >= L2, got %d", got)
		}
	})
}

// TestDecrement_Floor is a unit-level invariant for the smart constructor.
func TestDecrement_Floor(t *testing.T) {
	if got := domain.L0Clarify.Decrement(); got != domain.L0Clarify {
		t.Errorf("L0.Decrement() = %d, want L0", got)
	}
	if got := domain.L3ConstrainedSnippet.Decrement(); got != domain.L2LocationConcept {
		t.Errorf("L3.Decrement() = %d, want L2", got)
	}
}

func TestClampToFloor(t *testing.T) {
	if got := domain.InterventionLevel(-1).ClampToFloor(); got != domain.L0Clarify {
		t.Errorf("(-1).ClampToFloor() = %d, want L0", got)
	}
	if got := domain.L3ConstrainedSnippet.ClampToFloor(); got != domain.L3ConstrainedSnippet {
		t.Errorf("L3.ClampToFloor() = %d, want L3", got)
	}
}
