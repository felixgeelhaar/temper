package pairing

import (
	"errors"
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestClampValidator_L0_Clarify(t *testing.T) {
	v := NewClampValidator()

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{"valid question", "What does this function need to return when name is empty?", false},
		{"valid multi-question", "What inputs do you expect? How should errors propagate?", false},
		{"missing question mark", "Consider what happens when input is empty.", true},
		{"contains fenced code", "What about this?\n```go\nreturn fmt.Sprintf(\"hi\")\n```", true},
		{"contains indented code", "Try this:\n\n    return \"hello\"\n", true},
		{"empty content allowed", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(domain.L0Clarify, tc.content)
			if (err != nil) != tc.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tc.wantError)
			}
			if tc.wantError && !errors.Is(err, ErrClampViolation) {
				t.Errorf("expected ErrClampViolation, got %v", err)
			}
		})
	}
}

func TestClampValidator_L1_Category(t *testing.T) {
	v := NewClampValidator()

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{"category prose", "Think about how Go handles string formatting.", false},
		{"contains fenced code", "Look at fmt:\n```\nfmt.Sprintf\n```", true},
		{"contains indented code", "Use this:\n\n    fmt.Sprintf(\"hi\")\n", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(domain.L1CategoryHint, tc.content)
			if (err != nil) != tc.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

func TestClampValidator_L2_LocationConcept(t *testing.T) {
	v := NewClampValidator()

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{"location prose", "The issue is in your Hello function. Consider how Go checks for empty strings.", false},
		{"contains code block", "Use this:\n```go\nif name == \"\" {}\n```", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(domain.L2LocationConcept, tc.content)
			if (err != nil) != tc.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

func TestClampValidator_L3_Snippet(t *testing.T) {
	v := NewClampValidator()

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{"prose only", "Outline the structure first.", false},
		{"snippet with TODO placeholder", "```go\nfunc Hello() {\n  // TODO: implement\n}\n```", false},
		{"snippet with your-logic placeholder", "```go\nif x {\n  // your logic here\n}\n```", false},
		{"snippet with ellipsis placeholder", "```go\nfunc f() {\n  // ...\n}\n```", false},
		{"full solution without placeholder", "```go\nfunc Hello(n string) string { return \"Hello, \" + n }\n```", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(domain.L3ConstrainedSnippet, tc.content)
			if (err != nil) != tc.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

func TestClampValidator_L4_L5_Pass(t *testing.T) {
	v := NewClampValidator()
	full := "Here is the solution:\n```go\nfunc Hello(n string) string { return \"Hello, \" + n }\n```"

	if err := v.Validate(domain.L4PartialSolution, full); err != nil {
		t.Errorf("L4 should not enforce clamp, got %v", err)
	}
	if err := v.Validate(domain.L5FullSolution, full); err != nil {
		t.Errorf("L5 should not enforce clamp, got %v", err)
	}
}

func TestClampValidator_AdversarialPromptInjection(t *testing.T) {
	// Adversarial samples representing model jailbreak attempts that bypass
	// the system prompt clamp. These must be caught output-side.
	v := NewClampValidator()

	adversarial := []struct {
		name    string
		level   domain.InterventionLevel
		content string
	}{
		{
			"L1 model emits solution despite category-only directive",
			domain.L1CategoryHint,
			"Sure, here's how:\n\n```go\nreturn fmt.Sprintf(\"Hello, %s!\", name)\n```",
		},
		{
			"L0 model gives a hint instead of a question",
			domain.L0Clarify,
			"You should use fmt.Sprintf with a %s placeholder.",
		},
		{
			"L2 model leaks a code block in code fence variants",
			domain.L2LocationConcept,
			"Look here.\n\n   ```\nif name == \"\" { name = \"World\" }\n```",
		},
		{
			"L3 model emits full solution without placeholder",
			domain.L3ConstrainedSnippet,
			"```go\nfunc Hello(name string) string {\n  if name == \"\" { name = \"World\" }\n  return fmt.Sprintf(\"Hello, %s!\", name)\n}\n```",
		},
	}

	for _, tc := range adversarial {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.level, tc.content)
			if err == nil {
				t.Errorf("expected clamp violation for adversarial sample, got nil")
			}
			if !errors.Is(err, ErrClampViolation) {
				t.Errorf("expected ErrClampViolation, got %v", err)
			}
		})
	}
}

func TestClampValidator_Sanitize(t *testing.T) {
	v := NewClampValidator()

	in := "Try this:\n```go\nreturn fmt.Sprintf(\"hi\")\n```\nDoes that make sense?"
	out := v.Sanitize(domain.L1CategoryHint, in)

	if strings.Contains(out, "```") {
		t.Errorf("sanitized output still contains fenced code: %q", out)
	}
	if strings.Contains(out, "fmt.Sprintf") {
		t.Errorf("sanitized output still contains code text: %q", out)
	}
	if !strings.Contains(out, "Does that make sense?") {
		t.Errorf("sanitized output dropped surrounding prose: %q", out)
	}
}

func TestClampValidator_Sanitize_NoOpAtHigherLevels(t *testing.T) {
	v := NewClampValidator()
	in := "```go\nreturn 1\n```"
	if out := v.Sanitize(domain.L3ConstrainedSnippet, in); out != in {
		t.Errorf("Sanitize at L3 should be no-op, got %q", out)
	}
	if out := v.Sanitize(domain.L4PartialSolution, in); out != in {
		t.Errorf("Sanitize at L4 should be no-op, got %q", out)
	}
}

func TestClampValidator_TighteningDirective(t *testing.T) {
	v := NewClampValidator()
	d := v.TighteningDirective(domain.L1CategoryHint, "fenced code present")
	if !strings.Contains(d, "L1") {
		t.Errorf("directive must mention level: %q", d)
	}
	if !strings.Contains(d, "NO code blocks") {
		t.Errorf("directive must restate the rule: %q", d)
	}
}

func TestClampViolations_CounterIncrements(t *testing.T) {
	before := ClampViolations()

	v := NewClampValidator()
	_ = v.Validate(domain.L0Clarify, "Just do this:\n```go\nreturn 1\n```")
	_ = v.Validate(domain.L1CategoryHint, "```\nfmt.Sprintf\n```")

	after := ClampViolations()
	if after-before < 2 {
		t.Errorf("expected counter to increment at least twice, before=%d after=%d", before, after)
	}
}
