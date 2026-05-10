package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/pairing"
)

// Generator returns the LLM response for a single case. Abstracted so tests
// can pass canned responses and the CLI can wire a real provider.
type Generator interface {
	Generate(ctx context.Context, c Case, prompt, system string) (string, error)
}

// LLMGenerator wraps a llm.Provider to satisfy Generator.
type LLMGenerator struct {
	Provider llm.Provider
}

func (g *LLMGenerator) Generate(ctx context.Context, _ Case, prompt, system string) (string, error) {
	resp, err := g.Provider.Generate(ctx, &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System:      system,
		MaxTokens:   1024,
		Temperature: 0.7,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// Run executes every case, builds a prompt via the same Prompter used in
// production, generates a response via gen, and scores it. Returns
// aggregated results.
func Run(ctx context.Context, cases []Case, gen Generator) (*Result, error) {
	prompter := pairing.NewPrompter()
	validator := pairing.NewClampValidator()

	scores := make([]Score, 0, len(cases))
	for _, c := range cases {
		req := buildPromptRequest(c)
		userPrompt := prompter.BuildPrompt(req)
		systemPrompt := prompter.SystemPrompt(c.LevelDomain())

		response, err := gen.Generate(ctx, c, userPrompt, systemPrompt)
		if err != nil {
			scores = append(scores, Score{
				CaseID:         c.ID,
				Passed:         false,
				ClampViolation: fmt.Sprintf("generator error: %v", err),
			})
			continue
		}

		scores = append(scores, ScoreResponse(c, response, validator))
	}
	return Aggregate(scores), nil
}

// FormatReport renders a human-readable summary suitable for `make eval`.
func FormatReport(r *Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Eval results: %d/%d passed (%.1f%%)\n",
		r.Passed, r.Total, r.PassRate()*100)
	fmt.Fprintf(&sb, "Clamp violations: %d\n\n", r.ClampViolations)

	for _, s := range r.Scores {
		mark := "PASS"
		if !s.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(&sb, "[%s] %s\n", mark, s.CaseID)
		if s.ClampViolation != "" {
			fmt.Fprintf(&sb, "    clamp: %s\n", s.ClampViolation)
		}
		if s.NoQuestion {
			fmt.Fprintf(&sb, "    missing-question-mark\n")
		}
		for _, m := range s.Missing {
			fmt.Fprintf(&sb, "    missing required: %q\n", m)
		}
		for _, f := range s.Forbidden {
			fmt.Fprintf(&sb, "    forbidden present: %q\n", f)
		}
	}
	return sb.String()
}

func buildPromptRequest(c Case) pairing.PromptRequest {
	exercise := &domain.Exercise{
		ID:    c.ExerciseID,
		Title: c.ExerciseID,
	}

	var output *domain.RunOutput
	if c.TestsPassed > 0 || c.TestsFailed > 0 || len(c.BuildErrors) > 0 {
		output = &domain.RunOutput{
			BuildOK:     len(c.BuildErrors) == 0,
			TestsPassed: c.TestsPassed,
			TestsFailed: c.TestsFailed,
		}
		for _, msg := range c.BuildErrors {
			output.BuildErrors = append(output.BuildErrors, domain.Diagnostic{
				Message: msg,
			})
		}
	}

	var profile *domain.LearningProfile
	if c.Profile != nil {
		profile = &domain.LearningProfile{
			HintRequests: int(c.Profile.HintDependency * float64(c.Profile.TotalRuns)),
			TotalRuns:    c.Profile.TotalRuns,
		}
	}

	return pairing.PromptRequest{
		Intent:   c.IntentDomain(),
		Level:    c.LevelDomain(),
		Type:     domain.TypeHint,
		Exercise: exercise,
		Code:     c.Code,
		Output:   output,
		Profile:  profile,
	}
}
