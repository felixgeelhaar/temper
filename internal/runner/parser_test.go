package runner

import (
	"testing"
	"time"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser() returned nil")
	}
	if p.buildErrorRegex == nil {
		t.Error("buildErrorRegex should not be nil")
	}
}

func TestParser_ParseBuildErrors(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		output   string
		expected int
		wantFile string
		wantLine int
		wantMsg  string
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single error",
			output:   "main.go:10:5: undefined: fmt",
			expected: 1,
			wantFile: "main.go",
			wantLine: 10,
			wantMsg:  "undefined: fmt",
		},
		{
			name: "multiple errors",
			output: `main.go:10:5: undefined: fmt
main.go:15:1: syntax error: unexpected EOF
utils.go:20:10: cannot convert x to int`,
			expected: 3,
		},
		{
			name:     "non-error line",
			output:   "# some comment",
			expected: 0,
		},
		{
			name: "mixed output",
			output: `# building
main.go:10:5: undefined: fmt
ok      module/package`,
			expected: 1,
		},
		{
			name:     "error with path",
			output:   `pkg/handler/user.go:42:15: cannot use x (type int) as type string`,
			expected: 1,
			wantFile: "pkg/handler/user.go",
			wantLine: 42,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p.ParseBuildErrors(tc.output)
			if len(diagnostics) != tc.expected {
				t.Errorf("ParseBuildErrors() returned %d diagnostics, want %d", len(diagnostics), tc.expected)
				return
			}

			if tc.expected > 0 && tc.wantFile != "" {
				if diagnostics[0].File != tc.wantFile {
					t.Errorf("File = %s, want %s", diagnostics[0].File, tc.wantFile)
				}
			}
			if tc.expected > 0 && tc.wantLine > 0 {
				if diagnostics[0].Line != tc.wantLine {
					t.Errorf("Line = %d, want %d", diagnostics[0].Line, tc.wantLine)
				}
			}
			if tc.expected > 0 && tc.wantMsg != "" {
				if diagnostics[0].Message != tc.wantMsg {
					t.Errorf("Message = %s, want %s", diagnostics[0].Message, tc.wantMsg)
				}
			}
		})
	}
}

func TestParser_ParseBuildErrors_Severity(t *testing.T) {
	p := NewParser()

	output := "main.go:10:5: undefined: fmt"
	diagnostics := p.ParseBuildErrors(output)

	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Severity != "error" {
		t.Errorf("Severity = %s, want error", diagnostics[0].Severity)
	}
}

func TestParser_ParseTestOutput(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name         string
		output       string
		expected     int
		wantPassed   []bool
		wantNames    []string
		wantPackages []string
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name: "single passing test",
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:01Z","Action":"output","Package":"example","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}
{"Time":"2024-01-01T10:00:01Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}`,
			expected:     1,
			wantPassed:   []bool{true},
			wantNames:    []string{"TestAdd"},
			wantPackages: []string{"example"},
		},
		{
			name: "single failing test",
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:01Z","Action":"output","Package":"example","Test":"TestAdd","Output":"--- FAIL: TestAdd\n"}
{"Time":"2024-01-01T10:00:01Z","Action":"fail","Package":"example","Test":"TestAdd","Elapsed":0.002}`,
			expected:   1,
			wantPassed: []bool{false},
			wantNames:  []string{"TestAdd"},
		},
		{
			name: "multiple tests mixed results",
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}
{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestSubtract"}
{"Time":"2024-01-01T10:00:00Z","Action":"fail","Package":"example","Test":"TestSubtract","Elapsed":0.001}
{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestMultiply"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestMultiply","Elapsed":0.001}`,
			expected:   3,
			wantPassed: []bool{true, false, true},
			wantNames:  []string{"TestAdd", "TestSubtract", "TestMultiply"},
		},
		{
			name: "skipped test",
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestSkipped"}
{"Time":"2024-01-01T10:00:00Z","Action":"skip","Package":"example","Test":"TestSkipped","Elapsed":0.0}`,
			expected:   1,
			wantPassed: []bool{true}, // skipped counts as passed
		},
		{
			name: "package level events ignored",
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example"}`,
			expected: 0,
		},
		{
			name: "test with output",
			output: `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:00Z","Action":"output","Package":"example","Test":"TestAdd","Output":"line 1\n"}
{"Time":"2024-01-01T10:00:00Z","Action":"output","Package":"example","Test":"TestAdd","Output":"line 2\n"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}`,
			expected: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := p.ParseTestOutput(tc.output)

			if len(results) != tc.expected {
				t.Errorf("ParseTestOutput() returned %d results, want %d", len(results), tc.expected)
				return
			}

			for i, want := range tc.wantPassed {
				if i >= len(results) {
					break
				}
				if results[i].Passed != want {
					t.Errorf("results[%d].Passed = %v, want %v", i, results[i].Passed, want)
				}
			}

			for i, want := range tc.wantNames {
				if i >= len(results) {
					break
				}
				if results[i].Name != want {
					t.Errorf("results[%d].Name = %s, want %s", i, results[i].Name, want)
				}
			}

			for i, want := range tc.wantPackages {
				if i >= len(results) {
					break
				}
				if results[i].Package != want {
					t.Errorf("results[%d].Package = %s, want %s", i, results[i].Package, want)
				}
			}
		})
	}
}

func TestParser_ParseTestOutput_Duration(t *testing.T) {
	p := NewParser()

	output := `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":1.5}`

	results := p.ParseTestOutput(output)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	expectedDuration := time.Duration(1.5 * float64(time.Second))
	if results[0].Duration != expectedDuration {
		t.Errorf("Duration = %v, want %v", results[0].Duration, expectedDuration)
	}
}

func TestParser_ParseTestOutput_OutputAccumulation(t *testing.T) {
	p := NewParser()

	output := `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
{"Time":"2024-01-01T10:00:00Z","Action":"output","Package":"example","Test":"TestAdd","Output":"output1\n"}
{"Time":"2024-01-01T10:00:00Z","Action":"output","Package":"example","Test":"TestAdd","Output":"output2\n"}
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}`

	results := p.ParseTestOutput(output)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Output != "output1\noutput2\n" {
		t.Errorf("Output = %q, want %q", results[0].Output, "output1\noutput2\n")
	}
}

func TestParser_ParseTestOutput_InvalidJSON(t *testing.T) {
	p := NewParser()

	output := `{"Time":"2024-01-01T10:00:00Z","Action":"run","Package":"example","Test":"TestAdd"}
not valid json
{"Time":"2024-01-01T10:00:00Z","Action":"pass","Package":"example","Test":"TestAdd","Elapsed":0.001}`

	// Should handle invalid JSON gracefully
	results := p.ParseTestOutput(output)
	if len(results) != 1 {
		t.Errorf("expected 1 result (invalid json skipped), got %d", len(results))
	}
}

func TestParser_ParseFormatDiff(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name           string
		output         string
		wantHasChanges bool
		wantDiff       string
	}{
		{
			name:           "empty output",
			output:         "",
			wantHasChanges: false,
			wantDiff:       "",
		},
		{
			name:           "whitespace only",
			output:         "   \n\t  ",
			wantHasChanges: false,
			wantDiff:       "",
		},
		{
			name: "has diff",
			output: `diff -u main.go.orig main.go
--- main.go.orig
+++ main.go
@@ -1,3 +1,3 @@
 package main
-func main()   { }
+func main() {}`,
			wantHasChanges: true,
			wantDiff: `diff -u main.go.orig main.go
--- main.go.orig
+++ main.go
@@ -1,3 +1,3 @@
 package main
-func main()   { }
+func main() {}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hasChanges, diff := p.ParseFormatDiff(tc.output)

			if hasChanges != tc.wantHasChanges {
				t.Errorf("hasChanges = %v, want %v", hasChanges, tc.wantHasChanges)
			}
			if diff != tc.wantDiff {
				t.Errorf("diff = %q, want %q", diff, tc.wantDiff)
			}
		})
	}
}

func TestParser_ParseBuildErrors_Column(t *testing.T) {
	p := NewParser()

	output := "main.go:10:5: undefined: fmt"
	diagnostics := p.ParseBuildErrors(output)

	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}

	if diagnostics[0].Column != 5 {
		t.Errorf("Column = %d, want 5", diagnostics[0].Column)
	}
}

func TestParser_ParseBuildErrors_MultilineMessage(t *testing.T) {
	p := NewParser()

	// Each line should be parsed separately
	output := `main.go:10:5: undefined: fmt
	have string
	want int`

	diagnostics := p.ParseBuildErrors(output)

	// Only the first line matches the pattern
	if len(diagnostics) != 1 {
		t.Errorf("expected 1 diagnostic, got %d", len(diagnostics))
	}
}
