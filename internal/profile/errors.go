package profile

import (
	"regexp"
	"strings"
)

// Common Go error patterns for normalization
var goErrorPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`undefined: \w+`), "undefined: <identifier>"},
	{regexp.MustCompile(`cannot use .+ \(type .+\) as type .+`), "type mismatch"},
	{regexp.MustCompile(`not enough arguments in call to \w+`), "missing arguments"},
	{regexp.MustCompile(`too many arguments in call to \w+`), "too many arguments"},
	{regexp.MustCompile(`missing return`), "missing return"},
	{regexp.MustCompile(`declared but not used`), "unused variable"},
	{regexp.MustCompile(`imported and not used`), "unused import"},
	{regexp.MustCompile(`cannot assign to .+`), "invalid assignment"},
	{regexp.MustCompile(`syntax error: .+`), "syntax error"},
	{regexp.MustCompile(`expected .+, found .+`), "syntax error"},
	{regexp.MustCompile(`redeclared in this block`), "redeclaration"},
}

// Common test failure patterns
var testFailurePatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`got .+, want .+`), "assertion failed: wrong value"},
	{regexp.MustCompile(`expected .+ but got .+`), "assertion failed: wrong value"},
	{regexp.MustCompile(`nil pointer dereference`), "nil pointer"},
	{regexp.MustCompile(`panic: .+`), "panic"},
	{regexp.MustCompile(`timeout`), "timeout"},
	{regexp.MustCompile(`index out of range`), "index out of bounds"},
	{regexp.MustCompile(`deadlock`), "deadlock"},
}

// ExtractErrorPatterns extracts normalized error signatures from build/test output
func ExtractErrorPatterns(buildOutput, testOutput string) []string {
	patterns := make(map[string]bool)

	// Extract from build errors
	for _, line := range strings.Split(buildOutput, "\n") {
		if sig := normalizeError(line, goErrorPatterns); sig != "" {
			patterns[sig] = true
		}
	}

	// Extract from test failures
	for _, line := range strings.Split(testOutput, "\n") {
		if sig := normalizeError(line, testFailurePatterns); sig != "" {
			patterns[sig] = true
		}
		// Also check Go error patterns in test output
		if sig := normalizeError(line, goErrorPatterns); sig != "" {
			patterns[sig] = true
		}
	}

	result := make([]string, 0, len(patterns))
	for pattern := range patterns {
		result = append(result, pattern)
	}
	return result
}

// normalizeError tries to match and normalize an error line
func normalizeError(line string, patterns []struct {
	pattern     *regexp.Regexp
	replacement string
}) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	for _, p := range patterns {
		if p.pattern.MatchString(line) {
			return p.replacement
		}
	}

	return ""
}

// ExtractTestFailures extracts test function names that failed
func ExtractTestFailures(testOutput string) []string {
	var failures []string
	failPattern := regexp.MustCompile(`--- FAIL: (\w+)`)

	for _, line := range strings.Split(testOutput, "\n") {
		if matches := failPattern.FindStringSubmatch(line); len(matches) > 1 {
			failures = append(failures, matches[1])
		}
	}

	return failures
}

// CategorizeError categorizes an error into broad categories
func CategorizeError(errorSig string) string {
	switch {
	case strings.Contains(errorSig, "syntax"):
		return "syntax"
	case strings.Contains(errorSig, "type"):
		return "types"
	case strings.Contains(errorSig, "undefined"), strings.Contains(errorSig, "unused"):
		return "scope"
	case strings.Contains(errorSig, "nil"), strings.Contains(errorSig, "panic"):
		return "runtime"
	case strings.Contains(errorSig, "assertion"):
		return "logic"
	case strings.Contains(errorSig, "timeout"), strings.Contains(errorSig, "deadlock"):
		return "concurrency"
	default:
		return "other"
	}
}
