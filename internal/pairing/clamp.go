package pairing

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// ErrClampViolation is returned when an LLM response exceeds the policy clamp
// for the selected intervention level.
var ErrClampViolation = errors.New("clamp violation")

// ClampViolation describes how a response violated the clamp rules.
type ClampViolation struct {
	Level  domain.InterventionLevel
	Reason string
}

func (v *ClampViolation) Error() string {
	return fmt.Sprintf("clamp violation at L%d: %s", v.Level, v.Reason)
}

func (v *ClampViolation) Unwrap() error {
	return ErrClampViolation
}

var (
	codeFencePattern = regexp.MustCompile("(?m)^\\s*```")
	// Single-line indented code (4+ spaces or tab) at the start of a line.
	indentedCodeLine = regexp.MustCompile(`(?m)^(\t| {4,})\S`)
	// Placeholder markers indicating a constrained snippet rather than full solution.
	placeholderHints = []string{
		"// TODO",
		"// implement",
		"// your logic",
		"// your code",
		"// fill in",
		"# TODO",
		"# implement",
		"// ...",
		"# ...",
	}

	clampViolationCounter atomic.Int64
)

// ClampViolations returns the cumulative count of detected clamp violations
// since process start. Wired into the metrics endpoint.
func ClampViolations() int64 {
	return clampViolationCounter.Load()
}

// ClampValidator enforces level-appropriate output constraints on LLM responses.
//
// The selector picks a level based on policy + context, but the LLM may still
// over-help. This validator is the trust boundary: violation counts feed SLOs
// and the retry path tightens the system prompt.
type ClampValidator struct{}

// NewClampValidator returns a validator with the default rule table.
func NewClampValidator() *ClampValidator {
	return &ClampValidator{}
}

// Validate returns nil if content respects the level clamp, otherwise a
// *ClampViolation describing the rule that was broken.
func (v *ClampValidator) Validate(level domain.InterventionLevel, content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	switch level {
	case domain.L0Clarify:
		if hasCodeBlock(trimmed) {
			return record(level, "L0 clarify must not contain code blocks")
		}
		if !strings.Contains(trimmed, "?") {
			return record(level, "L0 clarify must contain a question")
		}
	case domain.L1CategoryHint:
		if hasCodeBlock(trimmed) {
			return record(level, "L1 category hint must not contain code blocks")
		}
	case domain.L2LocationConcept:
		if hasCodeBlock(trimmed) {
			return record(level, "L2 location/concept must not contain code blocks")
		}
	case domain.L3ConstrainedSnippet:
		if hasCodeBlock(trimmed) && !hasPlaceholder(trimmed) {
			return record(level, "L3 snippet must include placeholder markers (TODO/your logic/...)")
		}
	case domain.L4PartialSolution, domain.L5FullSolution:
		// Higher levels are gated by explicit user escalation; no output rules.
	}
	return nil
}

// Sanitize strips code blocks and indented code from content. Used as a
// last-resort fallback when retry still produces a violation: the user gets
// degraded but policy-compliant output rather than a broken promise.
func (v *ClampValidator) Sanitize(level domain.InterventionLevel, content string) string {
	if level > domain.L2LocationConcept {
		return content
	}

	stripped := stripFencedBlocks(content)
	stripped = indentedCodeLine.ReplaceAllString(stripped, "")
	stripped = strings.TrimSpace(stripped)
	if stripped == "" {
		return "[clamp-sanitized] No level-appropriate content was generated. Please rephrase your request."
	}
	return "[clamp-sanitized: code removed to respect L" +
		fmt.Sprintf("%d", int(level)) + " policy]\n\n" + stripped
}

// TighteningDirective returns an additional system-prompt fragment to inject
// on retry after a violation, restating the level constraint forcefully.
func (v *ClampValidator) TighteningDirective(level domain.InterventionLevel, reason string) string {
	return fmt.Sprintf(
		"\n\nCRITICAL OVERRIDE: Your previous response violated the L%d clamp (%s). "+
			"Generate a new response that contains NO code blocks and NO specific function names. "+
			"At this level, only conceptual prose and questions are permitted.",
		int(level), reason,
	)
}

func hasCodeBlock(s string) bool {
	if codeFencePattern.MatchString(s) {
		return true
	}
	if indentedCodeLine.MatchString(s) {
		return true
	}
	return false
}

func hasPlaceholder(s string) bool {
	lower := strings.ToLower(s)
	for _, hint := range placeholderHints {
		if strings.Contains(lower, strings.ToLower(hint)) {
			return true
		}
	}
	return false
}

func stripFencedBlocks(s string) string {
	var out strings.Builder
	inBlock := false
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inBlock = !inBlock
			continue
		}
		if inBlock {
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

func record(level domain.InterventionLevel, reason string) *ClampViolation {
	clampViolationCounter.Add(1)
	return &ClampViolation{Level: level, Reason: reason}
}
