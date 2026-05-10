package pairing

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestService_ModelForLevel_NilMapReturnsEmpty(t *testing.T) {
	s := NewService(nil, "")
	for _, l := range []domain.InterventionLevel{
		domain.L0Clarify, domain.L1CategoryHint, domain.L2LocationConcept,
		domain.L3ConstrainedSnippet, domain.L4PartialSolution, domain.L5FullSolution,
	} {
		if got := s.modelForLevel(l); got != "" {
			t.Errorf("level %v with nil map should return empty, got %q", l, got)
		}
	}
}

func TestService_ModelForLevel_RoutesPerLevel(t *testing.T) {
	s := NewService(nil, "")
	s.SetLevelModels(map[domain.InterventionLevel]string{
		domain.L0Clarify:           "haiku",
		domain.L1CategoryHint:      "haiku",
		domain.L2LocationConcept:   "sonnet",
		domain.L3ConstrainedSnippet: "sonnet",
		domain.L4PartialSolution:   "opus",
		domain.L5FullSolution:      "opus",
	})

	cases := []struct {
		level domain.InterventionLevel
		want  string
	}{
		{domain.L0Clarify, "haiku"},
		{domain.L1CategoryHint, "haiku"},
		{domain.L2LocationConcept, "sonnet"},
		{domain.L3ConstrainedSnippet, "sonnet"},
		{domain.L4PartialSolution, "opus"},
		{domain.L5FullSolution, "opus"},
	}
	for _, tc := range cases {
		if got := s.modelForLevel(tc.level); got != tc.want {
			t.Errorf("level %v → %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestService_ModelForLevel_UnsetLevelFallsBackToEmpty(t *testing.T) {
	// Config with only L4/L5 routed should leave L0..L3 to provider default.
	s := NewService(nil, "")
	s.SetLevelModels(map[domain.InterventionLevel]string{
		domain.L4PartialSolution: "opus",
	})

	if got := s.modelForLevel(domain.L1CategoryHint); got != "" {
		t.Errorf("L1 with no override → got %q, want empty", got)
	}
	if got := s.modelForLevel(domain.L4PartialSolution); got != "opus" {
		t.Errorf("L4 → got %q, want opus", got)
	}
}

func TestFallbackModelLabel(t *testing.T) {
	if got := fallbackModelLabel(""); got != "(provider default)" {
		t.Errorf("empty → got %q, want \"(provider default)\"", got)
	}
	if got := fallbackModelLabel("claude-haiku-4-5"); got != "claude-haiku-4-5" {
		t.Errorf("non-empty must round-trip, got %q", got)
	}
}
