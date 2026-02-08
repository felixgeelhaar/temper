package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CExecutor handles C code execution
type CExecutor struct {
	config   LanguageConfig
	compiler string // "gcc" or "clang"
}

// NewCExecutor creates a new C executor
func NewCExecutor() *CExecutor {
	configs := DefaultLanguageConfigs()
	compiler := "gcc"
	if _, err := exec.LookPath("gcc"); err != nil {
		compiler = "clang"
	}
	return &CExecutor{
		config:   configs[LanguageC],
		compiler: compiler,
	}
}

// Language returns the language this executor handles
func (e *CExecutor) Language() Language {
	return LanguageC
}

// Format checks C code formatting using clang-format
func (e *CExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	var cFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".c") || strings.HasSuffix(filename, ".h") {
			cFiles = append(cFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(cFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	args := append([]string{"--dry-run", "-Werror"}, cFiles...)
	cmd := exec.CommandContext(ctx, "clang-format", args...)
	output, _ := cmd.CombinedOutput()

	return &FormatResult{
		OK:   cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Diff: string(output),
	}, nil
}

// FormatFix formats C code and returns the formatted version
func (e *CExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
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
		if !strings.HasSuffix(filename, ".c") && !strings.HasSuffix(filename, ".h") {
			continue
		}

		filePath := filepath.Join(tmpDir, filename)
		cmd := exec.CommandContext(ctx, "clang-format", "-i", filePath)
		if err := cmd.Run(); err != nil {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		result[filename] = string(content)
	}

	return result, nil
}

// Build compiles C code using gcc or clang
func (e *CExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Collect source files (not headers)
	var srcFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".c") {
			srcFiles = append(srcFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(srcFiles) == 0 {
		return &BuildResult{OK: true, Output: ""}, nil
	}

	outPath := filepath.Join(tmpDir, "exercise")
	args := []string{"-Wall", "-Wextra", "-o", outPath}
	args = append(args, srcFiles...)

	cmd := exec.CommandContext(ctx, e.compiler, args...)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	return &BuildResult{
		OK:     err == nil,
		Output: string(output),
	}, nil
}

// Test compiles and runs C test code
func (e *CExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	start := time.Now()

	// Collect source files
	var srcFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".c") {
			srcFiles = append(srcFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(srcFiles) == 0 {
		return &TestResult{OK: true, Output: "no source files found", Duration: 0}, nil
	}

	// Compile all sources together
	outPath := filepath.Join(tmpDir, "test_runner")
	compileArgs := []string{"-Wall", "-Wextra", "-o", outPath}
	compileArgs = append(compileArgs, srcFiles...)

	compileCmd := exec.CommandContext(ctx, e.compiler, compileArgs...)
	compileCmd.Dir = tmpDir
	compileOutput, err := compileCmd.CombinedOutput()
	if err != nil {
		return &TestResult{
			OK:       false,
			Output:   string(compileOutput),
			Duration: time.Since(start),
		}, nil
	}

	// Run the compiled test binary
	runCmd := exec.CommandContext(ctx, outPath)
	runCmd.Dir = tmpDir
	output, _ := runCmd.CombinedOutput()
	duration := time.Since(start)

	return &TestResult{
		OK:       runCmd.ProcessState != nil && runCmd.ProcessState.ExitCode() == 0,
		Output:   string(output),
		Duration: duration,
	}, nil
}

// Ensure CExecutor implements LanguageExecutor
var _ LanguageExecutor = (*CExecutor)(nil)

// CPPExecutor handles C++ code execution
type CPPExecutor struct {
	config   LanguageConfig
	compiler string // "g++" or "clang++"
}

// NewCPPExecutor creates a new C++ executor
func NewCPPExecutor() *CPPExecutor {
	configs := DefaultLanguageConfigs()
	compiler := "g++"
	if _, err := exec.LookPath("g++"); err != nil {
		compiler = "clang++"
	}
	return &CPPExecutor{
		config:   configs[LanguageCPP],
		compiler: compiler,
	}
}

// Language returns the language this executor handles
func (e *CPPExecutor) Language() Language {
	return LanguageCPP
}

// Format checks C++ code formatting using clang-format
func (e *CPPExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	var cppFiles []string
	for filename := range code {
		if isCPPFile(filename) {
			cppFiles = append(cppFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(cppFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	args := append([]string{"--dry-run", "-Werror"}, cppFiles...)
	cmd := exec.CommandContext(ctx, "clang-format", args...)
	output, _ := cmd.CombinedOutput()

	return &FormatResult{
		OK:   cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Diff: string(output),
	}, nil
}

// FormatFix formats C++ code and returns the formatted version
func (e *CPPExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
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
		if !isCPPFile(filename) {
			continue
		}

		filePath := filepath.Join(tmpDir, filename)
		cmd := exec.CommandContext(ctx, "clang-format", "-i", filePath)
		if err := cmd.Run(); err != nil {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		result[filename] = string(content)
	}

	return result, nil
}

// Build compiles C++ code using g++ or clang++
func (e *CPPExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	var srcFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".cpp") || strings.HasSuffix(filename, ".cc") {
			srcFiles = append(srcFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(srcFiles) == 0 {
		return &BuildResult{OK: true, Output: ""}, nil
	}

	outPath := filepath.Join(tmpDir, "exercise")
	args := []string{"-std=c++17", "-Wall", "-Wextra", "-o", outPath}
	args = append(args, srcFiles...)

	cmd := exec.CommandContext(ctx, e.compiler, args...)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	return &BuildResult{
		OK:     err == nil,
		Output: string(output),
	}, nil
}

// Test compiles and runs C++ test code
func (e *CPPExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	start := time.Now()

	var srcFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".cpp") || strings.HasSuffix(filename, ".cc") {
			srcFiles = append(srcFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(srcFiles) == 0 {
		return &TestResult{OK: true, Output: "no source files found", Duration: 0}, nil
	}

	// Compile all sources together
	outPath := filepath.Join(tmpDir, "test_runner")
	compileArgs := []string{"-std=c++17", "-Wall", "-Wextra", "-o", outPath}
	compileArgs = append(compileArgs, srcFiles...)

	compileCmd := exec.CommandContext(ctx, e.compiler, compileArgs...)
	compileCmd.Dir = tmpDir
	compileOutput, err := compileCmd.CombinedOutput()
	if err != nil {
		return &TestResult{
			OK:       false,
			Output:   string(compileOutput),
			Duration: time.Since(start),
		}, nil
	}

	// Run the compiled test binary
	runCmd := exec.CommandContext(ctx, outPath)
	runCmd.Dir = tmpDir
	output, _ := runCmd.CombinedOutput()
	duration := time.Since(start)

	return &TestResult{
		OK:       runCmd.ProcessState != nil && runCmd.ProcessState.ExitCode() == 0,
		Output:   string(output),
		Duration: duration,
	}, nil
}

// isCPPFile checks if a filename is a C++ source or header file
func isCPPFile(filename string) bool {
	return strings.HasSuffix(filename, ".cpp") ||
		strings.HasSuffix(filename, ".cc") ||
		strings.HasSuffix(filename, ".hpp") ||
		strings.HasSuffix(filename, ".h")
}

// Ensure CPPExecutor implements LanguageExecutor
var _ LanguageExecutor = (*CPPExecutor)(nil)
