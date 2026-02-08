package runner

import (
	"context"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// JavaExecutor handles Java code execution
type JavaExecutor struct {
	config LanguageConfig
}

// NewJavaExecutor creates a new Java executor
func NewJavaExecutor() *JavaExecutor {
	configs := DefaultLanguageConfigs()
	return &JavaExecutor{
		config: configs[LanguageJava],
	}
}

// Language returns the language this executor handles
func (e *JavaExecutor) Language() Language {
	return LanguageJava
}

// Format checks Java code formatting using google-java-format
func (e *JavaExecutor) Format(ctx context.Context, code map[string]string) (*FormatResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	var javaFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".java") {
			javaFiles = append(javaFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(javaFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	// Try google-java-format first, fall back to simple compilation check
	args := append([]string{"--dry-run", "--set-exit-if-changed"}, javaFiles...)
	cmd := exec.CommandContext(ctx, "google-java-format", args...)
	output, _ := cmd.CombinedOutput()

	return &FormatResult{
		OK:   cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
		Diff: string(output),
	}, nil
}

// FormatFix formats Java code and returns the formatted version
func (e *JavaExecutor) FormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
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
		if !strings.HasSuffix(filename, ".java") {
			continue
		}

		filePath := filepath.Join(tmpDir, filename)
		cmd := exec.CommandContext(ctx, "google-java-format", "--replace", filePath)
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

// Build compiles Java code using javac
func (e *JavaExecutor) Build(ctx context.Context, code map[string]string) (*BuildResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Create output directory
	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	// Collect all Java files
	var javaFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".java") {
			javaFiles = append(javaFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(javaFiles) == 0 {
		return &BuildResult{OK: true, Output: ""}, nil
	}

	// Compile all Java files together
	args := append([]string{"-d", outDir}, javaFiles...)
	cmd := exec.CommandContext(ctx, "javac", args...)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	return &BuildResult{
		OK:     err == nil,
		Output: string(output),
	}, nil
}

// Test runs Java tests using JUnit
func (e *JavaExecutor) Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Create output directory
	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	// Collect all Java files
	var javaFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".java") {
			javaFiles = append(javaFiles, filepath.Join(tmpDir, filename))
		}
	}

	if len(javaFiles) == 0 {
		return &TestResult{OK: true, Output: "no test files found", Duration: 0}, nil
	}

	start := time.Now()

	// Check for JUnit standalone jar in common locations
	junitJar := findJUnitJar()

	if junitJar != "" {
		// Compile with JUnit on classpath
		compileArgs := []string{"-d", outDir, "-cp", junitJar}
		compileArgs = append(compileArgs, javaFiles...)
		compileCmd := exec.CommandContext(ctx, "javac", compileArgs...)
		compileCmd.Dir = tmpDir
		compileOutput, err := compileCmd.CombinedOutput()
		if err != nil {
			return &TestResult{
				OK:       false,
				Output:   string(compileOutput),
				Duration: time.Since(start),
			}, nil
		}

		// Run JUnit console launcher
		runArgs := []string{"-jar", junitJar,
			"--class-path", outDir,
			"--scan-classpath",
			"--reports-dir", filepath.Join(tmpDir, "reports"),
		}
		runArgs = append(runArgs, flags...)
		cmd := exec.CommandContext(ctx, "java", runArgs...)
		cmd.Dir = tmpDir
		output, _ := cmd.CombinedOutput()
		duration := time.Since(start)

		// Try to parse JUnit XML results
		testOutput := string(output)
		xmlPath := filepath.Join(tmpDir, "reports", "TEST-junit-jupiter.xml")
		if xmlData, err := os.ReadFile(xmlPath); err == nil {
			testOutput = testOutput + "\n--- XML Results ---\n" + string(xmlData)
		}

		return &TestResult{
			OK:       cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0,
			Output:   testOutput,
			Duration: duration,
		}, nil
	}

	// Fallback: compile and run main method as test runner
	compileArgs := []string{"-d", outDir}
	compileArgs = append(compileArgs, javaFiles...)
	compileCmd := exec.CommandContext(ctx, "javac", compileArgs...)
	compileCmd.Dir = tmpDir
	compileOutput, err := compileCmd.CombinedOutput()
	if err != nil {
		return &TestResult{
			OK:       false,
			Output:   string(compileOutput),
			Duration: time.Since(start),
		}, nil
	}

	// Find and run test classes
	var testOutput strings.Builder
	allPassed := true
	for filename := range code {
		if !strings.HasSuffix(filename, "Test.java") && !strings.HasSuffix(filename, "Tests.java") {
			continue
		}
		className := strings.TrimSuffix(filepath.Base(filename), ".java")
		cmd := exec.CommandContext(ctx, "java", "-cp", outDir, className)
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()
		testOutput.Write(output)
		if err != nil {
			allPassed = false
		}
	}
	duration := time.Since(start)

	return &TestResult{
		OK:       allPassed,
		Output:   testOutput.String(),
		Duration: duration,
	}, nil
}

// findJUnitJar looks for JUnit standalone jar in common locations
func findJUnitJar() string {
	paths := []string{
		"/usr/local/lib/junit-platform-console-standalone.jar",
		"/usr/lib/junit-platform-console-standalone.jar",
		"/opt/junit/junit-platform-console-standalone.jar",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// JUnitTestSuite represents JUnit XML test suite output
type JUnitTestSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Time     float64         `xml:"time,attr"`
	Cases    []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single JUnit test case
type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitError   `xml:"error,omitempty"`
}

// JUnitFailure represents a test failure
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// JUnitError represents a test error
type JUnitError struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// ParseJUnitXML parses JUnit XML output
func ParseJUnitXML(xmlData []byte) (*JUnitTestSuite, error) {
	var suite JUnitTestSuite
	if err := xml.Unmarshal(xmlData, &suite); err != nil {
		return nil, err
	}
	return &suite, nil
}

// Ensure JavaExecutor implements LanguageExecutor
var _ LanguageExecutor = (*JavaExecutor)(nil)
