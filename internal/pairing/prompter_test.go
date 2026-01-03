package pairing

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestNewPrompter(t *testing.T) {
	p := NewPrompter()
	if p == nil {
		t.Fatal("NewPrompter() returned nil")
	}
}

func TestPrompter_SystemPrompt(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name    string
		level   domain.InterventionLevel
		contain string
	}{
		{
			name:    "L0 clarify",
			level:   domain.L0Clarify,
			contain: "ONLY ask clarifying questions",
		},
		{
			name:    "L1 category hint",
			level:   domain.L1CategoryHint,
			contain: "CATEGORY or DIRECTION",
		},
		{
			name:    "L2 location concept",
			level:   domain.L2LocationConcept,
			contain: "LOCATION of the issue",
		},
		{
			name:    "L3 constrained snippet",
			level:   domain.L3ConstrainedSnippet,
			contain: "CONSTRAINED snippet",
		},
		{
			name:    "default level",
			level:   domain.L4PartialSolution,
			contain: "Provide appropriate guidance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.SystemPrompt(tt.level)
			if !strings.Contains(got, tt.contain) {
				t.Errorf("SystemPrompt(%v) should contain %q", tt.level, tt.contain)
			}
		})
	}
}

func TestPrompter_BuildPrompt_Exercise(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Exercise: &domain.Exercise{
			Title:       "Hello World",
			Description: "Write a hello world program",
		},
	}

	prompt := p.BuildPrompt(req)

	if !strings.Contains(prompt, "Hello World") {
		t.Error("Prompt should contain exercise title")
	}
	if !strings.Contains(prompt, "Write a hello world program") {
		t.Error("Prompt should contain exercise description")
	}
}

func TestPrompter_BuildPrompt_Code(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() {}",
		},
	}

	prompt := p.BuildPrompt(req)

	if !strings.Contains(prompt, "main.go") {
		t.Error("Prompt should contain filename")
	}
	if !strings.Contains(prompt, "package main") {
		t.Error("Prompt should contain code content")
	}
}

func TestPrompter_BuildPrompt_RunOutput(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name    string
		output  *domain.RunOutput
		contain string
	}{
		{
			name: "build success",
			output: &domain.RunOutput{
				BuildOK:   true,
				FormatOK:  true,
				TestsPassed: 3,
				TestsFailed: 0,
			},
			contain: "Build: ✓ Success",
		},
		{
			name: "build failed",
			output: &domain.RunOutput{
				BuildOK: false,
				BuildErrors: []domain.Diagnostic{
					{File: "main.go", Line: 5, Message: "undefined: x"},
				},
			},
			contain: "Build: ✗ Failed",
		},
		{
			name: "format needs work",
			output: &domain.RunOutput{
				BuildOK:  true,
				FormatOK: false,
			},
			contain: "Format: ✗ Needs formatting",
		},
		{
			name: "failing tests",
			output: &domain.RunOutput{
				BuildOK:     true,
				FormatOK:    true,
				TestsPassed: 2,
				TestsFailed: 1,
				TestResults: []domain.TestResult{
					{Name: "TestFoo", Passed: false, Output: "expected 1, got 2"},
				},
			},
			contain: "Failing tests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PromptRequest{
				Intent: domain.IntentHint,
				Level:  domain.L1CategoryHint,
				Type:   domain.TypeHint,
				Output: tt.output,
			}

			prompt := p.BuildPrompt(req)
			if !strings.Contains(prompt, tt.contain) {
				t.Errorf("Prompt should contain %q", tt.contain)
			}
		})
	}
}

func TestPrompter_BuildPrompt_ExerciseHints(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Exercise: &domain.Exercise{
			Title: "Test Exercise",
			Hints: domain.HintSet{
				L1: []string{"Think about string formatting"},
			},
		},
	}

	prompt := p.BuildPrompt(req)

	if !strings.Contains(prompt, "Available Hints") {
		t.Error("Prompt should contain hints section")
	}
	if !strings.Contains(prompt, "string formatting") {
		t.Error("Prompt should contain hint content")
	}
}

func TestPrompter_BuildPrompt_Spec(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Spec: &domain.ProductSpec{
			Name:    "Test Spec",
			Version: "1.0.0",
			Goals:   []string{"Build a great app"},
			Features: []domain.Feature{
				{
					ID:          "f1",
					Title:       "Login Feature",
					Description: "User can log in",
					Priority:    domain.PriorityHigh,
				},
			},
			AcceptanceCriteria: []domain.AcceptanceCriterion{
				{ID: "ac-1", Description: "User can log in", Satisfied: false},
				{ID: "ac-2", Description: "User can log out", Satisfied: true},
			},
		},
		FocusCriterion: &domain.AcceptanceCriterion{
			ID:          "ac-1",
			Description: "User can log in",
			Satisfied:   false,
		},
	}

	prompt := p.BuildPrompt(req)

	if !strings.Contains(prompt, "Product Specification Context") {
		t.Error("Prompt should contain spec context section")
	}
	if !strings.Contains(prompt, "Test Spec") {
		t.Error("Prompt should contain spec name")
	}
	if !strings.Contains(prompt, "Build a great app") {
		t.Error("Prompt should contain goals")
	}
	if !strings.Contains(prompt, "Current Focus") {
		t.Error("Prompt should contain focus section")
	}
	if !strings.Contains(prompt, "Spec-Anchored Guidance") {
		t.Error("Prompt should contain spec task addendum")
	}
}

func TestPrompter_BuildPrompt_SpecWithNonFunctional(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Spec: &domain.ProductSpec{
			Name:    "Test Spec",
			Version: "1.0.0",
			NonFunctional: domain.NonFunctionalReqs{
				Performance: []string{"Response time < 100ms"},
				Security:    []string{"HTTPS required"},
			},
		},
	}

	prompt := p.BuildPrompt(req)

	if !strings.Contains(prompt, "Non-Functional Requirements") {
		t.Error("Prompt should contain non-functional section")
	}
	if !strings.Contains(prompt, "Response time < 100ms") {
		t.Error("Prompt should contain performance requirements")
	}
	if !strings.Contains(prompt, "HTTPS required") {
		t.Error("Prompt should contain security requirements")
	}
}

func TestPrompter_BuildPrompt_SpecWithAPI(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Spec: &domain.ProductSpec{
			Name:    "Test Spec",
			Version: "1.0.0",
			Features: []domain.Feature{
				{
					ID:       "f1",
					Title:    "API Feature",
					Priority: domain.PriorityHigh,
					API: &domain.APISpec{
						Path:   "/api/v1/users",
						Method: "GET",
					},
				},
			},
		},
	}

	prompt := p.BuildPrompt(req)

	if !strings.Contains(prompt, "/api/v1/users") {
		t.Error("Prompt should contain API path")
	}
	if !strings.Contains(prompt, "GET") {
		t.Error("Prompt should contain API method")
	}
}

func TestPrompter_IntentDescription(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		intent  domain.Intent
		contain string
	}{
		{domain.IntentHint, "asking for a hint"},
		{domain.IntentReview, "code reviewed"},
		{domain.IntentStuck, "stuck"},
		{domain.IntentNext, "what to do next"},
		{domain.IntentExplain, "explanation"},
		{domain.Intent{}, "needs assistance"}, // Zero value for unknown
	}

	for _, tt := range tests {
		t.Run(tt.intent.String(), func(t *testing.T) {
			got := p.intentDescription(tt.intent)
			if !strings.Contains(got, tt.contain) {
				t.Errorf("intentDescription(%v) = %q; should contain %q", tt.intent, got, tt.contain)
			}
		})
	}
}

func TestPrompter_TaskInstruction(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		iType   domain.InterventionType
		contain string
	}{
		{domain.TypeQuestion, "clarifying question"},
		{domain.TypeHint, "category-level hint"},
		{domain.TypeNudge, "location of the issue"},
		{domain.TypeCritique, "constructive feedback"},
		{domain.TypeExplain, "Explain the concept"},
		{domain.TypeSnippet, "constrained snippet"},
		{domain.InterventionType{}, "appropriate guidance"}, // Zero value for unknown
	}

	for _, tt := range tests {
		t.Run(tt.iType.String(), func(t *testing.T) {
			got := p.taskInstruction(domain.IntentHint, domain.L1CategoryHint, tt.iType)
			if !strings.Contains(got, tt.contain) {
				t.Errorf("taskInstruction() for %v should contain %q", tt.iType, tt.contain)
			}
		})
	}
}

func TestPrompter_Truncate(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string truncated",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q; want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPrompter_SpecTaskAddendum_SatisfiedFocus(t *testing.T) {
	p := NewPrompter()

	spec := &domain.ProductSpec{Name: "Test"}
	focus := &domain.AcceptanceCriterion{
		ID:          "ac-1",
		Description: "Already done",
		Satisfied:   true,
	}

	result := p.specTaskAddendum(spec, focus)

	// Should not contain the "working toward" message for satisfied criterion
	if strings.Contains(result, "working toward: **Already done**") {
		t.Error("Should not show working toward for satisfied criterion")
	}
}
