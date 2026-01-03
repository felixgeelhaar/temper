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

	// Spec context for feature guidance sessions
	Spec           *domain.ProductSpec
	FocusCriterion *domain.AcceptanceCriterion
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

	// Spec context for feature guidance sessions
	if req.Spec != nil {
		sb.WriteString(p.buildSpecContext(req.Spec, req.FocusCriterion))
	}

	// Final instruction
	sb.WriteString("## Your Task\n\n")
	sb.WriteString(p.taskInstruction(req.Intent, req.Level, req.Type))

	// Add spec-specific instructions if applicable
	if req.Spec != nil {
		sb.WriteString(p.specTaskAddendum(req.Spec, req.FocusCriterion))
	}

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

// buildSpecContext creates the spec context section for feature guidance prompts
func (p *Prompter) buildSpecContext(spec *domain.ProductSpec, focus *domain.AcceptanceCriterion) string {
	var sb strings.Builder

	sb.WriteString("## Product Specification Context\n\n")
	sb.WriteString(fmt.Sprintf("**Spec**: %s (v%s)\n\n", spec.Name, spec.Version))

	// Goals provide high-level direction
	if len(spec.Goals) > 0 {
		sb.WriteString("### Goals\n")
		for _, goal := range spec.Goals {
			sb.WriteString(fmt.Sprintf("- %s\n", goal))
		}
		sb.WriteString("\n")
	}

	// Current focus criterion
	if focus != nil {
		sb.WriteString("### Current Focus\n")
		sb.WriteString(fmt.Sprintf("**Acceptance Criterion %s**: %s\n", focus.ID, focus.Description))
		if focus.Satisfied {
			sb.WriteString("Status: ✓ Satisfied\n")
		} else {
			sb.WriteString("Status: ⏳ In Progress\n")
		}
		sb.WriteString("\n")
	}

	// Progress overview
	satisfied := 0
	for _, ac := range spec.AcceptanceCriteria {
		if ac.Satisfied {
			satisfied++
		}
	}
	sb.WriteString(fmt.Sprintf("### Progress: %d/%d criteria satisfied\n\n", satisfied, len(spec.AcceptanceCriteria)))

	// Relevant features for context
	sb.WriteString("### Features in Scope\n")
	for _, feat := range spec.Features {
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", feat.Title, feat.Priority, p.truncate(feat.Description, 100)))
		if feat.API != nil {
			sb.WriteString(fmt.Sprintf("  - API: %s %s\n", feat.API.Method, feat.API.Path))
		}
	}
	sb.WriteString("\n")

	// Non-functional requirements as constraints
	if len(spec.NonFunctional.Performance) > 0 || len(spec.NonFunctional.Security) > 0 {
		sb.WriteString("### Non-Functional Requirements\n")
		for _, req := range spec.NonFunctional.Performance {
			sb.WriteString(fmt.Sprintf("- Performance: %s\n", req))
		}
		for _, req := range spec.NonFunctional.Security {
			sb.WriteString(fmt.Sprintf("- Security: %s\n", req))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// specTaskAddendum adds spec-specific instructions to the task
func (p *Prompter) specTaskAddendum(spec *domain.ProductSpec, focus *domain.AcceptanceCriterion) string {
	var sb strings.Builder

	sb.WriteString("\n\n### Spec-Anchored Guidance\n")
	sb.WriteString("When providing feedback, always anchor to the product specification:\n")
	sb.WriteString("- Focus on the current acceptance criterion\n")
	sb.WriteString("- Keep suggestions within the defined feature scope\n")
	sb.WriteString("- Reference non-functional requirements when relevant\n")

	if focus != nil && !focus.Satisfied {
		sb.WriteString(fmt.Sprintf("\nThe learner is working toward: **%s**\n", focus.Description))
		sb.WriteString("Guide them to satisfy this criterion without solving it for them.\n")
	}

	// Warn about scope drift
	sb.WriteString("\nIf the learner's code is drifting outside the spec scope, gently guide them back.\n")
	sb.WriteString("Reference the spec's goals and features to keep the implementation focused.\n")

	return sb.String()
}

// AuthoringSystemPrompt returns the system prompt for spec authoring
func (p *Prompter) AuthoringSystemPrompt(section string) string {
	return fmt.Sprintf(`You are a product specification assistant helping extract and organize requirements from project documentation.

Your role is to:
1. Analyze project documentation (vision, PRD, TDD, roadmap)
2. Extract relevant information for each spec section
3. Suggest well-structured entries matching the Specular YAML format
4. Cite sources for each suggestion (document path + section heading)

IMPORTANT CONSTRAINTS:
- Base all suggestions on the provided documentation
- Always cite sources in the format: "Source: docs/file.md#Section"
- Format suggestions to match Specular YAML structure exactly
- Ask clarifying questions when documentation is ambiguous
- Prioritize accuracy over completeness

Current focus: %s section

When suggesting entries, provide them as structured suggestions that can be directly inserted into the spec.`, section)
}

// BuildAuthoringPrompt creates a prompt for generating spec section suggestions
func (p *Prompter) BuildAuthoringPrompt(ctx AuthoringContext) string {
	var sb strings.Builder

	// Current spec state
	sb.WriteString("## Current Spec\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", ctx.Spec.Name))
	sb.WriteString(fmt.Sprintf("Version: %s\n\n", ctx.Spec.Version))

	// Show existing content for context
	switch ctx.Section {
	case "goals":
		sb.WriteString("### Current Goals\n")
		if len(ctx.Spec.Goals) == 0 {
			sb.WriteString("(none yet)\n")
		} else {
			for _, g := range ctx.Spec.Goals {
				sb.WriteString(fmt.Sprintf("- %s\n", g))
			}
		}
	case "features":
		sb.WriteString("### Current Features\n")
		if len(ctx.Spec.Features) == 0 {
			sb.WriteString("(none yet)\n")
		} else {
			for _, f := range ctx.Spec.Features {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Title, f.Description))
			}
		}
	case "acceptance_criteria":
		sb.WriteString("### Current Acceptance Criteria\n")
		if len(ctx.Spec.AcceptanceCriteria) == 0 {
			sb.WriteString("(none yet)\n")
		} else {
			for _, ac := range ctx.Spec.AcceptanceCriteria {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", ac.ID, ac.Description))
			}
		}
	case "non_functional":
		sb.WriteString("### Current Non-Functional Requirements\n")
		if len(ctx.Spec.NonFunctional.Performance) == 0 && len(ctx.Spec.NonFunctional.Security) == 0 {
			sb.WriteString("(none yet)\n")
		} else {
			for _, p := range ctx.Spec.NonFunctional.Performance {
				sb.WriteString(fmt.Sprintf("- Performance: %s\n", p))
			}
			for _, s := range ctx.Spec.NonFunctional.Security {
				sb.WriteString(fmt.Sprintf("- Security: %s\n", s))
			}
		}
	}
	sb.WriteString("\n")

	// Document context
	sb.WriteString("## Project Documentation\n\n")
	docContent := ctx.GetDocumentContent(4000) // ~1000 tokens for docs
	sb.WriteString(docContent)
	sb.WriteString("\n")

	// Task instruction
	sb.WriteString(fmt.Sprintf("## Task: Suggest entries for the '%s' section\n\n", ctx.Section))
	sb.WriteString(p.sectionInstructions(ctx.Section))

	return sb.String()
}

// sectionInstructions returns specific instructions for each section type
func (p *Prompter) sectionInstructions(section string) string {
	switch section {
	case "goals":
		return `Extract 3-5 high-level goals from the vision/PRD documents.
Each goal should be:
- Actionable and measurable
- Business-outcome focused
- One clear sentence

For each suggestion, provide:
1. The goal text
2. The source document and section
3. Your confidence level (high/medium/low)

Format your response as a numbered list with sources.`

	case "features":
		return `Identify distinct features from the PRD/TDD documents.
For each feature provide:
- id: kebab-case identifier (e.g., "user-authentication")
- title: Clear feature name
- description: 1-2 sentence description
- priority: high/medium/low
- success_criteria: List of measurable criteria
- api (if applicable): method and path

For each suggestion, cite the source document and section.
Format as YAML-compatible entries.`

	case "acceptance_criteria":
		return `Derive acceptance criteria from features and requirements.
Each criterion should be:
- Verifiable through testing
- Specific and measurable
- Linked to a feature or goal

Provide:
- id: Short identifier (e.g., "ac-1")
- description: Clear acceptance criterion text
- source: Where this was derived from

Format as a numbered list with YAML-compatible structure.`

	case "non_functional":
		return `Extract non-functional requirements from TDD and design docs.
Categories: performance, security, scalability, availability

Each should be:
- Specific and measurable (e.g., "Response time < 200ms")
- Categorized appropriately
- Sourced from documentation

Format as categorized lists (Performance, Security, Scalability).`

	default:
		return "Analyze the documentation and suggest appropriate content for this section."
	}
}

// BuildAuthoringHintPrompt creates a prompt for authoring hints
func (p *Prompter) BuildAuthoringHintPrompt(ctx AuthoringContext) string {
	var sb strings.Builder

	// Spec context
	sb.WriteString("## Spec Being Authored\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", ctx.Spec.Name))
	sb.WriteString(fmt.Sprintf("Current Section: %s\n\n", ctx.Section))

	// Document context
	sb.WriteString("## Available Documentation\n\n")
	docContent := ctx.GetDocumentContent(3000)
	sb.WriteString(docContent)
	sb.WriteString("\n")

	// User question
	sb.WriteString("## User Question\n\n")
	if ctx.Question != "" {
		sb.WriteString(ctx.Question)
	} else {
		sb.WriteString("Help me populate this section of the spec based on the project docs.")
	}
	sb.WriteString("\n\n")

	sb.WriteString("Provide a helpful answer based on the documentation. Cite sources when possible.")

	return sb.String()
}

// ParseSuggestions extracts structured suggestions from LLM response
func (p *Prompter) ParseSuggestions(content, section string) []domain.AuthoringSuggestion {
	var suggestions []domain.AuthoringSuggestion

	// Simple parsing: split by numbered items or bullet points
	// This is a basic implementation - could be enhanced with more sophisticated parsing
	lines := strings.Split(content, "\n")
	var currentSuggestion *domain.AuthoringSuggestion
	var currentValue strings.Builder
	suggestionID := 1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check if this starts a new suggestion (numbered or bulleted)
		isNewItem := false
		if len(trimmed) > 2 {
			// Check for numbered items: "1.", "2.", etc.
			if trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
				isNewItem = true
			}
			// Check for bullet points: "- ", "* "
			if (trimmed[0] == '-' || trimmed[0] == '*') && trimmed[1] == ' ' {
				isNewItem = true
			}
		}

		if isNewItem {
			// Save previous suggestion
			if currentSuggestion != nil && currentValue.Len() > 0 {
				currentSuggestion.Value = strings.TrimSpace(currentValue.String())
				suggestions = append(suggestions, *currentSuggestion)
			}

			// Start new suggestion
			currentSuggestion = &domain.AuthoringSuggestion{
				ID:         fmt.Sprintf("sug-%d", suggestionID),
				Section:    section,
				Confidence: 0.8, // Default confidence
			}
			suggestionID++
			currentValue.Reset()

			// Extract the content after the bullet/number
			content := strings.TrimSpace(trimmed[2:])
			currentValue.WriteString(content)

			// Check for source citation
			if idx := strings.Index(strings.ToLower(trimmed), "source:"); idx != -1 {
				currentSuggestion.Source = strings.TrimSpace(trimmed[idx+7:])
			}
		} else if currentSuggestion != nil {
			// Continue current suggestion
			currentValue.WriteString(" ")
			currentValue.WriteString(trimmed)

			// Check for source citation in continuation
			if idx := strings.Index(strings.ToLower(trimmed), "source:"); idx != -1 {
				currentSuggestion.Source = strings.TrimSpace(trimmed[idx+7:])
			}
		}
	}

	// Save last suggestion
	if currentSuggestion != nil && currentValue.Len() > 0 {
		currentSuggestion.Value = strings.TrimSpace(currentValue.String())
		suggestions = append(suggestions, *currentSuggestion)
	}

	return suggestions
}
