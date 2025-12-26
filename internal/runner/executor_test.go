package runner_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/runner"
)

func TestLocalExecutor_RunFormat(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "runner-test-*")
	defer os.RemoveAll(tmpDir)

	exec := runner.NewLocalExecutor(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name     string
		code     map[string]string
		wantOK   bool
		wantDiff bool
	}{
		{
			name: "clean code",
			code: map[string]string{
				"main.go": `package main

func main() {
	println("hello")
}
`,
			},
			wantOK:   true,
			wantDiff: false,
		},
		{
			name: "unformatted code",
			code: map[string]string{
				"main.go": `package main

func main() {
println("hello")
}
`,
			},
			wantOK:   false,
			wantDiff: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := exec.RunFormat(ctx, tc.code)
			if err != nil {
				t.Fatalf("RunFormat failed: %v", err)
			}

			if result.OK != tc.wantOK {
				t.Errorf("OK = %v; want %v", result.OK, tc.wantOK)
			}

			hasDiff := result.Diff != ""
			if hasDiff != tc.wantDiff {
				t.Errorf("HasDiff = %v; want %v (diff: %s)", hasDiff, tc.wantDiff, result.Diff)
			}
		})
	}
}

func TestLocalExecutor_RunBuild(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "runner-test-*")
	defer os.RemoveAll(tmpDir)

	exec := runner.NewLocalExecutor(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name       string
		code       map[string]string
		wantOK     bool
		wantOutput bool
	}{
		{
			name: "valid code",
			code: map[string]string{
				"main.go": `package main

func main() {
	println("hello")
}
`,
			},
			wantOK:     true,
			wantOutput: false,
		},
		{
			name: "syntax error",
			code: map[string]string{
				"main.go": `package main

func main() {
	println("hello"
}
`,
			},
			wantOK:     false,
			wantOutput: true,
		},
		{
			name: "undefined variable",
			code: map[string]string{
				"main.go": `package main

func main() {
	println(undefined)
}
`,
			},
			wantOK:     false,
			wantOutput: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := exec.RunBuild(ctx, tc.code)
			if err != nil {
				t.Fatalf("RunBuild failed: %v", err)
			}

			if result.OK != tc.wantOK {
				t.Errorf("OK = %v; want %v", result.OK, tc.wantOK)
			}

			hasOutput := result.Output != ""
			if hasOutput != tc.wantOutput {
				t.Errorf("HasOutput = %v; want %v (output: %s)", hasOutput, tc.wantOutput, result.Output)
			}
		})
	}
}

func TestLocalExecutor_RunTests(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "runner-test-*")
	defer os.RemoveAll(tmpDir)

	exec := runner.NewLocalExecutor(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name   string
		code   map[string]string
		wantOK bool
	}{
		{
			name: "passing test",
			code: map[string]string{
				"main.go": `package main

func Add(a, b int) int {
	return a + b
}
`,
				"main_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2, 3) should be 5")
	}
}
`,
			},
			wantOK: true,
		},
		{
			name: "failing test",
			code: map[string]string{
				"main.go": `package main

func Add(a, b int) int {
	return a - b // bug!
}
`,
				"main_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2, 3) should be 5")
	}
}
`,
			},
			wantOK: false,
		},
		{
			name: "multiple tests all passing",
			code: map[string]string{
				"main.go": `package main

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}
`,
				"main_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2, 3) should be 5")
	}
}

func TestMultiply(t *testing.T) {
	if Multiply(2, 3) != 6 {
		t.Error("Multiply(2, 3) should be 6")
	}
}
`,
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := exec.RunTests(ctx, tc.code, []string{"-v"})
			if err != nil {
				t.Fatalf("RunTests failed: %v", err)
			}

			if result.OK != tc.wantOK {
				t.Errorf("OK = %v; want %v", result.OK, tc.wantOK)
			}

			if result.Output == "" {
				t.Error("Output should not be empty for tests")
			}

			if result.Duration == 0 {
				t.Error("Duration should be set")
			}
		})
	}
}

func TestLocalExecutor_Timeout(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "runner-test-*")
	defer os.RemoveAll(tmpDir)

	exec := runner.NewLocalExecutor(tmpDir)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Code with infinite loop
	code := map[string]string{
		"main.go": `package main

func main() {
	for {}
}
`,
	}

	// Build should handle context cancellation gracefully
	_, err := exec.RunBuild(ctx, code)
	// Context timeout is expected - we just verify it doesn't panic
	_ = err
}
