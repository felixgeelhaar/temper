package domain

import "testing"

func TestRun_IsTerminal(t *testing.T) {
	terminal := []RunStatus{RunStatusCompleted, RunStatusFailed, RunStatusTimeout}
	for _, status := range terminal {
		r := Run{Status: status}
		if !r.IsTerminal() {
			t.Errorf("IsTerminal() should return true for %s", status)
		}
	}

	r := Run{Status: RunStatusRunning}
	if r.IsTerminal() {
		t.Error("IsTerminal() should return false for running status")
	}
}

func TestRun_Success(t *testing.T) {
	if (&Run{Status: RunStatusCompleted}).Success() {
		t.Error("Success() should return false when output is nil")
	}

	r := Run{
		Status: RunStatusCompleted,
		Output: &RunOutput{
			BuildOK:     true,
			TestsPassed: 3,
			TestsFailed: 0,
		},
	}
	if !r.Success() {
		t.Error("Success() should return true for completed runs with passing tests")
	}

	r.Output.TestsFailed = 1
	if r.Success() {
		t.Error("Success() should return false when tests fail")
	}
}

func TestRunOutput_StatusHelpers(t *testing.T) {
	output := &RunOutput{
		BuildErrors: []Diagnostic{{Message: "error"}},
		TestsPassed: 2,
		TestsFailed: 0,
	}

	if !output.HasErrors() {
		t.Error("HasErrors() should return true when build errors exist")
	}
	if !output.AllTestsPassed() {
		t.Error("AllTestsPassed() should return true when tests passed and none failed")
	}

	output.TestsPassed = 0
	if output.AllTestsPassed() {
		t.Error("AllTestsPassed() should return false when no tests passed")
	}
}
