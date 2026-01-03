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

// RustExecutor handles Rust code execution
type RustExecutor struct {
	config LanguageConfig
}

// NewRustExecutor creates a new Rust executor
func NewRustExecutor() *RustExecutor {
	configs := DefaultLanguageConfigs()
	return &RustExecutor{
		config: configs[LanguageRust],
	}
}

// Language returns the language this executor handles
func (e *RustExecutor) Language() Language {
	return LanguageRust
}

// Format checks Rust code formatting using rustfmt
func (e *RustExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Collect Rust files
	var rsFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".rs") {
			rsFiles = append(rsFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(rsFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	// Run rustfmt check
	args := append([]string{"--check"}, rsFiles...)
	cmd := exec.CommandContext(ctx, "rustfmt", args...)
	output, _ := cmd.CombinedOutput()

	return &FormatResult{
		OK:   cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Diff: string(output),
	}, nil
}

// FormatFix formats Rust code and returns the formatted version
func (e *RustExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
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
		if !strings.HasSuffix(filename, ".rs") {
			continue
		}

		filePath := filepath.Join(tmpDir, filename)
		cmd := exec.CommandContext(ctx, "rustfmt", filePath)
		if err := cmd.Run(); err != nil {
			continue
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

// Build compiles Rust code using cargo
func (e *RustExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Run cargo build
	cmd := exec.CommandContext(ctx, "cargo", "build", "--message-format=json")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	return &BuildResult{
		OK:     err == nil,
		Output: string(output),
	}, nil
}

// Test runs Rust tests using cargo test
func (e *RustExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := e.setupProject(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	start := time.Now()

	// Build first to catch compilation errors
	buildCmd := exec.CommandContext(ctx, "cargo", "build")
	buildCmd.Dir = tmpDir
	buildOutput, buildErr := buildCmd.CombinedOutput()

	if buildErr != nil {
		return &TestResult{
			OK:       false,
			Output:   string(buildOutput),
			Duration: time.Since(start),
		}, nil
	}

	// Run cargo test with unstable JSON output
	// Note: JSON output requires nightly or we fall back to regular output
	args := []string{"test", "--no-fail-fast"}
	args = append(args, flags...)

	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	duration := time.Since(start)

	return &TestResult{
		OK:       cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Output:   string(output),
		Duration: duration,
	}, nil
}

// setupProject creates a temporary Cargo project
func (e *RustExecutor) setupProject(code map[string]string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "temper-rust-*")
	if err != nil {
		return "", err
	}

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)

	// Write code files to src directory
	for filename, content := range code {
		if strings.HasSuffix(filename, ".rs") {
			filePath := filepath.Join(srcDir, filename)
			// Create parent directories if needed
			if dir := filepath.Dir(filePath); dir != srcDir {
				os.MkdirAll(dir, 0755)
			}
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				removeTempDir(tmpDir)
				return "", err
			}
		}
	}

	// Ensure lib.rs or main.rs exists
	hasLib := false
	hasMain := false
	for filename := range code {
		if filename == "lib.rs" {
			hasLib = true
		}
		if filename == "main.rs" {
			hasMain = true
		}
	}

	// If neither exists, create a lib.rs that includes all modules
	if !hasLib && !hasMain {
		var modules []string
		for filename := range code {
			if strings.HasSuffix(filename, ".rs") && filename != "lib.rs" && filename != "main.rs" {
				modName := strings.TrimSuffix(filename, ".rs")
				modules = append(modules, "mod "+modName+";")
			}
		}
		libContent := strings.Join(modules, "\n") + "\n"
		os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte(libContent), 0644)
	}

	// Add Cargo.toml if not present
	if _, ok := code["Cargo.toml"]; !ok {
		content := e.config.InitFiles["Cargo.toml"]
		os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(content), 0644)
	} else {
		os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(code["Cargo.toml"]), 0644)
	}

	return tmpDir, nil
}

// CargoTestResult represents cargo test JSON output structure
type CargoTestResult struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Name    string `json:"name"`
	Stdout  string `json:"stdout,omitempty"`
	Message string `json:"message,omitempty"`
}

// ParseCargoTestOutput parses cargo test JSON output
func ParseCargoTestOutput(jsonData []byte) ([]CargoTestResult, error) {
	var results []CargoTestResult
	lines := strings.Split(string(jsonData), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var result CargoTestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue // Skip non-JSON lines
		}
		results = append(results, result)
	}
	return results, nil
}

// Ensure RustExecutor implements LanguageExecutor
var _ LanguageExecutor = (*RustExecutor)(nil)
