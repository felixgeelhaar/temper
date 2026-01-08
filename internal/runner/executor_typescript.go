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

// TypeScriptExecutor handles TypeScript code execution
type TypeScriptExecutor struct {
	config LanguageConfig
}

// NewTypeScriptExecutor creates a new TypeScript executor
func NewTypeScriptExecutor() *TypeScriptExecutor {
	configs := DefaultLanguageConfigs()
	return &TypeScriptExecutor{
		config: configs[LanguageTypeScript],
	}
}

// Language returns the language this executor handles
func (e *TypeScriptExecutor) Language() Language {
	return LanguageTypeScript
}

// Format checks TypeScript code formatting using prettier
func (e *TypeScriptExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Collect TypeScript files
	var tsFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".ts") || strings.HasSuffix(filename, ".tsx") {
			tsFiles = append(tsFiles, filename)
		}
	}

	if len(tsFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	// Run prettier check
	args := append([]string{"prettier", "--check"}, tsFiles...)
	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()

	return &FormatResult{
		OK:   cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Diff: string(output),
	}, nil
}

// FormatFix formats TypeScript code and returns the formatted version
func (e *TypeScriptExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for filename, content := range code {
		result[filename] = content
	}

	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	for filename := range code {
		if !strings.HasSuffix(filename, ".ts") && !strings.HasSuffix(filename, ".tsx") {
			continue
		}

		// Run prettier on the file
		cmd := exec.CommandContext(ctx, "npx", "prettier", "--write", filename)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			continue
		}

		// Read formatted file
		content, err := os.ReadFile(filepath.Join(tmpDir, filename))
		if err != nil {
			continue
		}
		result[filename] = string(content)
	}

	return result, nil
}

// Build type-checks TypeScript code using tsc
func (e *TypeScriptExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Run TypeScript compiler in check mode
	cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	return &BuildResult{
		OK:     err == nil,
		Output: string(output),
	}, nil
}

// Test runs TypeScript tests using vitest
func (e *TypeScriptExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	start := time.Now()

	// Run vitest with JSON reporter
	args := []string{"vitest", "run", "--reporter=json"}
	args = append(args, flags...)

	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	duration := time.Since(start)

	return &TestResult{
		OK:       cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Output:   string(output),
		Duration: duration,
	}, nil
}

// setupProject creates a temporary project with necessary config files
func (e *TypeScriptExecutor) setupProject(code map[string]string) (string, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return "", err
	}

	// Add package.json if not present
	if _, ok := code["package.json"]; !ok {
		content := e.config.InitFiles["package.json"]
		os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(content), 0644)
	}

	// Add tsconfig.json if not present
	if _, ok := code["tsconfig.json"]; !ok {
		content := e.config.InitFiles["tsconfig.json"]
		os.WriteFile(filepath.Join(tmpDir, "tsconfig.json"), []byte(content), 0644)
	}

	// Add vitest config if test files present
	hasTests := false
	for filename := range code {
		if strings.Contains(filename, ".test.") || strings.Contains(filename, ".spec.") {
			hasTests = true
			break
		}
	}
	if hasTests {
		vitestConfig := `import { defineConfig } from 'vitest/config'
export default defineConfig({
  test: {
    globals: true,
  },
})`
		os.WriteFile(filepath.Join(tmpDir, "vitest.config.ts"), []byte(vitestConfig), 0644)
	}

	// Install dependencies (minimal install)
	cmd := exec.CommandContext(context.Background(), "npm", "install", "--prefer-offline", "--no-audit", "--no-fund")
	cmd.Dir = tmpDir
	cmd.Run() // Ignore errors - we'll fail later if needed

	return tmpDir, nil
}

// VitestResult represents vitest JSON output structure
type VitestResult struct {
	NumTotalTestSuites  int   `json:"numTotalTestSuites"`
	NumPassedTestSuites int   `json:"numPassedTestSuites"`
	NumFailedTestSuites int   `json:"numFailedTestSuites"`
	NumTotalTests       int   `json:"numTotalTests"`
	NumPassedTests      int   `json:"numPassedTests"`
	NumFailedTests      int   `json:"numFailedTests"`
	StartTime           int64 `json:"startTime"`
	Success             bool  `json:"success"`
	TestResults         []struct {
		Name     string `json:"name"`
		Status   string `json:"status"`
		Duration int    `json:"duration"`
		Message  string `json:"message,omitempty"`
	} `json:"testResults"`
}

// ParseVitestOutput parses vitest JSON output
func ParseVitestOutput(jsonData []byte) (*VitestResult, error) {
	var result VitestResult
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Ensure TypeScriptExecutor implements LanguageExecutor
var _ LanguageExecutor = (*TypeScriptExecutor)(nil)
