package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRun_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status RunStatus
		want   bool
	}{
		{"pending", RunStatusPending, false},
		{"running", RunStatusRunning, false},
		{"completed", RunStatusCompleted, true},
		{"failed", RunStatusFailed, true},
		{"timeout", RunStatusTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &Run{ID: uuid.New(), Status: tt.status}
			if got := run.IsTerminal(); got != tt.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRun_Success(t *testing.T) {
	tests := []struct {
		name   string
		status RunStatus
		output *RunOutput
		want   bool
	}{
		{"nil output", RunStatusCompleted, nil, false},
		{"completed with passing tests", RunStatusCompleted, &RunOutput{BuildOK: true, TestsFailed: 0}, true},
		{"completed with failing tests", RunStatusCompleted, &RunOutput{BuildOK: true, TestsFailed: 1}, false},
		{"completed with build failure", RunStatusCompleted, &RunOutput{BuildOK: false, TestsFailed: 0}, false},
		{"failed status", RunStatusFailed, &RunOutput{BuildOK: true, TestsFailed: 0}, false},
		{"running status", RunStatusRunning, &RunOutput{BuildOK: true, TestsFailed: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &Run{ID: uuid.New(), Status: tt.status, Output: tt.output}
			if got := run.Success(); got != tt.want {
				t.Errorf("Success() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunOutput_HasErrors(t *testing.T) {
	tests := []struct {
		name   string
		errors []Diagnostic
		want   bool
	}{
		{"no errors", nil, false},
		{"empty errors", []Diagnostic{}, false},
		{"with errors", []Diagnostic{{Message: "error"}}, true},
		{"multiple errors", []Diagnostic{{Message: "error1"}, {Message: "error2"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &RunOutput{BuildErrors: tt.errors}
			if got := output.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunOutput_AllTestsPassed(t *testing.T) {
	tests := []struct {
		name        string
		testsPassed int
		testsFailed int
		want        bool
	}{
		{"all passed", 5, 0, true},
		{"some failed", 3, 2, false},
		{"all failed", 0, 5, false},
		{"no tests", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &RunOutput{
				TestsPassed: tt.testsPassed,
				TestsFailed: tt.testsFailed,
			}
			if got := output.AllTestsPassed(); got != tt.want {
				t.Errorf("AllTestsPassed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunStatus_Constants(t *testing.T) {
	tests := []struct {
		status RunStatus
		want   string
	}{
		{RunStatusPending, "pending"},
		{RunStatusRunning, "running"},
		{RunStatusCompleted, "completed"},
		{RunStatusFailed, "failed"},
		{RunStatusTimeout, "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.status.String() != tt.want {
				t.Errorf("RunStatus = %q, want %q", tt.status.String(), tt.want)
			}
		})
	}
}

func TestRun_Struct(t *testing.T) {
	now := time.Now()
	startedAt := now
	finishedAt := now.Add(5 * time.Second)
	exerciseID := "go-v1/hello"

	run := &Run{
		ID:         uuid.New(),
		ArtifactID: uuid.New(),
		UserID:     uuid.New(),
		ExerciseID: &exerciseID,
		Status:     RunStatusCompleted,
		Recipe: CheckRecipe{
			Format: true,
			Build:  true,
			Test:   true,
		},
		Output:     &RunOutput{BuildOK: true},
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		CreatedAt:  now,
	}

	if run.Status != RunStatusCompleted {
		t.Errorf("Status = %q, want completed", run.Status)
	}
	if *run.ExerciseID != exerciseID {
		t.Errorf("ExerciseID = %q, want %q", *run.ExerciseID, exerciseID)
	}
}

func TestRunOutput_Struct(t *testing.T) {
	output := &RunOutput{
		FormatOK:    true,
		FormatDiff:  "",
		BuildOK:     true,
		BuildOutput: "",
		BuildErrors: []Diagnostic{},
		TestOK:      true,
		TestOutput:  "PASS",
		TestResults: []TestResult{
			{Name: "TestHello", Passed: true, Duration: time.Second},
		},
		TestsPassed: 1,
		TestsFailed: 0,
		Duration:    2 * time.Second,
		Logs:        "full logs",
		Risks: []RiskNotice{
			{ID: "SEC001", Severity: RiskSeverityMedium},
		},
	}

	if !output.FormatOK {
		t.Error("FormatOK should be true")
	}
	if len(output.TestResults) != 1 {
		t.Errorf("TestResults len = %d, want 1", len(output.TestResults))
	}
	if len(output.Risks) != 1 {
		t.Errorf("Risks len = %d, want 1", len(output.Risks))
	}
}

func TestDiagnostic_Struct(t *testing.T) {
	diag := Diagnostic{
		File:     "main.go",
		Line:     10,
		Column:   5,
		Severity: "error",
		Message:  "undefined: x",
	}

	if diag.File != "main.go" {
		t.Errorf("File = %q, want main.go", diag.File)
	}
	if diag.Line != 10 {
		t.Errorf("Line = %d, want 10", diag.Line)
	}
}

func TestTestResult_Struct(t *testing.T) {
	result := TestResult{
		Package:  "main",
		Name:     "TestHello",
		Passed:   true,
		Duration: 100 * time.Millisecond,
		Output:   "test output",
	}

	if result.Name != "TestHello" {
		t.Errorf("Name = %q, want TestHello", result.Name)
	}
	if !result.Passed {
		t.Error("Passed should be true")
	}
}

func TestRiskNotice_Struct(t *testing.T) {
	notice := RiskNotice{
		ID:          "SEC001",
		Category:    RiskCategorySecurity,
		Severity:    RiskSeverityHigh,
		Title:       "SQL Injection Risk",
		Description: "User input not sanitized",
		File:        "db.go",
		Line:        42,
		Suggestion:  "Use parameterized queries",
	}

	if notice.Category != RiskCategorySecurity {
		t.Errorf("Category = %q, want security", notice.Category)
	}
	if notice.Severity != RiskSeverityHigh {
		t.Errorf("Severity = %q, want high", notice.Severity)
	}
}

func TestRiskCategory_Constants(t *testing.T) {
	tests := []struct {
		category RiskCategory
		want     string
	}{
		{RiskCategorySecurity, "security"},
		{RiskCategoryQuality, "quality"},
		{RiskCategoryPerformance, "performance"},
		{RiskCategoryReliability, "reliability"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.category) != tt.want {
				t.Errorf("RiskCategory = %q, want %q", tt.category, tt.want)
			}
		})
	}
}

func TestRiskSeverity_Constants(t *testing.T) {
	tests := []struct {
		severity RiskSeverity
		want     string
	}{
		{RiskSeverityHigh, "high"},
		{RiskSeverityMedium, "medium"},
		{RiskSeverityLow, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.severity) != tt.want {
				t.Errorf("RiskSeverity = %q, want %q", tt.severity, tt.want)
			}
		})
	}
}
