package profile

import (
	"testing"
)

func TestExtractErrorPatterns(t *testing.T) {
	tests := []struct {
		name        string
		buildOutput string
		testOutput  string
		wantLen     int
		wantContain []string
	}{
		{
			name:        "empty inputs",
			buildOutput: "",
			testOutput:  "",
			wantLen:     0,
		},
		{
			name:        "undefined identifier",
			buildOutput: "main.go:10: undefined: someVar",
			testOutput:  "",
			wantLen:     1,
			wantContain: []string{"undefined: <identifier>"},
		},
		{
			name:        "type mismatch",
			buildOutput: "cannot use x (type int) as type string",
			testOutput:  "",
			wantLen:     1,
			wantContain: []string{"type mismatch"},
		},
		{
			name:        "missing arguments",
			buildOutput: "not enough arguments in call to doSomething",
			testOutput:  "",
			wantLen:     1,
			wantContain: []string{"missing arguments"},
		},
		{
			name:        "unused variable",
			buildOutput: "x declared but not used",
			testOutput:  "",
			wantLen:     1,
			wantContain: []string{"unused variable"},
		},
		{
			name:        "test assertion failure",
			buildOutput: "",
			testOutput:  "got 42, want 100",
			wantLen:     1,
			wantContain: []string{"assertion failed: wrong value"},
		},
		{
			name:        "nil pointer",
			buildOutput: "",
			testOutput:  "nil pointer dereference",
			wantLen:     1,
			wantContain: []string{"nil pointer"},
		},
		{
			name:        "panic",
			buildOutput: "",
			testOutput:  "panic: runtime error",
			wantLen:     1,
			wantContain: []string{"panic"},
		},
		{
			name:        "multiple errors",
			buildOutput: "undefined: x\nsyntax error: missing brace",
			testOutput:  "got 1, want 2",
			wantLen:     3,
			wantContain: []string{"undefined: <identifier>", "syntax error", "assertion failed: wrong value"},
		},
		{
			name:        "deduplicates same error",
			buildOutput: "undefined: x\nundefined: y\nundefined: z",
			testOutput:  "",
			wantLen:     1, // All normalized to same pattern
			wantContain: []string{"undefined: <identifier>"},
		},
		{
			name:        "syntax error",
			buildOutput: "syntax error: unexpected semicolon",
			testOutput:  "",
			wantLen:     1,
			wantContain: []string{"syntax error"},
		},
		{
			name:        "timeout",
			buildOutput: "",
			testOutput:  "test timeout after 30s",
			wantLen:     1,
			wantContain: []string{"timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := ExtractErrorPatterns(tt.buildOutput, tt.testOutput)

			if len(patterns) != tt.wantLen {
				t.Errorf("ExtractErrorPatterns() returned %d patterns, want %d: %v", len(patterns), tt.wantLen, patterns)
			}

			for _, want := range tt.wantContain {
				found := false
				for _, p := range patterns {
					if p == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExtractErrorPatterns() missing pattern %q, got %v", want, patterns)
				}
			}
		})
	}
}

func TestExtractTestFailures(t *testing.T) {
	tests := []struct {
		name       string
		testOutput string
		want       []string
	}{
		{
			name:       "no failures",
			testOutput: "PASS\nok  \tpkg\t0.001s",
			want:       nil,
		},
		{
			name:       "single failure",
			testOutput: "--- FAIL: TestSomething (0.00s)",
			want:       []string{"TestSomething"},
		},
		{
			name: "multiple failures",
			testOutput: `--- FAIL: TestOne (0.00s)
    test.go:10: assertion failed
--- FAIL: TestTwo (0.00s)
    test.go:20: another failure
FAIL`,
			want: []string{"TestOne", "TestTwo"},
		},
		{
			name: "mixed pass and fail",
			testOutput: `=== RUN   TestPass
--- PASS: TestPass (0.00s)
=== RUN   TestFail
--- FAIL: TestFail (0.00s)
FAIL`,
			want: []string{"TestFail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTestFailures(tt.testOutput)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractTestFailures() returned %d failures, want %d", len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("ExtractTestFailures()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		errorSig string
		want     string
	}{
		{"syntax error", "syntax"},
		{"type mismatch", "types"},
		{"undefined: <identifier>", "scope"},
		{"unused variable", "scope"},
		{"nil pointer", "runtime"},
		{"panic", "runtime"},
		{"assertion failed: wrong value", "logic"},
		{"timeout", "concurrency"},
		{"deadlock", "concurrency"},
		{"unknown error", "other"},
		{"", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.errorSig, func(t *testing.T) {
			got := CategorizeError(tt.errorSig)
			if got != tt.want {
				t.Errorf("CategorizeError(%q) = %q, want %q", tt.errorSig, got, tt.want)
			}
		})
	}
}

func TestNormalizeError(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"empty line", "", ""},
		{"whitespace only", "   ", ""},
		{"undefined var", "undefined: myVar", "undefined: <identifier>"},
		{"type error", "cannot use x (type int) as type string", "type mismatch"},
		{"no match", "some random text", ""},
		{"expected found", "expected '}', found 'EOF'", "syntax error"},
		{"redeclared", "x redeclared in this block", "redeclaration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeError(tt.line, goErrorPatterns)
			if got != tt.want {
				t.Errorf("normalizeError(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}
