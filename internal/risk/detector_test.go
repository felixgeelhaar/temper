package risk

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestDetector_Analyze_HardcodedSecret(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"config.go": `package config
const password = "secret123"
`,
	}

	risks := d.Analyze(code)

	found := false
	for _, r := range risks {
		if r.ID == "SEC001" {
			found = true
			if r.Severity != domain.RiskSeverityHigh {
				t.Errorf("Severity = %q; want %q", r.Severity, domain.RiskSeverityHigh)
			}
			if r.Category != domain.RiskCategorySecurity {
				t.Errorf("Category = %q; want %q", r.Category, domain.RiskCategorySecurity)
			}
		}
	}

	if !found {
		t.Error("Expected to find SEC001 (hardcoded secret) risk")
	}
}

func TestDetector_Analyze_UncheckedError(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"main.go": `package main
func example() {
	_ = doSomething()
}
`,
	}

	risks := d.Analyze(code)

	found := false
	for _, r := range risks {
		if r.ID == "GO001" {
			found = true
		}
	}

	if !found {
		t.Error("Expected to find GO001 (unchecked error) risk")
	}
}

func TestDetector_Analyze_TodoComment(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"main.go": `package main
// TODO: implement this
func unfinished() {}
`,
	}

	risks := d.Analyze(code)

	found := false
	for _, r := range risks {
		if r.ID == "GO002" {
			found = true
			if r.Line != 2 {
				t.Errorf("Line = %d; want 2", r.Line)
			}
		}
	}

	if !found {
		t.Error("Expected to find GO002 (TODO comment) risk")
	}
}

func TestDetector_Analyze_PythonBareExcept(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"handler.py": `def handle():
    try:
        risky()
    except:
        pass
`,
	}

	risks := d.Analyze(code)

	found := false
	for _, r := range risks {
		if r.ID == "PY001" {
			found = true
		}
	}

	if !found {
		t.Error("Expected to find PY001 (bare except) risk")
	}
}

func TestDetector_Analyze_TypeScriptAnyType(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"utils.ts": `function process(data: any): void {
    console.log(data);
}
`,
	}

	risks := d.Analyze(code)

	anyTypeFound := false
	consoleLogFound := false
	for _, r := range risks {
		if r.ID == "TS004" {
			anyTypeFound = true
		}
		if r.ID == "TS003" {
			consoleLogFound = true
		}
	}

	if !anyTypeFound {
		t.Error("Expected to find TS004 (any type) risk")
	}
	if !consoleLogFound {
		t.Error("Expected to find TS003 (console.log) risk")
	}
}

func TestDetector_Analyze_InnerHTML(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"dom.js": `function render(html) {
    document.body.innerHTML = html;
}
`,
	}

	risks := d.Analyze(code)

	found := false
	for _, r := range risks {
		if r.ID == "TS001" {
			found = true
			if r.Severity != domain.RiskSeverityHigh {
				t.Errorf("Severity = %q; want %q", r.Severity, domain.RiskSeverityHigh)
			}
		}
	}

	if !found {
		t.Error("Expected to find TS001 (innerHTML) risk")
	}
}

func TestDetector_Analyze_NoRisks(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"clean.go": `package main

func Hello(name string) string {
	if name == "" {
		name = "World"
	}
	return "Hello, " + name + "!"
}
`,
	}

	risks := d.Analyze(code)

	// Should only find the string concatenation warning at most
	for _, r := range risks {
		if r.Severity == domain.RiskSeverityHigh {
			t.Errorf("Unexpected high severity risk: %s - %s", r.ID, r.Title)
		}
	}
}

func TestDetector_Analyze_LanguageFiltering(t *testing.T) {
	d := NewDetector()

	// Go-specific pattern should not apply to Python files
	code := map[string]string{
		"script.py": `# Some Python code
_ = do_something()
`,
	}

	risks := d.Analyze(code)

	for _, r := range risks {
		if r.ID == "GO001" {
			t.Error("GO001 (Go pattern) should not apply to Python files")
		}
	}
}

func TestDetector_Analyze_MultipleFiles(t *testing.T) {
	d := NewDetector()

	code := map[string]string{
		"config.go": `package config
const apiKey = "abc123"
`,
		"handler.py": `def handle():
    try:
        risky()
    except:
        pass
`,
	}

	risks := d.Analyze(code)

	goRiskFound := false
	pyRiskFound := false

	for _, r := range risks {
		if r.ID == "SEC001" && r.File == "config.go" {
			goRiskFound = true
		}
		if r.ID == "PY001" && r.File == "handler.py" {
			pyRiskFound = true
		}
	}

	if !goRiskFound {
		t.Error("Expected to find risk in Go file")
	}
	if !pyRiskFound {
		t.Error("Expected to find risk in Python file")
	}
}

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("NewDetector() returned nil")
	}
	if len(d.patterns) == 0 {
		t.Error("expected patterns to be initialized")
	}
}

func TestDetector_Analyze_Empty(t *testing.T) {
	d := NewDetector()
	risks := d.Analyze(map[string]string{})
	if len(risks) != 0 {
		t.Errorf("expected 0 risks for empty code, got %d", len(risks))
	}
}

func TestDetector_Analyze_InsecureRandom(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.go": `package main

import "math/rand"

func main() {
	x := rand.Intn(100)
	println(x)
}
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "SEC004" {
			found = true
			if r.Category != domain.RiskCategorySecurity {
				t.Errorf("expected Security category, got %s", r.Category)
			}
			if r.File != "main.go" {
				t.Errorf("expected main.go file, got %s", r.File)
			}
		}
	}
	if !found {
		t.Error("expected SEC004 pattern for math/rand")
	}
}

func TestDetector_Analyze_DeferClose(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.go": `package main

import "os"

func main() {
	file, _ := os.Open("test.txt")
	defer file.Close()
}
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "GO003" {
			found = true
			if r.Category != domain.RiskCategoryReliability {
				t.Errorf("expected Reliability category, got %s", r.Category)
			}
		}
	}
	if !found {
		t.Error("expected GO003 pattern for deferred close")
	}
}

func TestDetector_Analyze_PythonEval(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.py": `
result = eval(user_input)
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "PY002" {
			found = true
			if r.Severity != domain.RiskSeverityHigh {
				t.Errorf("expected High severity, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected PY002 pattern for eval")
	}
}

func TestDetector_Analyze_JSEval(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"app.js": `
const result = eval(userInput);
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "TS002" {
			found = true
			if r.Severity != domain.RiskSeverityHigh {
				t.Errorf("expected High severity, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected TS002 pattern for JavaScript eval")
	}
}

func TestDetector_Analyze_CommentedCode(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.go": `package main

// func oldFunction() {
//     println("old")
// }

func main() {}
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "GEN001" {
			found = true
		}
	}
	if !found {
		t.Error("expected GEN001 pattern for commented-out code")
	}
}

func TestDetector_Analyze_LineNumbers(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.go": `package main

import "math/rand"

func main() {
	x := rand.Intn(100)
	println(x)
}
`,
	}
	risks := d.Analyze(code)
	for _, r := range risks {
		if r.ID == "SEC004" {
			// math/rand is on line 3, rand.Intn is on line 6
			if r.Line != 3 && r.Line != 6 {
				t.Errorf("expected line 3 or 6, got %d", r.Line)
			}
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"main.go", "go"},
		{"handler.py", "python"},
		{"app.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"index.js", "javascript"},
		{"Button.jsx", "javascript"},
		{"README.md", ""},
		{"data.json", ""},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := detectLanguage(tc.filename)
			if got != tc.expected {
				t.Errorf("detectLanguage(%q) = %q, want %q", tc.filename, got, tc.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{[]string{"go"}, "go", true},
	}

	for _, tc := range tests {
		got := contains(tc.slice, tc.item)
		if got != tc.expected {
			t.Errorf("contains(%v, %q) = %v, want %v", tc.slice, tc.item, got, tc.expected)
		}
	}
}

func TestDetector_Analyze_StringConcatLoop(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.go": `package main

func main() {
	s := ""
	for i := 0; i < 10; i++ {
		s += "x"
	}
}
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "GO004" {
			found = true
			if r.Category != domain.RiskCategoryPerformance {
				t.Errorf("expected Performance category, got %s", r.Category)
			}
		}
	}
	if !found {
		t.Error("expected GO004 pattern for string concatenation")
	}
}

func TestDetector_Analyze_PythonPrint(t *testing.T) {
	d := NewDetector()
	code := map[string]string{
		"main.py": `
print("debug info")
`,
	}
	risks := d.Analyze(code)
	found := false
	for _, r := range risks {
		if r.ID == "PY003" {
			found = true
		}
	}
	if !found {
		t.Error("expected PY003 pattern for print statement")
	}
}
