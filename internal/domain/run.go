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
	FormatOK    bool          // gofmt passed
	FormatDiff  string        // gofmt diff output (if any changes needed)
	BuildOK     bool          // go build passed
	BuildErrors []Diagnostic  // compilation errors
	TestResults []TestResult  // individual test results
	TestsPassed int           // count of passing tests
	TestsFailed int           // count of failing tests
	Duration    time.Duration // total execution time
	Logs        string        // full output logs
}

// Diagnostic represents a compiler or lint error/warning
type Diagnostic struct {
	File     string
	Line     int
	Column   int
	Severity string // error, warning
	Message  string
}

// TestResult represents the outcome of a single test
type TestResult struct {
	Package  string
	Name     string
	Passed   bool
	Duration time.Duration
	Output   string
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
