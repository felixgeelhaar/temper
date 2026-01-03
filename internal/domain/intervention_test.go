package domain

import (
	"testing"
)

func TestInterventionLevel_String(t *testing.T) {
	tests := []struct {
		level    InterventionLevel
		expected string
	}{
		{L0Clarify, "clarify"},
		{L1CategoryHint, "category"},
		{L2LocationConcept, "location"},
		{L3ConstrainedSnippet, "snippet"},
		{L4PartialSolution, "partial"},
		{L5FullSolution, "solution"},
		{InterventionLevel(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.level.String()
			if got != tc.expected {
				t.Errorf("Level(%d).String() = %q; want %q", tc.level, got, tc.expected)
			}
		})
	}
}

func TestInterventionLevel_Description(t *testing.T) {
	// Each level should have a non-empty description
	levels := []InterventionLevel{
		L0Clarify, L1CategoryHint, L2LocationConcept,
		L3ConstrainedSnippet, L4PartialSolution, L5FullSolution,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			desc := level.Description()
			if desc == "" {
				t.Errorf("Level(%d).Description() is empty", level)
			}
		})
	}
}

func TestDefaultPolicy(t *testing.T) {
	policy := DefaultPolicy()

	if policy.MaxLevel != L3ConstrainedSnippet {
		t.Errorf("DefaultPolicy().MaxLevel = %d; want %d", policy.MaxLevel, L3ConstrainedSnippet)
	}
	if policy.PatchingEnabled {
		t.Error("DefaultPolicy().PatchingEnabled should be false")
	}
	if policy.CooldownSeconds != 60 {
		t.Errorf("DefaultPolicy().CooldownSeconds = %d; want 60", policy.CooldownSeconds)
	}
	if policy.Track != "practice" {
		t.Errorf("DefaultPolicy().Track = %q; want %q", policy.Track, "practice")
	}
}

func TestInterviewPrepPolicy(t *testing.T) {
	policy := InterviewPrepPolicy()

	if policy.MaxLevel != L2LocationConcept {
		t.Errorf("InterviewPrepPolicy().MaxLevel = %d; want %d", policy.MaxLevel, L2LocationConcept)
	}
	if policy.CooldownSeconds != 120 {
		t.Errorf("InterviewPrepPolicy().CooldownSeconds = %d; want 120", policy.CooldownSeconds)
	}
	if policy.Track != "interview-prep" {
		t.Errorf("InterviewPrepPolicy().Track = %q; want %q", policy.Track, "interview-prep")
	}
}

func TestLearningPolicy_ClampLevel(t *testing.T) {
	tests := []struct {
		name      string
		maxLevel  InterventionLevel
		requested InterventionLevel
		expected  InterventionLevel
	}{
		{"below max", L3ConstrainedSnippet, L1CategoryHint, L1CategoryHint},
		{"at max", L3ConstrainedSnippet, L3ConstrainedSnippet, L3ConstrainedSnippet},
		{"above max", L3ConstrainedSnippet, L5FullSolution, L3ConstrainedSnippet},
		{"strict policy", L2LocationConcept, L4PartialSolution, L2LocationConcept},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := LearningPolicy{MaxLevel: tc.maxLevel}
			got := policy.ClampLevel(tc.requested)
			if got != tc.expected {
				t.Errorf("ClampLevel(%d) = %d; want %d", tc.requested, got, tc.expected)
			}
		})
	}
}
