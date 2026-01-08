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
		name     string
		level    domain.InterventionLevel
		contains []string
	}{
		{
			name:  "L0 contains clarifying questions only",
			level: domain.L0Clarify,
			contains: []string{
				"ONLY ask clarifying questions",
				"Do NOT provide any hints",
			},
		},
		{
			name:  "L1 contains category hint",
			level: domain.L1CategoryHint,
			contains: []string{
				"CATEGORY or DIRECTION",
				"Do NOT mention specific functions",
				"Do NOT show any code",
			},
		},
		{
			name:  "L2 contains location concept",
			level: domain.L2LocationConcept,
			contains: []string{
				"LOCATION of the issue",
				"CONCEPT that applies",
				"Do NOT show code solutions",
			},
		},
		{
			name:  "L3 contains constrained snippet",
			level: domain.L3ConstrainedSnippet,
			contains: []string{
				"CONSTRAINED snippet",
				"OUTLINE",
				"placeholders",
			},
		},
		{
			name:  "L4 uses default guidance",
			level: domain.L4PartialSolution,
			contains: []string{
				"appropriate guidance",
				"prefer less help",
			},
		},
		{
			name:  "L5 uses default guidance",
			level: domain.L5FullSolution,
			contains: []string{
				"appropriate guidance",
				"learner should do the thinking",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.SystemPrompt(tt.level)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("SystemPrompt(%v) should contain %q", tt.level, expected)
				}
			}

			// All should contain base prompt
			if !strings.Contains(result, "adaptive programming tutor") {
				t.Errorf("SystemPrompt(%v) missing base prompt", tt.level)
			}
		})
	}
}

func TestPrompter_intentDescription(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name     string
		intent   domain.Intent
		expected string
	}{
		{
			name:     "IntentHint",
			intent:   domain.IntentHint,
			expected: "asking for a hint",
		},
		{
			name:     "IntentReview",
			intent:   domain.IntentReview,
			expected: "wants their code reviewed",
		},
		{
			name:     "IntentStuck",
			intent:   domain.IntentStuck,
			expected: "stuck and needs help",
		},
		{
			name:     "IntentNext",
			intent:   domain.IntentNext,
			expected: "wants to know what to do next",
		},
		{
			name:     "IntentExplain",
			intent:   domain.IntentExplain,
			expected: "wants an explanation",
		},
		{
			name:     "unknown intent",
			intent:   domain.Intent("unknown"),
			expected: "needs assistance",
		},
		{
			name:     "empty intent",
			intent:   domain.Intent(""),
			expected: "needs assistance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.intentDescription(tt.intent)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("intentDescription(%q) = %q, want to contain %q", tt.intent, result, tt.expected)
			}
		})
	}
}

func TestPrompter_taskInstruction(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name     string
		iType    domain.InterventionType
		contains []string
	}{
		{
			name:  "TypeQuestion",
			iType: domain.TypeQuestion,
			contains: []string{
				"clarifying question",
				"Do NOT give any hints",
			},
		},
		{
			name:  "TypeHint",
			iType: domain.TypeHint,
			contains: []string{
				"category-level hint",
				"discover the specific solution",
			},
		},
		{
			name:  "TypeNudge",
			iType: domain.TypeNudge,
			contains: []string{
				"location of the issue",
				"relevant concept",
			},
		},
		{
			name:  "TypeCritique",
			iType: domain.TypeCritique,
			contains: []string{
				"Review the code",
				"constructive feedback",
			},
		},
		{
			name:  "TypeExplain",
			iType: domain.TypeExplain,
			contains: []string{
				"Explain the concept",
				"analogies or examples",
			},
		},
		{
			name:  "TypeSnippet",
			iType: domain.TypeSnippet,
			contains: []string{
				"constrained snippet",
				"structure",
			},
		},
		{
			name:  "unknown type",
			iType: domain.InterventionType("unknown"),
			contains: []string{
				"appropriate guidance",
				"learner should remain the author",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.taskInstruction(domain.IntentHint, domain.L1CategoryHint, tt.iType)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("taskInstruction(_, _, %q) should contain %q, got %q", tt.iType, expected, result)
				}
			}
		})
	}
}

func TestPrompter_truncate(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated with ellipsis",
			input:    "hello world",
			maxLen:   5,
			expected: "hello...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "maxLen of 0",
			input:    "hello",
			maxLen:   0,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestPrompter_BuildPrompt_BasicStructure(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
	}

	result := p.BuildPrompt(req)

	// Should contain intent description
	if !strings.Contains(result, "Learner Intent") {
		t.Error("BuildPrompt should contain Learner Intent section")
	}

	// Should contain intervention level
	if !strings.Contains(result, "Intervention Level") {
		t.Error("BuildPrompt should contain Intervention Level section")
	}

	// Should contain task section
	if !strings.Contains(result, "Your Task") {
		t.Error("BuildPrompt should contain Your Task section")
	}
}

func TestPrompter_BuildPrompt_WithExercise(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentHint,
		Level:  domain.L1CategoryHint,
		Type:   domain.TypeHint,
		Exercise: &domain.Exercise{
			Title:       "Hello World",
			Description: "Write a function that returns a greeting",
			Hints: domain.HintSet{
				L1: []string{"Think about string formatting"},
			},
		},
	}

	result := p.BuildPrompt(req)

	// Should contain exercise title
	if !strings.Contains(result, "Hello World") {
		t.Error("BuildPrompt should contain exercise title")
	}

	// Should contain description
	if !strings.Contains(result, "Write a function") {
		t.Error("BuildPrompt should contain exercise description")
	}

	// Should contain hints
	if !strings.Contains(result, "string formatting") {
		t.Error("BuildPrompt should contain hints for level")
	}
}

func TestPrompter_BuildPrompt_WithCode(t *testing.T) {
	p := NewPrompter()

	req := PromptRequest{
		Intent: domain.IntentReview,
		Level:  domain.L2LocationConcept,
		Type:   domain.TypeCritique,
		Code: map[string]string{
			"main.go": "package main\n\nfunc Hello() string {\n\treturn \"\"\n}",
		},
	}

	result := p.BuildPrompt(req)

	// Should contain code section
	if !strings.Contains(result, "Current Code") {
		t.Error("BuildPrompt should contain Current Code section")
	}

	// Should contain filename
	if !strings.Contains(result, "main.go") {
		t.Error("BuildPrompt should contain filename")
	}

	// Should contain code content
	if !strings.Contains(result, "package main") {
		t.Error("BuildPrompt should contain code content")
	}
}

func TestPrompter_BuildPrompt_WithRunOutput(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name     string
		output   *domain.RunOutput
		contains []string
	}{
		{
			name: "successful build and tests",
			output: &domain.RunOutput{
				BuildOK:     true,
				FormatOK:    true,
				TestsPassed: 3,
				TestsFailed: 0,
			},
			contains: []string{
				"Build: ✓ Success",
				"Format: ✓ Clean",
				"3 passed, 0 failed",
			},
		},
		{
			name: "failed build with errors",
			output: &domain.RunOutput{
				BuildOK:  false,
				FormatOK: true,
				BuildErrors: []domain.Diagnostic{
					{File: "main.go", Line: 10, Message: "undefined: foo"},
				},
			},
			contains: []string{
				"Build: ✗ Failed",
				"main.go:10",
				"undefined: foo",
			},
		},
		{
			name: "format issues",
			output: &domain.RunOutput{
				BuildOK:  true,
				FormatOK: false,
			},
			contains: []string{
				"Format: ✗ Needs formatting",
			},
		},
		{
			name: "failing tests",
			output: &domain.RunOutput{
				BuildOK:     true,
				FormatOK:    true,
				TestsPassed: 2,
				TestsFailed: 1,
				TestResults: []domain.TestResult{
					{Name: "TestHello", Passed: false, Output: "expected 'Hello' but got ''"},
				},
			},
			contains: []string{
				"2 passed, 1 failed",
				"Failing tests:",
				"TestHello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PromptRequest{
				Intent: domain.IntentStuck,
				Level:  domain.L2LocationConcept,
				Type:   domain.TypeNudge,
				Output: tt.output,
			}

			result := p.BuildPrompt(req)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("BuildPrompt should contain %q for %s", expected, tt.name)
				}
			}
		})
	}
}

func TestPrompter_BuildPrompt_WithSpec(t *testing.T) {
	p := NewPrompter()

	spec := &domain.ProductSpec{
		Name:    "Test Product",
		Version: "1.0.0",
		Goals:   []string{"Goal 1", "Goal 2"},
		Features: []domain.Feature{
			{
				Title:       "User Auth",
				Priority:    domain.PriorityHigh,
				Description: "User authentication feature",
				API:         &domain.APISpec{Method: "POST", Path: "/auth/login"},
			},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "ac-1", Description: "Users can login", Satisfied: true},
			{ID: "ac-2", Description: "Users can logout", Satisfied: false},
		},
		NonFunctional: domain.NonFunctionalReqs{
			Performance: []string{"Response time < 200ms"},
			Security:    []string{"Use HTTPS"},
		},
	}

	req := PromptRequest{
		Intent:         domain.IntentHint,
		Level:          domain.L1CategoryHint,
		Type:           domain.TypeHint,
		Spec:           spec,
		FocusCriterion: &spec.AcceptanceCriteria[1],
	}

	result := p.BuildPrompt(req)

	// Should contain spec name
	if !strings.Contains(result, "Test Product") {
		t.Error("BuildPrompt should contain spec name")
	}

	// Should contain goals
	if !strings.Contains(result, "Goal 1") {
		t.Error("BuildPrompt should contain goals")
	}

	// Should contain focus criterion
	if !strings.Contains(result, "Users can logout") {
		t.Error("BuildPrompt should contain focus criterion")
	}

	// Should contain progress
	if !strings.Contains(result, "1/2") {
		t.Error("BuildPrompt should contain progress")
	}

	// Should contain feature
	if !strings.Contains(result, "User Auth") {
		t.Error("BuildPrompt should contain feature")
	}

	// Should contain API
	if !strings.Contains(result, "POST /auth/login") {
		t.Error("BuildPrompt should contain API")
	}

	// Should contain non-functional requirements
	if !strings.Contains(result, "Response time < 200ms") {
		t.Error("BuildPrompt should contain performance requirements")
	}

	// Should contain spec-anchored guidance
	if !strings.Contains(result, "Spec-Anchored") {
		t.Error("BuildPrompt should contain spec task addendum")
	}
}

func TestPrompter_buildSpecContext(t *testing.T) {
	p := NewPrompter()

	spec := &domain.ProductSpec{
		Name:    "My Spec",
		Version: "2.0",
		Goals:   []string{"Build something cool"},
		Features: []domain.Feature{
			{Title: "Feature A", Priority: domain.PriorityHigh, Description: "First feature"},
		},
		AcceptanceCriteria: []domain.AcceptanceCriterion{
			{ID: "1", Description: "Criteria 1", Satisfied: true},
			{ID: "2", Description: "Criteria 2", Satisfied: false},
		},
	}

	result := p.buildSpecContext(spec, nil)

	if !strings.Contains(result, "My Spec (v2.0)") {
		t.Error("buildSpecContext should contain spec name and version")
	}

	if !strings.Contains(result, "Build something cool") {
		t.Error("buildSpecContext should contain goals")
	}

	if !strings.Contains(result, "1/2 criteria satisfied") {
		t.Error("buildSpecContext should contain progress")
	}
}

func TestPrompter_specTaskAddendum(t *testing.T) {
	p := NewPrompter()

	spec := &domain.ProductSpec{}

	// Without focus criterion
	result := p.specTaskAddendum(spec, nil)
	if !strings.Contains(result, "Spec-Anchored Guidance") {
		t.Error("specTaskAddendum should contain anchored guidance header")
	}

	// With unsatisfied focus criterion
	focus := &domain.AcceptanceCriterion{
		Description: "Test criterion",
		Satisfied:   false,
	}
	result = p.specTaskAddendum(spec, focus)
	if !strings.Contains(result, "Test criterion") {
		t.Error("specTaskAddendum should contain criterion description")
	}
	if !strings.Contains(result, "satisfy this criterion") {
		t.Error("specTaskAddendum should contain satisfaction guidance")
	}

	// With satisfied focus criterion
	focus.Satisfied = true
	result = p.specTaskAddendum(spec, focus)
	if strings.Contains(result, "satisfy this criterion") {
		t.Error("specTaskAddendum should not mention satisfaction for satisfied criterion")
	}
}

func TestPrompter_sectionInstructions(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		section  string
		contains []string
	}{
		{
			section:  "goals",
			contains: []string{"3-5 high-level goals", "confidence level"},
		},
		{
			section:  "features",
			contains: []string{"id:", "title:", "priority:", "api"},
		},
		{
			section:  "acceptance_criteria",
			contains: []string{"Verifiable", "id:", "description:"},
		},
		{
			section:  "non_functional",
			contains: []string{"performance", "security", "measurable"},
		},
		{
			section:  "unknown",
			contains: []string{"Analyze the documentation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.section, func(t *testing.T) {
			result := p.sectionInstructions(tt.section)
			for _, expected := range tt.contains {
				if !strings.Contains(strings.ToLower(result), strings.ToLower(expected)) {
					t.Errorf("sectionInstructions(%q) should contain %q", tt.section, expected)
				}
			}
		})
	}
}

func TestPrompter_AuthoringSystemPrompt(t *testing.T) {
	p := NewPrompter()

	result := p.AuthoringSystemPrompt("features")

	if !strings.Contains(result, "product specification assistant") {
		t.Error("AuthoringSystemPrompt should describe role")
	}

	if !strings.Contains(result, "features section") {
		t.Error("AuthoringSystemPrompt should mention current section")
	}

	if !strings.Contains(result, "cite sources") {
		t.Error("AuthoringSystemPrompt should mention citation requirement")
	}
}

func TestPrompter_BuildAuthoringPrompt(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name     string
		section  string
		spec     *domain.ProductSpec
		contains []string
	}{
		{
			name:    "goals section empty",
			section: "goals",
			spec: &domain.ProductSpec{
				Name:    "Test",
				Version: "1.0",
			},
			contains: []string{
				"Current Goals",
				"(none yet)",
				"Task: Suggest entries for the 'goals' section",
			},
		},
		{
			name:    "goals section with existing",
			section: "goals",
			spec: &domain.ProductSpec{
				Name:    "Test",
				Version: "1.0",
				Goals:   []string{"Existing goal"},
			},
			contains: []string{
				"Existing goal",
			},
		},
		{
			name:    "features section",
			section: "features",
			spec: &domain.ProductSpec{
				Name:    "Test",
				Version: "1.0",
				Features: []domain.Feature{
					{Title: "Feature X", Description: "Test feature"},
				},
			},
			contains: []string{
				"Current Features",
				"Feature X",
			},
		},
		{
			name:    "acceptance_criteria section",
			section: "acceptance_criteria",
			spec: &domain.ProductSpec{
				Name:    "Test",
				Version: "1.0",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "ac-1", Description: "Test criterion"},
				},
			},
			contains: []string{
				"Current Acceptance Criteria",
				"[ac-1]",
				"Test criterion",
			},
		},
		{
			name:    "non_functional section",
			section: "non_functional",
			spec: &domain.ProductSpec{
				Name:    "Test",
				Version: "1.0",
				NonFunctional: domain.NonFunctionalReqs{
					Performance: []string{"Fast"},
					Security:    []string{"Secure"},
				},
			},
			contains: []string{
				"Non-Functional Requirements",
				"Performance: Fast",
				"Security: Secure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := AuthoringContext{
				Spec:    tt.spec,
				Section: tt.section,
			}

			result := p.BuildAuthoringPrompt(ctx)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("BuildAuthoringPrompt should contain %q for section %s", expected, tt.section)
				}
			}
		})
	}
}

func TestPrompter_BuildAuthoringHintPrompt(t *testing.T) {
	p := NewPrompter()

	ctx := AuthoringContext{
		Spec: &domain.ProductSpec{
			Name:    "Test Spec",
			Version: "1.0",
		},
		Section:  "features",
		Question: "How do I add authentication?",
	}

	result := p.BuildAuthoringHintPrompt(ctx)

	if !strings.Contains(result, "Test Spec") {
		t.Error("BuildAuthoringHintPrompt should contain spec name")
	}

	if !strings.Contains(result, "features") {
		t.Error("BuildAuthoringHintPrompt should contain section")
	}

	if !strings.Contains(result, "How do I add authentication") {
		t.Error("BuildAuthoringHintPrompt should contain user question")
	}

	// Test without question
	ctx.Question = ""
	result = p.BuildAuthoringHintPrompt(ctx)
	if !strings.Contains(result, "Help me populate") {
		t.Error("BuildAuthoringHintPrompt should contain default help text when no question")
	}
}

func TestPrompter_ParseSuggestions(t *testing.T) {
	p := NewPrompter()

	tests := []struct {
		name            string
		content         string
		section         string
		expectedCount   int
		checkFirst      func(*domain.AuthoringSuggestion) bool
	}{
		{
			name: "numbered list",
			content: `1. First suggestion
2. Second suggestion
3. Third suggestion`,
			section:       "goals",
			expectedCount: 3,
			checkFirst: func(s *domain.AuthoringSuggestion) bool {
				return strings.Contains(s.Value.(string), "First suggestion")
			},
		},
		{
			name: "bullet points",
			content: `- First item
- Second item`,
			section:       "goals",
			expectedCount: 2,
			checkFirst: func(s *domain.AuthoringSuggestion) bool {
				return strings.Contains(s.Value.(string), "First item")
			},
		},
		{
			name: "asterisk bullets",
			content: `* Item one
* Item two`,
			section:       "features",
			expectedCount: 2,
			checkFirst: func(s *domain.AuthoringSuggestion) bool {
				return s.Section == "features"
			},
		},
		{
			name: "with source citation",
			content: `1. Goal with source Source: docs/vision.md#Mission`,
			section:       "goals",
			expectedCount: 1,
			checkFirst: func(s *domain.AuthoringSuggestion) bool {
				return s.Source == "docs/vision.md#Mission"
			},
		},
		{
			name:            "empty content",
			content:         "",
			section:         "goals",
			expectedCount:   0,
			checkFirst:      nil,
		},
		{
			name: "multiline suggestion",
			content: `1. First part of suggestion
   continuing on next line
2. Second suggestion`,
			section:       "goals",
			expectedCount: 2,
			checkFirst: func(s *domain.AuthoringSuggestion) bool {
				return strings.Contains(s.Value.(string), "continuing")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := p.ParseSuggestions(tt.content, tt.section)

			if len(results) != tt.expectedCount {
				t.Errorf("ParseSuggestions returned %d suggestions, want %d", len(results), tt.expectedCount)
			}

			if tt.checkFirst != nil && len(results) > 0 {
				if !tt.checkFirst(&results[0]) {
					t.Errorf("First suggestion failed check: %+v", results[0])
				}
			}

			// Check all suggestions have IDs and sections
			for i, s := range results {
				if s.ID == "" {
					t.Errorf("Suggestion %d has empty ID", i)
				}
				if s.Section != tt.section {
					t.Errorf("Suggestion %d has section %q, want %q", i, s.Section, tt.section)
				}
			}
		})
	}
}
