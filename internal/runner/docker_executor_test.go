package runner_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/runner"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping Docker executor tests")
	}
	// Also check if Docker daemon is running by trying to connect
	cmd := exec.Command("docker", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("Docker daemon not running, skipping Docker executor tests: %v, output: %s", err, string(output))
	}
	// Additional check for socket connectivity
	cmd = exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if _, err := cmd.Output(); err != nil {
		t.Skipf("Cannot connect to Docker daemon, skipping: %v", err)
	}
}

func TestDockerExecutor_RunFormat(t *testing.T) {
	skipIfNoDocker(t)

	cfg := runner.DefaultDockerConfig()
	cfg.Timeout = 60 * time.Second // Longer timeout for image pull
	exec, err := runner.NewDockerExecutor(cfg)
	if err != nil {
		t.Fatalf("Failed to create Docker executor: %v", err)
	}
	defer exec.Close()

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

func TestDockerExecutor_RunBuild(t *testing.T) {
	skipIfNoDocker(t)

	cfg := runner.DefaultDockerConfig()
	cfg.Timeout = 60 * time.Second
	exec, err := runner.NewDockerExecutor(cfg)
	if err != nil {
		t.Fatalf("Failed to create Docker executor: %v", err)
	}
	defer exec.Close()

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

func TestDockerExecutor_RunTests(t *testing.T) {
	skipIfNoDocker(t)

	cfg := runner.DefaultDockerConfig()
	cfg.Timeout = 60 * time.Second
	exec, err := runner.NewDockerExecutor(cfg)
	if err != nil {
		t.Fatalf("Failed to create Docker executor: %v", err)
	}
	defer exec.Close()

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

func TestDockerExecutor_ResourceLimits(t *testing.T) {
	skipIfNoDocker(t)

	cfg := runner.DockerConfig{
		BaseImage:  "golang:1.23-alpine",
		MemoryMB:   128, // Limited but reasonable memory
		CPULimit:   0.5,
		NetworkOff: true,
		Timeout:    90 * time.Second, // Go builds need time in containers
	}

	exec, err := runner.NewDockerExecutor(cfg)
	if err != nil {
		t.Fatalf("Failed to create Docker executor: %v", err)
	}
	defer exec.Close()

	ctx := context.Background()

	// Simple code that should still work with limited resources
	code := map[string]string{
		"main.go": `package main

func main() {
	println("hello")
}
`,
	}

	result, err := exec.RunBuild(ctx, code)
	if err != nil {
		t.Fatalf("RunBuild failed: %v", err)
	}

	if !result.OK {
		t.Errorf("Build should succeed with limited resources. Output: %s", result.Output)
	}
}

func TestDockerExecutor_NetworkDisabled(t *testing.T) {
	skipIfNoDocker(t)

	cfg := runner.DefaultDockerConfig()
	cfg.NetworkOff = true
	cfg.Timeout = 90 * time.Second // Go builds need time in containers

	exec, err := runner.NewDockerExecutor(cfg)
	if err != nil {
		t.Fatalf("Failed to create Docker executor: %v", err)
	}
	defer exec.Close()

	ctx := context.Background()

	// Simple code - just verify the container runs with network disabled
	code := map[string]string{
		"main.go": `package main

func main() {
	println("hello from isolated container")
}
`,
	}

	// This should build successfully even with network disabled
	result, err := exec.RunBuild(ctx, code)
	if err != nil {
		t.Fatalf("RunBuild failed: %v", err)
	}

	if !result.OK {
		t.Errorf("Build should succeed. Output: %s", result.Output)
	}
}
