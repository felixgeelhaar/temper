package pairing

import (
	"fmt"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Prompter builds prompts for the LLM
type Prompter struct{}

// NewPrompter creates a new prompter
func NewPrompter() *Prompter {
	return &Prompter{}
}

// PromptRequest contains data for building a prompt
type PromptRequest struct {
	Intent   domain.Intent
	Level    domain.InterventionLevel
	Type     domain.InterventionType
	Exercise *domain.Exercise
	Code     map[string]string
	Output   *domain.RunOutput
	Profile  *domain.LearningProfile
}

// SystemPrompt returns the system prompt for a given level
func (p *Prompter) SystemPrompt(level domain.InterventionLevel) string {
	base := `You are an adaptive programming tutor helping a learner practice Go.
Your goal is to help them understand and learn, NOT to solve the problem for them.
The learner should remain the author of their code at all times.

CRITICAL CONSTRAINTS based on intervention level:`

	switch level {
	case domain.L0Clarify:
		return base + `
- ONLY ask clarifying questions
- Do NOT provide any hints or solutions
- Help the learner articulate what they're trying to do
- Example: "What do you expect this function to return when the input is empty?"`

	case domain.L1CategoryHint:
		return base + `
- Hint at the CATEGORY or DIRECTION to explore
- Do NOT mention specific functions, methods, or syntax
- Do NOT show any code
- Example: "Think about how Go handles string formatting"`

	case domain.L2LocationConcept:
		return base + `
- Point to the LOCATION of the issue (file/function/line area)
- Explain the CONCEPT that applies
- Do NOT show code solutions
- Do NOT give step-by-step instructions
- Example: "The issue is in your Hello function. Consider how to check if a string is empty in Go."`

	case domain.L3ConstrainedSnippet:
		return base + `
- Provide a CONSTRAINED snippet or OUTLINE
- Show structure, not the complete solution
- Use placeholders like "// your logic here"
- Example: "Your function structure should be: check condition, then format string"`

	default:
		return base + `
- Provide appropriate guidance for the learner's level
- Always prefer less help over more
- The learner should do the thinking`
	}
}

// BuildPrompt constructs the user prompt for the LLM
func (p *Prompter) BuildPrompt(req PromptRequest) string {
	var sb strings.Builder

	// Exercise context
	if req.Exercise != nil {
		sb.WriteString(fmt.Sprintf("## Exercise: %s\n\n", req.Exercise.Title))
		sb.WriteString(fmt.Sprintf("%s\n\n", req.Exercise.Description))
	}

	// Learner intent
	sb.WriteString(fmt.Sprintf("## Learner Intent: %s\n\n", p.intentDescription(req.Intent)))

	// Intervention constraints
	sb.WriteString(fmt.Sprintf("## Intervention Level: L%d (%s)\n", req.Level, req.Level.String()))
	sb.WriteString(fmt.Sprintf("%s\n\n", req.Level.Description()))

	// Current code
	if len(req.Code) > 0 {
		sb.WriteString("## Current Code\n\n")
		for filename, content := range req.Code {
			sb.WriteString(fmt.Sprintf("### %s\n```go\n%s\n```\n\n", filename, content))
		}
	}

	// Run output
	if req.Output != nil {
		sb.WriteString("## Run Results\n\n")
		if req.Output.BuildOK {
			sb.WriteString("- Build: ✓ Success\n")
		} else {
			sb.WriteString("- Build: ✗ Failed\n")
			for _, diag := range req.Output.BuildErrors {
				sb.WriteString(fmt.Sprintf("  - %s:%d: %s\n", diag.File, diag.Line, diag.Message))
			}
		}

		if req.Output.FormatOK {
			sb.WriteString("- Format: ✓ Clean\n")
		} else {
			sb.WriteString("- Format: ✗ Needs formatting\n")
		}

		sb.WriteString(fmt.Sprintf("- Tests: %d passed, %d failed\n", req.Output.TestsPassed, req.Output.TestsFailed))

		if req.Output.TestsFailed > 0 {
			sb.WriteString("\nFailing tests:\n")
			for _, test := range req.Output.TestResults {
				if !test.Passed {
					sb.WriteString(fmt.Sprintf("- %s: %s\n", test.Name, p.truncate(test.Output, 200)))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Exercise hints for this level (if available)
	if req.Exercise != nil {
		hints := req.Exercise.GetHintsForLevel(req.Level)
		if len(hints) > 0 {
			sb.WriteString("## Available Hints (for reference)\n")
			for _, hint := range hints {
				sb.WriteString(fmt.Sprintf("- %s\n", hint))
			}
			sb.WriteString("\n")
		}
	}

	// Final instruction
	sb.WriteString("## Your Task\n\n")
	sb.WriteString(p.taskInstruction(req.Intent, req.Level, req.Type))

	return sb.String()
}

func (p *Prompter) intentDescription(intent domain.Intent) string {
	switch intent {
	case domain.IntentHint:
		return "The learner is asking for a hint"
	case domain.IntentReview:
		return "The learner wants their code reviewed"
	case domain.IntentStuck:
		return "The learner is stuck and needs help"
	case domain.IntentNext:
		return "The learner wants to know what to do next"
	case domain.IntentExplain:
		return "The learner wants an explanation"
	default:
		return "The learner needs assistance"
	}
}

func (p *Prompter) taskInstruction(intent domain.Intent, level domain.InterventionLevel, iType domain.InterventionType) string {
	switch iType {
	case domain.TypeQuestion:
		return `Provide a clarifying question that helps the learner think through the problem.
Do NOT give any hints or direction - just help them articulate their thinking.`

	case domain.TypeHint:
		return `Provide a category-level hint that points the learner in the right direction.
Be vague enough that they still need to discover the specific solution themselves.`

	case domain.TypeNudge:
		return `Point to the location of the issue and explain the relevant concept.
Help them understand WHAT to think about, but not exactly HOW to solve it.`

	case domain.TypeCritique:
		return `Review the code and provide constructive feedback.
Point out issues with the approach without giving the solution.
Help them understand why something might not work.`

	case domain.TypeExplain:
		return `Explain the concept that applies here.
Use analogies or examples if helpful.
Connect to the specific code location but don't solve it for them.`

	case domain.TypeSnippet:
		return `Provide a constrained snippet or outline.
Show the structure they should use, but leave the key logic for them to implement.
Use comments like "// implement condition here" as placeholders.`

	default:
		return `Provide appropriate guidance at the specified level.
Remember: less help is better. The learner should remain the author.`
	}
}

func (p *Prompter) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
