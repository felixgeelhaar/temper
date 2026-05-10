package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/pairing"
)

func TestScoreResponse_ClampPass(t *testing.T) {
	c := Case{
		ID:    "L1-clean",
		Level: 1, // L1CategoryHint
		Expect: Expectations{
			ClampPasses: true,
		},
	}
	s := ScoreResponse(c, "Think about how Go formats strings.", pairing.NewClampValidator())
	if !s.Passed {
		t.Fatalf("expected pass, got %+v", s)
	}
}

func TestScoreResponse_ClampViolation(t *testing.T) {
	c := Case{
		ID:    "L1-violates",
		Level: 1,
		Expect: Expectations{
			ClampPasses: true,
		},
	}
	s := ScoreResponse(c, "Use this:\n```go\nreturn fmt.Sprintf(\"hi\")\n```", pairing.NewClampValidator())
	if s.Passed {
		t.Fatal("expected fail for code-block at L1")
	}
	if s.ClampViolation == "" {
		t.Error("expected ClampViolation message to be populated")
	}
}

func TestScoreResponse_RequiredAndForbidden(t *testing.T) {
	c := Case{
		ID:    "fmt-mention",
		Level: 1,
		Expect: Expectations{
			ClampPasses:         true,
			RequiredSubstrings:  []string{"fmt"},
			ForbiddenSubstrings: []string{"Sprintf"},
		},
	}

	good := ScoreResponse(c, "Have a look at the fmt package's helpers.", pairing.NewClampValidator())
	if !good.Passed {
		t.Errorf("expected pass, got %+v", good)
	}

	missing := ScoreResponse(c, "Think about formatting.", pairing.NewClampValidator())
	if missing.Passed || len(missing.Missing) == 0 {
		t.Errorf("expected Missing[fmt], got %+v", missing)
	}

	forbidden := ScoreResponse(c, "Use fmt.Sprintf with a placeholder.", pairing.NewClampValidator())
	if forbidden.Passed || len(forbidden.Forbidden) == 0 {
		t.Errorf("expected Forbidden[Sprintf], got %+v", forbidden)
	}
}

func TestScoreResponse_MustContainQuestion(t *testing.T) {
	c := Case{
		ID:    "L0-clarify",
		Level: 0,
		Expect: Expectations{
			ClampPasses:         true,
			MustContainQuestion: true,
		},
	}

	with := ScoreResponse(c, "What inputs do you expect?", pairing.NewClampValidator())
	if !with.Passed {
		t.Errorf("expected pass for question, got %+v", with)
	}

	without := ScoreResponse(c, "Consider what happens when input is empty.", pairing.NewClampValidator())
	if without.Passed {
		t.Errorf("expected fail without question mark")
	}
}

func TestAggregate(t *testing.T) {
	scores := []Score{
		{CaseID: "a", Passed: true},
		{CaseID: "b", Passed: false, ClampViolation: "L1: code block"},
		{CaseID: "c", Passed: true},
		{CaseID: "d", Passed: false, NoQuestion: true},
	}
	r := Aggregate(scores)
	if r.Total != 4 {
		t.Errorf("Total = %d, want 4", r.Total)
	}
	if r.Passed != 2 {
		t.Errorf("Passed = %d, want 2", r.Passed)
	}
	if r.ClampViolations != 1 {
		t.Errorf("ClampViolations = %d, want 1", r.ClampViolations)
	}
	if rate := r.PassRate(); rate < 0.49 || rate > 0.51 {
		t.Errorf("PassRate = %v, want ~0.5", rate)
	}
}

// stubGenerator returns a fixed response for every case. Used to test the
// runner end-to-end without an LLM.
type stubGenerator struct{ response string }

func (s *stubGenerator) Generate(_ context.Context, _ Case, _, _ string) (string, error) {
	return s.response, nil
}

func TestRun_EndToEnd_Stubbed(t *testing.T) {
	cases := []Case{
		{
			ID:    "L0-question",
			Level: 0,
			Expect: Expectations{
				ClampPasses:         true,
				MustContainQuestion: true,
			},
		},
		{
			ID:    "L1-no-code",
			Level: 1,
			Expect: Expectations{
				ClampPasses: true,
			},
		},
	}

	gen := &stubGenerator{response: "What does this need to return?"}
	r, err := Run(context.Background(), cases, gen)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 2 {
		t.Errorf("Total = %d, want 2", r.Total)
	}
	if r.Passed != 2 {
		t.Errorf("Passed = %d, want 2 (stub gave a clamp-clean question)", r.Passed)
	}
}

func TestLoadCases_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: smoke-test
description: minimal case
intent: hint
level: 1
language: go
expect:
  clamp_passes: true
  required_substrings:
    - fmt
`
	if err := writeFileImpl(dir, "case1.yaml", yaml); err != nil {
		t.Fatal(err)
	}

	cases, err := LoadCases(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(cases))
	}
	if cases[0].ID != "smoke-test" {
		t.Errorf("ID = %q, want smoke-test", cases[0].ID)
	}
	if cases[0].Level != 1 {
		t.Errorf("Level = %d, want 1", cases[0].Level)
	}
	if !strings.Contains(cases[0].Expect.RequiredSubstrings[0], "fmt") {
		t.Errorf("RequiredSubstrings = %v, want [fmt]", cases[0].Expect.RequiredSubstrings)
	}
}

