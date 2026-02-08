package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// mockExecutor is a test implementation of Executor
type mockExecutor struct {
	formatResult    *FormatResult
	formatFixResult map[string]string
	buildResult     *BuildResult
	testResult      *TestResult
	formatErr       error
	formatFixErr    error
	buildErr        error
	testErr         error
}

func (m *mockExecutor) RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error) {
	if m.formatErr != nil {
		return nil, m.formatErr
	}
	if m.formatResult != nil {
		return m.formatResult, nil
	}
	return &FormatResult{OK: true, Diff: ""}, nil
}

func (m *mockExecutor) RunFormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	if m.formatFixErr != nil {
		return nil, m.formatFixErr
	}
	if m.formatFixResult != nil {
		return m.formatFixResult, nil
	}
	return code, nil
}

func (m *mockExecutor) RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	if m.buildResult != nil {
		return m.buildResult, nil
	}
	return &BuildResult{OK: true, Output: ""}, nil
}

func (m *mockExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	if m.testErr != nil {
		return nil, m.testErr
	}
	if m.testResult != nil {
		return m.testResult, nil
	}
	return &TestResult{OK: true, Output: "", Duration: time.Millisecond}, nil
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PoolSize != 3 {
		t.Errorf("PoolSize = %d, want 3", cfg.PoolSize)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", cfg.Timeout)
	}
	if cfg.MemoryMB != 256 {
		t.Errorf("MemoryMB = %d, want 256", cfg.MemoryMB)
	}
	if cfg.CPULimit != 0.5 {
		t.Errorf("CPULimit = %v, want 0.5", cfg.CPULimit)
	}
	if cfg.BaseImage != "temper-runner-sandbox:latest" {
		t.Errorf("BaseImage = %s, want temper-runner-sandbox:latest", cfg.BaseImage)
	}
}

func TestNewService(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{}

	svc := NewService(cfg, exec)

	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
	if svc.parser == nil {
		t.Error("parser should not be nil")
	}
	if svc.riskDetector == nil {
		t.Error("riskDetector should not be nil")
	}
}

func TestService_Execute_FormatOnly(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		formatResult: &FormatResult{OK: true, Diff: ""},
	}

	svc := NewService(cfg, exec)

	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		},
		Recipe: domain.CheckRecipe{
			Format: true,
			Build:  false,
			Test:   false,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.FormatOK {
		t.Error("FormatOK should be true")
	}
}

func TestService_Execute_BuildOnly(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		buildResult: &BuildResult{OK: true, Output: ""},
	}

	svc := NewService(cfg, exec)

	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		},
		Recipe: domain.CheckRecipe{
			Format: false,
			Build:  true,
			Test:   false,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.BuildOK {
		t.Error("BuildOK should be true")
	}
}

func TestService_Execute_TestsSkippedOnBuildFailure(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		buildResult: &BuildResult{OK: false, Output: "build error"},
	}

	svc := NewService(cfg, exec)

	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() { undefined }\n",
		},
		Recipe: domain.CheckRecipe{
			Format: false,
			Build:  true,
			Test:   true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.BuildOK {
		t.Error("BuildOK should be false")
	}
	// Tests should not run when build fails
	if output.TestOK {
		t.Error("TestOK should be false when build fails")
	}
}

func TestService_Execute_AllChecks(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		formatResult: &FormatResult{OK: true, Diff: ""},
		buildResult:  &BuildResult{OK: true, Output: ""},
		testResult: &TestResult{
			OK:       true,
			Output:   `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}` + "\n" + `{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}`,
			Duration: time.Millisecond,
		},
	}

	svc := NewService(cfg, exec)

	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go":      "package main\n\nfunc Add(a, b int) int { return a + b }\n",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n",
		},
		Recipe: domain.CheckRecipe{
			Format: true,
			Build:  true,
			Test:   true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.FormatOK {
		t.Error("FormatOK should be true")
	}
	if !output.BuildOK {
		t.Error("BuildOK should be true")
	}
	if !output.TestOK {
		t.Error("TestOK should be true")
	}
	if output.TestsPassed != 1 {
		t.Errorf("TestsPassed = %d, want 1", output.TestsPassed)
	}
}

func TestService_Execute_WithBuildErrors(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		buildResult: &BuildResult{
			OK:     false,
			Output: "main.go:5:10: undefined: fmt",
		},
	}

	svc := NewService(cfg, exec)

	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() { fmt.Println() }\n",
		},
		Recipe: domain.CheckRecipe{
			Build: true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.BuildOK {
		t.Error("BuildOK should be false")
	}
	if len(output.BuildErrors) != 1 {
		t.Errorf("expected 1 build error, got %d", len(output.BuildErrors))
	}
}

func TestService_IsRunning(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{}
	svc := NewService(cfg, exec)

	runID := uuid.New()

	// Initially not running
	if svc.IsRunning(runID) {
		t.Error("should not be running initially")
	}
}

func TestService_FormatCode(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		formatFixResult: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		},
	}
	svc := NewService(cfg, exec)

	code := map[string]string{
		"main.go": "package main\n\nfunc   main(){}\n",
	}

	result, err := svc.FormatCode(context.Background(), code)
	if err != nil {
		t.Fatalf("FormatCode() error = %v", err)
	}

	if _, ok := result["main.go"]; !ok {
		t.Error("result should contain main.go")
	}
}

func TestService_Cancel_NotFound(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{}
	svc := NewService(cfg, exec)

	err := svc.Cancel(uuid.New())
	if err == nil {
		t.Error("Cancel() should return error for non-existent run")
	}
}

func TestDefaultDockerConfig(t *testing.T) {
	cfg := DefaultDockerConfig()

	if cfg.BaseImage != "golang:1.23-alpine" {
		t.Errorf("BaseImage = %s, want golang:1.23-alpine", cfg.BaseImage)
	}
	if cfg.MemoryMB != 256 {
		t.Errorf("MemoryMB = %d, want 256", cfg.MemoryMB)
	}
	if cfg.CPULimit != 0.5 {
		t.Errorf("CPULimit = %v, want 0.5", cfg.CPULimit)
	}
	if !cfg.NetworkOff {
		t.Error("NetworkOff should be true")
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", cfg.Timeout)
	}
}

func TestService_Execute_CustomTimeout(t *testing.T) {
	cfg := Config{
		Timeout: 10 * time.Second,
	}
	exec := &mockExecutor{
		buildResult: &BuildResult{OK: true},
	}

	svc := NewService(cfg, exec)

	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		},
		Recipe: domain.CheckRecipe{
			Build:   true,
			Timeout: 5, // 5 second custom timeout
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.BuildOK {
		t.Error("BuildOK should be true")
	}
}

func TestService_Execute_RiskDetection(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{
		buildResult: &BuildResult{OK: true},
	}

	svc := NewService(cfg, exec)

	// Code with patterns that trigger risk detection:
	// - math/rand (insecure random)
	// - defer file.Close() without error check
	req := ExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Code: map[string]string{
			"main.go": `package main

import (
	"math/rand"
	"os"
)

func main() {
	x := rand.Intn(100)
	println(x)
	file, _ := os.Open("test.txt")
	defer file.Close()
}
`,
		},
		Recipe: domain.CheckRecipe{
			Build: true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Risk detector should find patterns
	if len(output.Risks) == 0 {
		t.Error("expected risk detection for risky code patterns")
	}
}

func TestService_Wait_AlreadyCompleted(t *testing.T) {
	cfg := DefaultConfig()
	exec := &mockExecutor{}
	svc := NewService(cfg, exec)

	// Wait for a run that doesn't exist should return immediately
	err := svc.Wait(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestLanguage_String(t *testing.T) {
	tests := []struct {
		lang Language
		want string
	}{
		{LanguageGo, "go"},
		{LanguagePython, "python"},
		{LanguageTypeScript, "typescript"},
		{LanguageRust, "rust"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.lang.String(); got != tc.want {
				t.Errorf("String() = %v, want %v", got, tc.want)
			}
		})
	}
}

// mockLanguageExecutor is a test implementation of LanguageExecutor
type mockLanguageExecutor struct {
	lang         Language
	formatResult *FormatResult
	formatFixRes map[string]string
	buildResult  *BuildResult
	testResult   *TestResult
	formatErr    error
	formatFixErr error
	buildErr     error
	testErr      error
}

func (m *mockLanguageExecutor) Language() Language {
	return m.lang
}

func (m *mockLanguageExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	if m.formatErr != nil {
		return nil, m.formatErr
	}
	if m.formatResult != nil {
		return m.formatResult, nil
	}
	return &FormatResult{OK: true, Diff: ""}, nil
}

func (m *mockLanguageExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	if m.formatFixErr != nil {
		return nil, m.formatFixErr
	}
	if m.formatFixRes != nil {
		return m.formatFixRes, nil
	}
	return code, nil
}

func (m *mockLanguageExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	if m.buildResult != nil {
		return m.buildResult, nil
	}
	return &BuildResult{OK: true, Output: ""}, nil
}

func (m *mockLanguageExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	if m.testErr != nil {
		return nil, m.testErr
	}
	if m.testResult != nil {
		return m.testResult, nil
	}
	return &TestResult{OK: true, Output: "", Duration: time.Millisecond}, nil
}

func TestNewMultiLanguageService(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	if svc == nil {
		t.Fatal("NewMultiLanguageService() returned nil")
	}
	if svc.registry == nil {
		t.Error("registry should not be nil")
	}
	if svc.parser == nil {
		t.Error("parser should not be nil")
	}
	if svc.riskDetector == nil {
		t.Error("riskDetector should not be nil")
	}

	// Check all language executors are registered
	langs := svc.SupportedLanguages()
	if len(langs) != 7 {
		t.Errorf("expected 7 languages, got %d", len(langs))
	}
}

func TestMultiLanguageService_Execute(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	// Note: This test uses real executors which may fail without Go installed
	// In real tests we would mock the executors
	req := MultiExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Language:   LanguageGo,
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		},
		Recipe: domain.CheckRecipe{
			Format: true,
			Build:  false,
			Test:   false,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.FormatOK {
		t.Error("FormatOK should be true for formatted code")
	}
}

func TestMultiLanguageService_Execute_UnsupportedLanguage(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	// Unregister all executors to test error
	svc.registry = NewExecutorRegistry() // Fresh registry with no executors

	req := MultiExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Language:   LanguageGo,
		Code:       map[string]string{"main.go": "package main"},
		Recipe:     domain.CheckRecipe{Build: true},
	}

	_, err := svc.Execute(context.Background(), req)
	if err == nil {
		t.Error("expected error for unregistered executor")
	}
}

func TestMultiLanguageService_FormatCode(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	code := map[string]string{
		"main.go": "package main\n\nfunc   main(){}",
	}

	result, err := svc.FormatCode(context.Background(), LanguageGo, code)
	if err != nil {
		t.Fatalf("FormatCode() error = %v", err)
	}

	if _, ok := result["main.go"]; !ok {
		t.Error("result should contain main.go")
	}
}

func TestMultiLanguageService_Cancel(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	// Try to cancel a non-existent run
	err := svc.Cancel(uuid.New())
	if err == nil {
		t.Error("Cancel() should return error for non-existent run")
	}
}

func TestMultiLanguageService_IsRunning(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	// Check a run that doesn't exist
	if svc.IsRunning(uuid.New()) {
		t.Error("should not be running")
	}
}

func TestMultiLanguageService_Wait(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	// Wait for a run that doesn't exist should return immediately
	err := svc.Wait(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestMultiLanguageService_SupportedLanguages(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	langs := svc.SupportedLanguages()

	expected := map[Language]bool{
		LanguageGo:         true,
		LanguagePython:     true,
		LanguageTypeScript: true,
		LanguageRust:       true,
		LanguageJava:       true,
		LanguageC:          true,
		LanguageCPP:        true,
	}

	for _, lang := range langs {
		if !expected[lang] {
			t.Errorf("unexpected language: %s", lang)
		}
		delete(expected, lang)
	}

	if len(expected) > 0 {
		t.Errorf("missing languages: %v", expected)
	}
}

func TestMultiLanguageService_Registry(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	registry := svc.Registry()
	if registry == nil {
		t.Error("Registry() returned nil")
	}
}

func TestMultiLanguageService_ParseBuildErrors(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	tests := []struct {
		name   string
		lang   Language
		output string
		want   int
	}{
		{
			name:   "Go errors",
			lang:   LanguageGo,
			output: "main.go:10:5: undefined: fmt",
			want:   1,
		},
		{
			name:   "Python errors",
			lang:   LanguagePython,
			output: "SyntaxError: invalid syntax",
			want:   1,
		},
		{
			name:   "TypeScript errors",
			lang:   LanguageTypeScript,
			output: "error TS2304: Cannot find name 'foo'",
			want:   1,
		},
		{
			name:   "Rust errors",
			lang:   LanguageRust,
			output: "error[E0425]: cannot find value",
			want:   1,
		},
		{
			name:   "Unknown language",
			lang:   Language("unknown"),
			output: "some error",
			want:   0,
		},
		{
			name:   "Empty output",
			lang:   LanguageGo,
			output: "",
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.parseBuildErrors(tc.lang, tc.output)
			if len(got) != tc.want {
				t.Errorf("parseBuildErrors() returned %d errors, want %d", len(got), tc.want)
			}
		})
	}
}

func TestMultiLanguageService_ParseTestOutput(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	tests := []struct {
		name   string
		lang   Language
		output string
		want   int
	}{
		{
			name: "Go test output",
			lang: LanguageGo,
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}`,
			want: 1,
		},
		{
			name:   "Python test output",
			lang:   LanguagePython,
			output: "PASSED test_example.py::test_add",
			want:   0, // simplified parser returns empty
		},
		{
			name:   "TypeScript test output",
			lang:   LanguageTypeScript,
			output: `{"numTotalTests": 5, "numPassedTests": 4}`,
			want:   0, // simplified parser returns empty
		},
		{
			name:   "Rust test output",
			lang:   LanguageRust,
			output: `running 2 tests... test result: ok`,
			want:   0, // simplified parser returns empty
		},
		{
			name:   "Unknown language",
			lang:   Language("unknown"),
			output: "test output",
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.parseTestOutput(tc.lang, tc.output)
			if len(got) != tc.want {
				t.Errorf("parseTestOutput() returned %d results, want %d", len(got), tc.want)
			}
		})
	}
}

func TestMultiLanguageService_Execute_WithCustomTimeout(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	req := MultiExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Language:   LanguageGo,
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		},
		Recipe: domain.CheckRecipe{
			Format:  true,
			Timeout: 5, // 5 second custom timeout
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.FormatOK {
		t.Error("FormatOK should be true")
	}
}

func TestMultiLanguageService_Execute_AllChecks(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	req := MultiExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Language:   LanguageGo,
		Code: map[string]string{
			"main.go":      "package main\n\nfunc main() {}\n\nfunc Add(a, b int) int { return a + b }\n",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n",
		},
		Recipe: domain.CheckRecipe{
			Format: true,
			Build:  true,
			Test:   true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.FormatOK {
		t.Error("FormatOK should be true")
	}
	if !output.BuildOK {
		t.Errorf("BuildOK should be true, output: %s", output.BuildOutput)
	}
}

func TestMultiLanguageService_Execute_BuildFailure(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	req := MultiExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Language:   LanguageGo,
		Code: map[string]string{
			"main.go": "package main\n\nfunc main() { undefined }\n",
		},
		Recipe: domain.CheckRecipe{
			Build: true,
			Test:  true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.BuildOK {
		t.Error("BuildOK should be false for broken code")
	}
	// Tests should not run when build fails
	if output.TestOK {
		t.Error("TestOK should be false when build fails")
	}
}

func TestMultiLanguageService_Execute_WithRisks(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	req := MultiExecuteRequest{
		RunID:      uuid.New(),
		UserID:     uuid.New(),
		ArtifactID: uuid.New(),
		Language:   LanguageGo,
		Code: map[string]string{
			"main.go": `package main

import "math/rand"

func main() {
	x := rand.Intn(100)
	println(x)
}
`,
		},
		Recipe: domain.CheckRecipe{
			Build: true,
		},
	}

	output, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(output.Risks) == 0 {
		t.Error("expected risks for math/rand usage")
	}
}

func TestMultiLanguageService_FormatCode_UnsupportedLanguage(t *testing.T) {
	cfg := DefaultConfig()
	svc := NewMultiLanguageService(cfg)

	// Replace registry with empty one
	svc.registry = NewExecutorRegistry()

	_, err := svc.FormatCode(context.Background(), LanguageGo, map[string]string{})
	if err == nil {
		t.Error("expected error for unregistered executor")
	}
}

func TestGoExecutor_Format(t *testing.T) {
	exec := NewGoExecutor(false)
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
		{
			name: "non-go file only",
			code: map[string]string{
				"README.md": "# Hello",
			},
			wantOK:   true,
			wantDiff: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := exec.Format(ctx, tc.code)
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}

			if result.OK != tc.wantOK {
				t.Errorf("OK = %v; want %v", result.OK, tc.wantOK)
			}

			hasDiff := result.Diff != ""
			if hasDiff != tc.wantDiff {
				t.Errorf("HasDiff = %v; want %v", hasDiff, tc.wantDiff)
			}
		})
	}
}

func TestGoExecutor_FormatFix(t *testing.T) {
	exec := NewGoExecutor(false)
	ctx := context.Background()

	code := map[string]string{
		"main.go": `package main

func main() {
println("hello")
}
`,
		"README.md": "# Hello",
	}

	result, err := exec.FormatFix(ctx, code)
	if err != nil {
		t.Fatalf("FormatFix failed: %v", err)
	}

	// Check all original files are present
	if _, ok := result["main.go"]; !ok {
		t.Error("result should contain main.go")
	}
	if _, ok := result["README.md"]; !ok {
		t.Error("result should contain README.md")
	}

	// Formatted code should have proper indentation
	if !strings.Contains(result["main.go"], "\tprintln") {
		t.Error("formatted code should have proper indentation")
	}
}

func TestGoExecutor_Build(t *testing.T) {
	exec := NewGoExecutor(false)
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
			name: "with existing go.mod",
			code: map[string]string{
				"go.mod": "module example\n\ngo 1.22\n",
				"main.go": `package main

func main() {
	println("hello")
}
`,
			},
			wantOK:     true,
			wantOutput: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := exec.Build(ctx, tc.code)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
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

func TestGoExecutor_Test(t *testing.T) {
	exec := NewGoExecutor(false)
	ctx := context.Background()

	tests := []struct {
		name   string
		code   map[string]string
		flags  []string
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
			name: "with verbose flag",
			code: map[string]string{
				"main.go": `package main

func Hello() string {
	return "Hello"
}
`,
				"main_test.go": `package main

import "testing"

func TestHello(t *testing.T) {
	if Hello() != "Hello" {
		t.Error("Hello should return Hello")
	}
}
`,
			},
			flags:  []string{"-v"},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := exec.Test(ctx, tc.code, tc.flags)
			if err != nil {
				t.Fatalf("Test failed: %v", err)
			}

			if result.OK != tc.wantOK {
				t.Errorf("OK = %v; want %v", result.OK, tc.wantOK)
			}

			if result.Output == "" {
				t.Error("Output should not be empty")
			}

			if result.Duration == 0 {
				t.Error("Duration should be set")
			}
		})
	}
}

func TestNewLocalExecutor(t *testing.T) {
	exec := NewLocalExecutor("/tmp/test")
	if exec == nil {
		t.Fatal("NewLocalExecutor returned nil")
	}
	if exec.workDir != "/tmp/test" {
		t.Errorf("workDir = %s, want /tmp/test", exec.workDir)
	}
}

func TestParsePytestOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTests int
		wantErr   bool
	}{
		{
			name: "valid pytest output",
			input: `{
				"summary": {"passed": 2, "failed": 1, "total": 3},
				"tests": [
					{"nodeid": "test_example.py::test_add", "outcome": "passed", "call": {"duration": 0.001}},
					{"nodeid": "test_example.py::test_sub", "outcome": "passed", "call": {"duration": 0.002}},
					{"nodeid": "test_example.py::test_mul", "outcome": "failed", "call": {"duration": 0.003}}
				]
			}`,
			wantTests: 3,
			wantErr:   false,
		},
		{
			name:      "empty output",
			input:     `{}`,
			wantTests: 0,
			wantErr:   false,
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParsePytestOutput([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Tests) != tc.wantTests {
				t.Errorf("got %d tests, want %d", len(result.Tests), tc.wantTests)
			}
		})
	}
}

func TestParseVitestOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTests int
		wantErr   bool
	}{
		{
			name: "valid vitest output",
			input: `{
				"numTotalTests": 3,
				"numPassedTests": 2,
				"numFailedTests": 1,
				"testResults": [
					{"name": "test add", "status": "passed", "duration": 10},
					{"name": "test sub", "status": "passed", "duration": 5},
					{"name": "test mul", "status": "failed", "duration": 8, "message": "expected 6 but got 5"}
				]
			}`,
			wantTests: 3,
			wantErr:   false,
		},
		{
			name:      "empty output",
			input:     `{}`,
			wantTests: 0,
			wantErr:   false,
		},
		{
			name:    "invalid json",
			input:   `{broken`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseVitestOutput([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.TestResults) != tc.wantTests {
				t.Errorf("got %d tests, want %d", len(result.TestResults), tc.wantTests)
			}
		})
	}
}

func TestParseCargoTestOutput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantResults int
	}{
		{
			name: "valid cargo test output",
			input: `{"type":"suite","event":"started","test_count":2}
{"type":"test","event":"started","name":"tests::test_add"}
{"type":"test","name":"tests::test_add","event":"ok"}
{"type":"test","event":"started","name":"tests::test_sub"}
{"type":"test","name":"tests::test_sub","event":"failed","message":"assertion failed"}
{"type":"suite","event":"failed","passed":1,"failed":1,"ignored":0}`,
			wantResults: 6,
		},
		{
			name:        "empty output",
			input:       "",
			wantResults: 0,
		},
		{
			name: "mixed with invalid lines",
			input: `{"type":"suite","event":"started"}
not json line
{"type":"test","event":"ok","name":"test1"}`,
			wantResults: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := ParseCargoTestOutput([]byte(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tc.wantResults {
				t.Errorf("got %d results, want %d", len(results), tc.wantResults)
			}
		})
	}
}
