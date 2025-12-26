package domain

import (
	"time"

	"github.com/google/uuid"
)

// Run represents a code execution
type Run struct {
	ID         uuid.UUID
	ArtifactID uuid.UUID
	UserID     uuid.UUID
	ExerciseID *string
	Status     RunStatus
	Recipe     CheckRecipe
	Output     *RunOutput
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

// RunStatus represents the current state of a run
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusTimeout   RunStatus = "timeout"
)

// RunOutput contains the results of a code execution
type RunOutput struct {
	FormatOK    bool          `json:"format_passed"`  // gofmt passed
	FormatDiff  string        `json:"format_output"`  // gofmt diff output (if any changes needed)
	BuildOK     bool          `json:"build_passed"`   // go build passed
	BuildOutput string        `json:"build_output"`   // build error output
	BuildErrors []Diagnostic  `json:"build_errors"`   // compilation errors
	TestOK      bool          `json:"test_passed"`    // all tests passed
	TestOutput  string        `json:"test_output"`    // test output
	TestResults []TestResult  `json:"test_results"`   // individual test results
	TestsPassed int           `json:"tests_passed"`   // count of passing tests
	TestsFailed int           `json:"tests_failed"`   // count of failing tests
	Duration    time.Duration `json:"duration"`       // total execution time
	Logs        string        `json:"logs"`           // full output logs
}

// Diagnostic represents a compiler or lint error/warning
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // error, warning
	Message  string `json:"message"`
}

// TestResult represents the outcome of a single test
type TestResult struct {
	Package  string        `json:"package"`
	Name     string        `json:"name"`
	Passed   bool          `json:"passed"`
	Duration time.Duration `json:"elapsed"`
	Output   string        `json:"output,omitempty"`
}

// IsTerminal returns true if the run is in a terminal state
func (r *Run) IsTerminal() bool {
	return r.Status == RunStatusCompleted ||
		r.Status == RunStatusFailed ||
		r.Status == RunStatusTimeout
}

// Success returns true if the run completed successfully with all tests passing
func (r *Run) Success() bool {
	if r.Output == nil {
		return false
	}
	return r.Status == RunStatusCompleted &&
		r.Output.BuildOK &&
		r.Output.TestsFailed == 0
}

// HasErrors returns true if there are any build errors
func (o *RunOutput) HasErrors() bool {
	return len(o.BuildErrors) > 0
}

// AllTestsPassed returns true if all tests passed
func (o *RunOutput) AllTestsPassed() bool {
	return o.TestsFailed == 0 && o.TestsPassed > 0
}
