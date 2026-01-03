package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GoExecutor handles Go code execution
type GoExecutor struct {
	config LanguageConfig
	docker bool
}

// NewGoExecutor creates a new Go executor
func NewGoExecutor(useDocker bool) *GoExecutor {
	configs := DefaultLanguageConfigs()
	return &GoExecutor{
		config: configs[LanguageGo],
		docker: useDocker,
	}
}

// Language returns the language this executor handles
func (e *GoExecutor) Language() Language {
	return LanguageGo
}

// Format checks Go code formatting using gofmt
func (e *GoExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

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

// FormatFix formats Go code and returns the formatted version
func (e *GoExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for filename, content := range code {
		result[filename] = content
	}

	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	for filename := range code {
		if !strings.HasSuffix(filename, ".go") {
			continue
		}
		filePath := filepath.Join(tmpDir, filename)
		cmd := exec.CommandContext(ctx, "gofmt", filePath)
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		result[filename] = string(output)
	}

	return result, nil
}

// Build compiles Go code
func (e *GoExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Add go.mod if not present
	if _, ok := code["go.mod"]; !ok {
		modContent := e.config.InitFiles["go.mod"]
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)
	}

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

// Test runs Go tests with JSON output
func (e *GoExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Add go.mod if not present
	if _, ok := code["go.mod"]; !ok {
		modContent := e.config.InitFiles["go.mod"]
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)
	}

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

// Ensure GoExecutor implements LanguageExecutor
var _ LanguageExecutor = (*GoExecutor)(nil)
