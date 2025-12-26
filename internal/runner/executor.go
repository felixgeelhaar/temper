package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor defines the interface for code execution
type Executor interface {
	// RunFormat runs gofmt and returns diff
	RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error)

	// RunBuild runs go build
	RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error)

	// RunTests runs go test with JSON output
	RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error)
}

// FormatResult contains the result of gofmt
type FormatResult struct {
	OK   bool
	Diff string
}

// BuildResult contains the result of go build
type BuildResult struct {
	OK     bool
	Output string
}

// TestResult contains the result of go test
type TestResult struct {
	OK       bool
	Output   string
	Duration time.Duration
}

// LocalExecutor executes code locally (for development)
type LocalExecutor struct {
	workDir string
}

// NewLocalExecutor creates a new local executor
func NewLocalExecutor(workDir string) *LocalExecutor {
	return &LocalExecutor{workDir: workDir}
}

func (e *LocalExecutor) RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error) {
	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Run gofmt -d on all .go files
	var allDiffs strings.Builder
	for filename := range code {
		if !strings.HasSuffix(filename, ".go") {
			continue
		}
		cmd := exec.CommandContext(ctx, "gofmt", "-d", filepath.Join(tmpDir, filename))
		output, _ := cmd.CombinedOutput()
		if len(output) > 0 {
			allDiffs.Write(output)
		}
	}

	diff := allDiffs.String()
	return &FormatResult{
		OK:   diff == "",
		Diff: diff,
	}, nil
}

func (e *LocalExecutor) RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error) {
	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Initialize go.mod if not present
	if _, ok := code["go.mod"]; !ok {
		modContent := "module exercise\n\ngo 1.22\n"
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)
	}

	// Run go build
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &BuildResult{
			OK:     false,
			Output: string(output),
		}, nil
	}

	return &BuildResult{
		OK:     true,
		Output: string(output),
	}, nil
}

func (e *LocalExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Initialize go.mod if not present
	if _, ok := code["go.mod"]; !ok {
		modContent := "module exercise\n\ngo 1.22\n"
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)
	}

	// Run go test with JSON output
	start := time.Now()
	args := append([]string{"test", "-json", "./..."}, flags...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	duration := time.Since(start)

	return &TestResult{
		OK:       cmd.ProcessState.ExitCode() == 0,
		Output:   string(output),
		Duration: duration,
	}, nil
}

// Helper functions
func createTempCodeDir(code map[string]string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "temper-run-*")
	if err != nil {
		return "", err
	}

	for filename, content := range code {
		filePath := filepath.Join(tmpDir, filename)
		// Create parent directories if needed
		if dir := filepath.Dir(filePath); dir != tmpDir {
			os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			removeTempDir(tmpDir)
			return "", err
		}
	}

	return tmpDir, nil
}

func removeTempDir(dir string) {
	os.RemoveAll(dir)
}

// DockerExecutor executes code in Docker containers
type DockerExecutor struct {
	baseImage  string
	memoryMB   int
	cpuLimit   float64
	networkOff bool
}

// DockerConfig holds Docker executor configuration
type DockerConfig struct {
	BaseImage  string
	MemoryMB   int
	CPULimit   float64
	NetworkOff bool
}

// NewDockerExecutor creates a new Docker executor
func NewDockerExecutor(cfg DockerConfig) *DockerExecutor {
	if cfg.BaseImage == "" {
		cfg.BaseImage = "temper-runner-sandbox:latest"
	}
	if cfg.MemoryMB == 0 {
		cfg.MemoryMB = 256
	}
	if cfg.CPULimit == 0 {
		cfg.CPULimit = 0.5
	}

	return &DockerExecutor{
		baseImage:  cfg.BaseImage,
		memoryMB:   cfg.MemoryMB,
		cpuLimit:   cfg.CPULimit,
		networkOff: cfg.NetworkOff,
	}
}

func (e *DockerExecutor) RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error) {
	// TODO: Implement Docker-based gofmt execution
	// 1. Create temp container with code mounted
	// 2. Run gofmt -d on all .go files
	// 3. Capture diff output
	// 4. Return result

	return &FormatResult{OK: true}, nil
}

func (e *DockerExecutor) RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error) {
	// TODO: Implement Docker-based go build execution
	// 1. Create temp container
	// 2. Write code files to container
	// 3. Run go build
	// 4. Capture output
	// 5. Return result

	return &BuildResult{OK: true}, nil
}

func (e *DockerExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	// TODO: Implement Docker-based go test execution
	// 1. Create temp container
	// 2. Write code files
	// 3. Run go test -json with flags
	// 4. Capture JSON output
	// 5. Return result

	return &TestResult{OK: true, Output: "{}"}, nil
}
