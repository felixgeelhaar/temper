package risk

import (
	"regexp"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Pattern represents a risk detection pattern
type Pattern struct {
	ID          string
	Category    domain.RiskCategory
	Severity    domain.RiskSeverity
	Title       string
	Description string
	Suggestion  string
	Regex       *regexp.Regexp
	Languages   []string // empty means all languages
}

// Detector analyzes code for risky patterns
type Detector struct {
	patterns []Pattern
}

// NewDetector creates a new risk detector with default patterns
func NewDetector() *Detector {
	return &Detector{
		patterns: defaultPatterns(),
	}
}

// Analyze scans code files for risky patterns
func (d *Detector) Analyze(code map[string]string) []domain.RiskNotice {
	var risks []domain.RiskNotice

	for filename, content := range code {
		lang := detectLanguage(filename)
		fileRisks := d.analyzeFile(filename, content, lang)
		risks = append(risks, fileRisks...)
	}

	return risks
}

func (d *Detector) analyzeFile(filename, content, lang string) []domain.RiskNotice {
	var risks []domain.RiskNotice
	lines := strings.Split(content, "\n")

	for _, pattern := range d.patterns {
		// Skip if pattern doesn't apply to this language
		if len(pattern.Languages) > 0 && !contains(pattern.Languages, lang) {
			continue
		}

		// Check each line
		for lineNum, line := range lines {
			if pattern.Regex.MatchString(line) {
				risks = append(risks, domain.RiskNotice{
					ID:          pattern.ID,
					Category:    pattern.Category,
					Severity:    pattern.Severity,
					Title:       pattern.Title,
					Description: pattern.Description,
					File:        filename,
					Line:        lineNum + 1,
					Suggestion:  pattern.Suggestion,
				})
			}
		}
	}

	return risks
}

func detectLanguage(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".go"):
		return "go"
	case strings.HasSuffix(filename, ".py"):
		return "python"
	case strings.HasSuffix(filename, ".ts"), strings.HasSuffix(filename, ".tsx"):
		return "typescript"
	case strings.HasSuffix(filename, ".js"), strings.HasSuffix(filename, ".jsx"):
		return "javascript"
	default:
		return ""
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func defaultPatterns() []Pattern {
	return []Pattern{
		// Security - High severity
		{
			ID:          "SEC001",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityHigh,
			Title:       "Hardcoded secret detected",
			Description: "Passwords, API keys, or secrets should not be hardcoded in source code.",
			Suggestion:  "Use environment variables or a secrets manager instead.",
			Regex:       regexp.MustCompile(`(?i)(password|secret|api_?key|token)\s*[:=]\s*["'][\w\-]+["']`),
		},
		{
			ID:          "SEC002",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityHigh,
			Title:       "SQL injection risk",
			Description: "String concatenation in SQL queries can lead to SQL injection attacks.",
			Suggestion:  "Use parameterized queries or prepared statements.",
			Regex:       regexp.MustCompile(`(?i)(query|exec|execute)\s*\([^)]*\+\s*[^)]+\)`),
		},
		{
			ID:          "SEC003",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityHigh,
			Title:       "Command injection risk",
			Description: "Executing shell commands with user input can lead to command injection.",
			Suggestion:  "Avoid shell execution with user input, or use proper escaping.",
			Regex:       regexp.MustCompile(`(?i)(exec\.Command|os\.system|subprocess\.(call|run|Popen))\s*\([^)]*\+`),
			Languages:   []string{"go", "python"},
		},
		{
			ID:          "SEC004",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Insecure random for crypto",
			Description: "math/rand is not cryptographically secure.",
			Suggestion:  "Use crypto/rand for security-sensitive random values.",
			Regex:       regexp.MustCompile(`math/rand|rand\.Seed|rand\.Int|rand\.Intn`),
			Languages:   []string{"go"},
		},
		{
			ID:          "SEC005",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Potential path traversal",
			Description: "File paths with user input may allow directory traversal attacks.",
			Suggestion:  "Validate and sanitize file paths, use filepath.Clean().",
			Regex:       regexp.MustCompile(`(os\.Open|ioutil\.ReadFile|os\.ReadFile)\s*\([^)]*\+`),
			Languages:   []string{"go"},
		},

		// Go Quality - Medium severity
		{
			ID:          "GO001",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Unchecked error",
			Description: "Error return value is being ignored.",
			Suggestion:  "Always check error returns: if err != nil { return err }",
			Regex:       regexp.MustCompile(`\s+_\s*=\s*\w+\([^)]*\)`),
			Languages:   []string{"go"},
		},
		{
			ID:          "GO002",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityLow,
			Title:       "TODO/FIXME comment",
			Description: "Incomplete implementation marked with TODO or FIXME.",
			Suggestion:  "Address or remove TODO comments before completion.",
			Regex:       regexp.MustCompile(`(?i)//\s*(TODO|FIXME|HACK|XXX)`),
			Languages:   []string{"go"},
		},
		{
			ID:          "GO003",
			Category:    domain.RiskCategoryReliability,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Deferred close without error check",
			Description: "defer file.Close() doesn't handle potential close errors.",
			Suggestion:  "Consider checking close errors for important resources.",
			Regex:       regexp.MustCompile(`defer\s+\w+\.Close\(\)`),
			Languages:   []string{"go"},
		},
		{
			ID:          "GO004",
			Category:    domain.RiskCategoryPerformance,
			Severity:    domain.RiskSeverityLow,
			Title:       "String concatenation in loop",
			Description: "Repeated string concatenation is inefficient.",
			Suggestion:  "Use strings.Builder for building strings in loops.",
			Regex:       regexp.MustCompile(`\+=\s*"[^"]*"`),
			Languages:   []string{"go"},
		},

		// Python patterns
		{
			ID:          "PY001",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Bare except clause",
			Description: "Catching all exceptions hides bugs and makes debugging difficult.",
			Suggestion:  "Catch specific exceptions instead of using bare 'except:'.",
			Regex:       regexp.MustCompile(`except\s*:`),
			Languages:   []string{"python"},
		},
		{
			ID:          "PY002",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityHigh,
			Title:       "Eval with dynamic input",
			Description: "eval() with user input is dangerous and allows code injection.",
			Suggestion:  "Avoid eval(). Use ast.literal_eval() for safe evaluation.",
			Regex:       regexp.MustCompile(`eval\s*\(`),
			Languages:   []string{"python"},
		},
		{
			ID:          "PY003",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityLow,
			Title:       "Print statement for debugging",
			Description: "Print statements may be leftover debug code.",
			Suggestion:  "Use logging module instead of print for production code.",
			Regex:       regexp.MustCompile(`^\s*print\s*\(`),
			Languages:   []string{"python"},
		},

		// TypeScript/JavaScript patterns
		{
			ID:          "TS001",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityHigh,
			Title:       "innerHTML assignment",
			Description: "Direct innerHTML assignment can lead to XSS vulnerabilities.",
			Suggestion:  "Use textContent or sanitize HTML before insertion.",
			Regex:       regexp.MustCompile(`\.innerHTML\s*=`),
			Languages:   []string{"typescript", "javascript"},
		},
		{
			ID:          "TS002",
			Category:    domain.RiskCategorySecurity,
			Severity:    domain.RiskSeverityHigh,
			Title:       "Eval usage",
			Description: "eval() executes arbitrary code and is a security risk.",
			Suggestion:  "Avoid eval(). Use safer alternatives like JSON.parse().",
			Regex:       regexp.MustCompile(`\beval\s*\(`),
			Languages:   []string{"typescript", "javascript"},
		},
		{
			ID:          "TS003",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Console.log in code",
			Description: "Console statements may be leftover debug code.",
			Suggestion:  "Remove console.log or use a proper logging library.",
			Regex:       regexp.MustCompile(`console\.(log|debug|info)\s*\(`),
			Languages:   []string{"typescript", "javascript"},
		},
		{
			ID:          "TS004",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityMedium,
			Title:       "Any type usage",
			Description: "Using 'any' type defeats TypeScript's type safety.",
			Suggestion:  "Use specific types or 'unknown' for truly dynamic values.",
			Regex:       regexp.MustCompile(`:\s*any\b`),
			Languages:   []string{"typescript"},
		},

		// General patterns
		{
			ID:          "GEN001",
			Category:    domain.RiskCategoryQuality,
			Severity:    domain.RiskSeverityLow,
			Title:       "Commented-out code",
			Description: "Commented-out code creates noise and confusion.",
			Suggestion:  "Remove commented-out code; use version control for history.",
			Regex:       regexp.MustCompile(`^\s*(//|#)\s*(if|for|func|def|class|const|let|var)\s+\w+`),
		},
	}
}
