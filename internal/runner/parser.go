package runner

import (
	"bufio"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Parser parses go tool output
type Parser struct {
	buildErrorRegex *regexp.Regexp
}

// NewParser creates a new parser
func NewParser() *Parser {
	return &Parser{
		// Matches: file.go:line:col: message
		buildErrorRegex: regexp.MustCompile(`^(.+\.go):(\d+):(\d+):\s*(.+)$`),
	}
}

// ParseBuildErrors parses go build output for errors
func (p *Parser) ParseBuildErrors(output string) []domain.Diagnostic {
	var diagnostics []domain.Diagnostic

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		matches := p.buildErrorRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		lineNum, _ := strconv.Atoi(matches[2])
		colNum, _ := strconv.Atoi(matches[3])

		diagnostics = append(diagnostics, domain.Diagnostic{
			File:     matches[1],
			Line:     lineNum,
			Column:   colNum,
			Severity: "error",
			Message:  matches[4],
		})
	}

	return diagnostics
}

// TestEvent represents a go test JSON event
type TestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

// ParseTestOutput parses go test -json output
func (p *Parser) ParseTestOutput(output string) []domain.TestResult {
	tests := make(map[string]*domain.TestResult)
	var order []string

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		// Skip package-level events
		if event.Test == "" {
			continue
		}

		key := event.Package + "/" + event.Test

		switch event.Action {
		case "run":
			tests[key] = &domain.TestResult{
				Package: event.Package,
				Name:    event.Test,
			}
			order = append(order, key)

		case "output":
			if t, ok := tests[key]; ok {
				t.Output += event.Output
			}

		case "pass":
			if t, ok := tests[key]; ok {
				t.Passed = true
				t.Duration = time.Duration(event.Elapsed * float64(time.Second))
			}

		case "fail":
			if t, ok := tests[key]; ok {
				t.Passed = false
				t.Duration = time.Duration(event.Elapsed * float64(time.Second))
			}

		case "skip":
			// Mark as passed (skipped tests are considered passing)
			if t, ok := tests[key]; ok {
				t.Passed = true
				t.Duration = time.Duration(event.Elapsed * float64(time.Second))
			}
		}
	}

	// Return in order of execution
	results := make([]domain.TestResult, 0, len(order))
	for _, key := range order {
		if t, ok := tests[key]; ok {
			results = append(results, *t)
		}
	}

	return results
}

// ParseFormatDiff parses gofmt -d output
func (p *Parser) ParseFormatDiff(output string) (hasChanges bool, diff string) {
	output = strings.TrimSpace(output)
	if output == "" {
		return false, ""
	}
	return true, output
}
