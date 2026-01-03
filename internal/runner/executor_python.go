package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PythonExecutor handles Python code execution
type PythonExecutor struct {
	config LanguageConfig
}

// NewPythonExecutor creates a new Python executor
func NewPythonExecutor() *PythonExecutor {
	configs := DefaultLanguageConfigs()
	return &PythonExecutor{
		config: configs[LanguagePython],
	}
}

// Language returns the language this executor handles
func (e *PythonExecutor) Language() Language {
	return LanguagePython
}

// Format checks Python code formatting using ruff
func (e *PythonExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Collect Python files
	var pyFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".py") {
			pyFiles = append(pyFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(pyFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	// Try ruff first, fall back to black
	args := append([]string{"-m", "ruff", "format", "--check", "--diff"}, pyFiles...)
	cmd := exec.CommandContext(ctx, "python3", args...)
	output, err := cmd.CombinedOutput()

	// If ruff not available, try black
	if err != nil && strings.Contains(string(output), "No module named") {
		args = append([]string{"-m", "black", "--check", "--diff"}, pyFiles...)
		cmd = exec.CommandContext(ctx, "python3", args...)
		output, _ = cmd.CombinedOutput()
	}

	diff := string(output)
	return &FormatResult{
		OK:   cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Diff: diff,
	}, nil
}

// FormatFix formats Python code and returns the formatted version
func (e *PythonExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
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
		if !strings.HasSuffix(filename, ".py") {
			continue
		}
		filePath := filepath.Join(tmpDir, filename)

		// Try ruff first
		cmd := exec.CommandContext(ctx, "python3", "-m", "ruff", "format", filePath)
		if err := cmd.Run(); err != nil {
			// Fall back to black
			cmd = exec.CommandContext(ctx, "python3", "-m", "black", filePath)
			cmd.Run()
		}

		// Read formatted file
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		result[filename] = string(content)
	}

	return result, nil
}

// Build checks Python syntax using py_compile
func (e *PythonExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	var allErrors strings.Builder
	allOK := true

	for filename := range code {
		if !strings.HasSuffix(filename, ".py") {
			continue
		}
		// Skip test files for syntax check - they'll be checked during test run
		if strings.HasPrefix(filename, "test_") || strings.HasSuffix(filename, "_test.py") {
			continue
		}

		filePath := filepath.Join(tmpDir, filename)
		cmd := exec.CommandContext(ctx, "python3", "-m", "py_compile", filePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			allOK = false
			allErrors.WriteString(string(output))
		}
	}

	return &BuildResult{
		OK:     allOK,
		Output: allErrors.String(),
	}, nil
}

// Test runs Python tests using pytest
func (e *PythonExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Create requirements.txt if not present
	if _, ok := code["requirements.txt"]; !ok {
		reqContent := e.config.InitFiles["requirements.txt"]
		os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(reqContent), 0644)
	}

	// Run pytest with JSON output
	start := time.Now()
	args := []string{"-m", "pytest", "-v", "--tb=short"}

	// Add JSON report if pytest-json-report is available
	jsonFile := filepath.Join(tmpDir, ".pytest_results.json")
	args = append(args, "--json-report", "--json-report-file="+jsonFile)

	args = append(args, flags...)
	args = append(args, tmpDir)

	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	duration := time.Since(start)

	// Try to parse JSON results
	testOutput := string(output)
	if jsonData, err := os.ReadFile(jsonFile); err == nil {
		// Convert pytest JSON to a more readable format
		testOutput = testOutput + "\n--- JSON Results ---\n" + string(jsonData)
	}

	return &TestResult{
		OK:       cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Output:   testOutput,
		Duration: duration,
	}, nil
}

// PytestResult represents pytest JSON output structure
type PytestResult struct {
	Created   float64 `json:"created"`
	Duration  float64 `json:"duration"`
	ExitCode  int     `json:"exitcode"`
	Collectors []struct {
		NodeID   string `json:"nodeid"`
		Outcome  string `json:"outcome"`
	} `json:"collectors"`
	Tests []struct {
		NodeID   string  `json:"nodeid"`
		Outcome  string  `json:"outcome"`
		Duration float64 `json:"duration"`
		Call     struct {
			Duration float64 `json:"duration"`
		} `json:"call"`
	} `json:"tests"`
	Summary struct {
		Passed  int `json:"passed"`
		Failed  int `json:"failed"`
		Total   int `json:"total"`
	} `json:"summary"`
}

// ParsePytestOutput parses pytest JSON output
func ParsePytestOutput(jsonData []byte) (*PytestResult, error) {
	var result PytestResult
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Ensure PythonExecutor implements LanguageExecutor
var _ LanguageExecutor = (*PythonExecutor)(nil)
