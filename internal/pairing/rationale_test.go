package pairing

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestBuildRationale_AlwaysIncludesLevelIntentClampModel(t *testing.T) {
	got := buildRationale(domain.L2LocationConcept, InterventionRequest{
		Intent: domain.IntentStuck,
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}, "claude-haiku-4-5", "")

	mustContain(t, got, "L2", "intent=stuck", "policy clamp=L3", "model=claude-haiku-4-5")
}

func TestBuildRationale_NoModelLabelsAsProviderDefault(t *testing.T) {
	got := buildRationale(domain.L1CategoryHint, InterventionRequest{
		Intent: domain.IntentHint,
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}, "", "")
	if !strings.Contains(got, "model=(provider default)") {
		t.Errorf("empty model should label provider default, got: %s", got)
	}
}

func TestBuildRationale_WithProfileSurfacesDependencyAndTopic(t *testing.T) {
	req := InterventionRequest{
		Intent: domain.IntentHint,
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
		Context: InterventionContext{
			Exercise: &domain.Exercise{
				ID:         "go-v1/concurrency/channels",
				Difficulty: domain.DifficultyAdvanced,
			},
			Profile: &domain.LearningProfile{
				TotalRuns:    20,
				HintRequests: 5, // dependency 25%
				TopicSkills: map[string]domain.SkillLevel{
					"go/concurrency": {Level: 0.42},
				},
			},
		},
	}

	got := buildRationale(domain.L2LocationConcept, req, "claude-sonnet-4-6", "")
	mustContain(t, got,
		"hint dependency 25%",
		"topic=go/concurrency",
		"skill 0.42",
		"difficulty=advanced",
	)
}

func TestBuildRationale_RunOutputSignalsAreSurfaced(t *testing.T) {
	cases := []struct {
		name   string
		output *domain.RunOutput
		expect string
	}{
		{
			"all tests pass",
			&domain.RunOutput{BuildOK: true, TestsPassed: 5, TestsFailed: 0},
			"all tests passing",
		},
		{
			"build errors",
			&domain.RunOutput{BuildOK: false, BuildErrors: []domain.Diagnostic{{Message: "x"}}},
			"build errors present",
		},
		{
			"some tests failing",
			&domain.RunOutput{BuildOK: true, TestsPassed: 1, TestsFailed: 3},
			"3 tests failing",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRationale(domain.L2LocationConcept, InterventionRequest{
				Intent:  domain.IntentStuck,
				Policy:  domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
				Context: InterventionContext{RunOutput: tc.output},
			}, "", "")
			if !strings.Contains(got, tc.expect) {
				t.Errorf("expected %q in: %s", tc.expect, got)
			}
		})
	}
}

func TestBuildRationale_AppendsClampNote(t *testing.T) {
	got := buildRationale(domain.L1CategoryHint, InterventionRequest{
		Intent: domain.IntentHint,
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}, "", "; clamp violated, retry succeeded")
	if !strings.HasSuffix(got, "; clamp violated, retry succeeded") {
		t.Errorf("clamp note should be appended verbatim, got: %s", got)
	}
}

func mustContain(t *testing.T, s string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(s, p) {
			t.Errorf("rationale missing %q in: %s", p, s)
		}
	}
}
