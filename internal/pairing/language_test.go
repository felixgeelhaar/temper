package pairing

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestSystemPromptForLanguage_NamesLanguage(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"go", "Go"},
		{"python", "Python"},
		{"typescript", "TypeScript"},
		{"java", "Java"},
		{"rust", "Rust"},
		{"c", "C"},
	}

	p := NewPrompter()
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			out := p.SystemPromptForLanguage(domain.L1CategoryHint, tc.lang)
			if !strings.Contains(out, "practice "+tc.want) {
				t.Errorf("expected %q in system prompt, got: %s", "practice "+tc.want, out)
			}
		})
	}
}

func TestSystemPromptForLanguage_EmptyFallback(t *testing.T) {
	p := NewPrompter()
	out := p.SystemPromptForLanguage(domain.L1CategoryHint, "")
	if !strings.Contains(out, "the chosen programming language") {
		t.Errorf("empty language should use generic phrase, got: %s", out)
	}
	if strings.Contains(out, "practice Go.") {
		t.Errorf("empty language must not hardcode Go: %s", out)
	}
}

func TestSystemPromptForLanguage_PythonUsesHashComments(t *testing.T) {
	p := NewPrompter()
	out := p.SystemPromptForLanguage(domain.L3ConstrainedSnippet, "python")
	if !strings.Contains(out, "# your logic here") {
		t.Errorf("Python L3 placeholder must use # comment marker, got: %s", out)
	}
	if strings.Contains(out, "// your logic here") {
		t.Errorf("Python L3 must not use // comment marker, got: %s", out)
	}
}

func TestSystemPromptForLanguage_GoUsesSlashComments(t *testing.T) {
	p := NewPrompter()
	out := p.SystemPromptForLanguage(domain.L3ConstrainedSnippet, "go")
	if !strings.Contains(out, "// your logic here") {
		t.Errorf("Go L3 placeholder must use // comment marker, got: %s", out)
	}
}

func TestSystemPrompt_DefaultIsLanguageAgnostic(t *testing.T) {
	// SystemPrompt() with no language argument must not reference Go anymore;
	// fix for the hardcoded "practice Go" claim flagged in the audit.
	p := NewPrompter()
	out := p.SystemPrompt(domain.L1CategoryHint)
	if strings.Contains(out, "practice Go.") {
		t.Errorf("default SystemPrompt must not hardcode Go: %s", out)
	}
}

func TestExerciseLanguage_Helper(t *testing.T) {
	if got := exerciseLanguage(nil); got != "" {
		t.Errorf("nil exercise → got %q, want empty", got)
	}
	if got := exerciseLanguage(&domain.Exercise{Language: "python"}); got != "python" {
		t.Errorf("got %q, want python", got)
	}
}

func TestLanguageIdiomExample_PerLanguage(t *testing.T) {
	// Each language gets a distinct idiom example so the L3 prompt is
	// not generic. Verify the per-language tail differs.
	seen := map[string]string{}
	for _, l := range []string{"go", "python", "typescript", "java", "rust", "c"} {
		ex := languageIdiomExample(l)
		if prev, ok := seen[ex]; ok {
			t.Errorf("idiom example collision: %s and %s both yield %q", l, prev, ex)
		}
		seen[ex] = l
	}
}
