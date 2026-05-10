package eval

import (
	"strings"

	"github.com/felixgeelhaar/temper/internal/pairing"
)

// Score is the result of evaluating a single case against an LLM response.
type Score struct {
	CaseID         string
	Passed         bool
	ClampViolation string   // populated when the clamp check failed
	Missing        []string // RequiredSubstrings that did not appear
	Forbidden      []string // ForbiddenSubstrings that did appear
	NoQuestion     bool     // MustContainQuestion was set but content has no '?'
}

// Result aggregates Scores from a run.
type Result struct {
	Total          int
	Passed         int
	ClampViolations int
	Scores         []Score
}

// PassRate returns Passed/Total as a 0..1 ratio. Zero on empty Result.
func (r *Result) PassRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Passed) / float64(r.Total)
}

// ScoreResponse evaluates a single LLM response against a case's expectations.
// validator is the same ClampValidator used in production, ensuring eval and
// runtime apply identical rules.
func ScoreResponse(c Case, response string, validator *pairing.ClampValidator) Score {
	s := Score{CaseID: c.ID, Passed: true}

	if c.Expect.ClampPasses {
		if err := validator.Validate(c.LevelDomain(), response); err != nil {
			s.Passed = false
			s.ClampViolation = err.Error()
		}
	}

	if c.Expect.MustContainQuestion && !strings.Contains(response, "?") {
		s.Passed = false
		s.NoQuestion = true
	}

	for _, want := range c.Expect.RequiredSubstrings {
		if !strings.Contains(response, want) {
			s.Passed = false
			s.Missing = append(s.Missing, want)
		}
	}

	for _, bad := range c.Expect.ForbiddenSubstrings {
		if strings.Contains(response, bad) {
			s.Passed = false
			s.Forbidden = append(s.Forbidden, bad)
		}
	}

	return s
}

// Aggregate sums Scores into a Result.
func Aggregate(scores []Score) *Result {
	r := &Result{Total: len(scores), Scores: scores}
	for _, s := range scores {
		if s.Passed {
			r.Passed++
		}
		if s.ClampViolation != "" {
			r.ClampViolations++
		}
	}
	return r
}
