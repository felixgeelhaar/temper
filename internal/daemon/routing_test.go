package daemon

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestBuildLevelModelMap_Empty(t *testing.T) {
	if m := buildLevelModelMap(nil); m != nil {
		t.Errorf("nil input → got %v, want nil", m)
	}
	if m := buildLevelModelMap(map[string]string{}); m != nil {
		t.Errorf("empty input → got %v, want nil", m)
	}
}

func TestBuildLevelModelMap_AcceptsBothKeyForms(t *testing.T) {
	in := map[string]string{
		"0":  "h",
		"L1": "h",
		"2":  "s",
		"L3": "s",
		"4":  "o",
		"L5": "o",
	}
	got := buildLevelModelMap(in)
	want := map[domain.InterventionLevel]string{
		domain.L0Clarify:            "h",
		domain.L1CategoryHint:       "h",
		domain.L2LocationConcept:    "s",
		domain.L3ConstrainedSnippet: "s",
		domain.L4PartialSolution:    "o",
		domain.L5FullSolution:       "o",
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d, want %d (%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("level %v: got %q, want %q", k, got[k], v)
		}
	}
}

func TestBuildLevelModelMap_IgnoresUnknownKeys(t *testing.T) {
	got := buildLevelModelMap(map[string]string{
		"0":     "h",
		"L99":   "ignored",
		"hello": "ignored",
		"6":     "ignored",
	})
	if len(got) != 1 {
		t.Errorf("expected 1 entry, got %d (%v)", len(got), got)
	}
	if got[domain.L0Clarify] != "h" {
		t.Errorf("L0 lost: %v", got)
	}
}
