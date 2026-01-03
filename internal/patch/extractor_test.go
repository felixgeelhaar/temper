package patch

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

func TestExtractor_ExtractCodeBlocks(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		name    string
		content string
		want    int
		blocks  []CodeBlock
	}{
		{
			name:    "no code blocks",
			content: "Just some text without any code",
			want:    0,
		},
		{
			name: "single go block",
			content: `Here's some code:
` + "```go" + `
func main() {
    fmt.Println("hello")
}
` + "```",
			want: 1,
			blocks: []CodeBlock{
				{Language: "go", Content: "func main() {\n    fmt.Println(\"hello\")\n}"},
			},
		},
		{
			name: "multiple blocks",
			content: `First block:
` + "```python" + `
def hello():
    print("hi")
` + "```" + `
Second block:
` + "```go" + `
package main
` + "```",
			want: 2,
		},
		{
			name: "block with file hint",
			content: "```go\n// file: main.go\npackage main\n```",
			want:    1,
			blocks: []CodeBlock{
				{Language: "go", Content: "package main", File: "main.go"},
			},
		},
		{
			name: "block without language",
			content: "```\nsome code\n```",
			want:    1,
			blocks: []CodeBlock{
				{Language: "", Content: "some code"},
			},
		},
		{
			name:    "empty block ignored",
			content: "```go\n   \n```",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := e.extractCodeBlocks(tt.content)
			if len(blocks) != tt.want {
				t.Errorf("extractCodeBlocks() got %d blocks, want %d", len(blocks), tt.want)
			}

			// Check specific block content if provided
			for i, expected := range tt.blocks {
				if i >= len(blocks) {
					break
				}
				if blocks[i].Language != expected.Language {
					t.Errorf("block[%d].Language = %q, want %q", i, blocks[i].Language, expected.Language)
				}
				if blocks[i].Content != expected.Content {
					t.Errorf("block[%d].Content = %q, want %q", i, blocks[i].Content, expected.Content)
				}
				if blocks[i].File != expected.File {
					t.Errorf("block[%d].File = %q, want %q", i, blocks[i].File, expected.File)
				}
			}
		})
	}
}

func TestExtractor_DetermineFile(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		name        string
		block       CodeBlock
		targets     []domain.Target
		currentCode map[string]string
		want        string
	}{
		{
			name:  "uses block file if specified",
			block: CodeBlock{File: "explicit.go"},
			want:  "explicit.go",
		},
		{
			name:    "uses intervention target",
			block:   CodeBlock{Language: "go"},
			targets: []domain.Target{{File: "target.go"}},
			want:    "target.go",
		},
		{
			name:        "infers from language and existing files",
			block:       CodeBlock{Language: "python"},
			currentCode: map[string]string{"app.py": "# code"},
			want:        "app.py",
		},
		{
			name:  "defaults to main file for language",
			block: CodeBlock{Language: "go"},
			want:  "main.go",
		},
		{
			name:  "returns empty for unknown language",
			block: CodeBlock{Language: "unknown"},
			want:  "",
		},
		{
			name:  "typescript extension",
			block: CodeBlock{Language: "typescript"},
			want:  "main.ts",
		},
		{
			name:  "javascript extension",
			block: CodeBlock{Language: "javascript"},
			want:  "main.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.determineFile(tt.block, tt.targets, tt.currentCode)
			if got != tt.want {
				t.Errorf("determineFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractor_GenerateDescription(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		name   string
		intent domain.Intent
		want   string
	}{
		{"hint", domain.IntentHint, "Suggested code change"},
		{"review", domain.IntentReview, "Recommended improvement"},
		{"stuck", domain.IntentStuck, "Solution to unblock progress"},
		{"explain", domain.IntentExplain, "Example code for explanation"},
		{"unknown", domain.Intent{}, "Code patch from AI pairing"}, // Zero value for unknown
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intervention := &domain.Intervention{Intent: tt.intent}
			got := e.generateDescription(intervention, CodeBlock{})
			if got != tt.want {
				t.Errorf("generateDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractor_ExtractPatches(t *testing.T) {
	e := NewExtractor()
	sessionID := uuid.New()

	tests := []struct {
		name         string
		intervention *domain.Intervention
		currentCode  map[string]string
		wantPatches  int
	}{
		{
			name: "extracts single patch",
			intervention: &domain.Intervention{
				ID:      uuid.New(),
				Intent:  domain.IntentStuck,
				Content: "Here's the fix:\n```go\npackage main\n```",
			},
			currentCode: map[string]string{"main.go": "// old code"},
			wantPatches: 1,
		},
		{
			name: "extracts multiple patches",
			intervention: &domain.Intervention{
				ID:      uuid.New(),
				Intent:  domain.IntentStuck,
				Content: "```go\n// file: a.go\npackage a\n```\n```go\n// file: b.go\npackage b\n```",
			},
			wantPatches: 2,
		},
		{
			name: "no patches from text-only",
			intervention: &domain.Intervention{
				ID:      uuid.New(),
				Intent:  domain.IntentHint,
				Content: "Consider using a loop",
			},
			wantPatches: 0,
		},
		{
			name: "uses target from intervention",
			intervention: &domain.Intervention{
				ID:      uuid.New(),
				Intent:  domain.IntentReview,
				Content: "```go\nfunc better() {}\n```",
				Targets: []domain.Target{{File: "specific.go", StartLine: 10, EndLine: 10}},
			},
			wantPatches: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := e.ExtractPatches(tt.intervention, sessionID, tt.currentCode)
			if len(patches) != tt.wantPatches {
				t.Errorf("ExtractPatches() got %d patches, want %d", len(patches), tt.wantPatches)
			}

			// Verify patch fields if patches exist
			for _, patch := range patches {
				if patch.ID == uuid.Nil {
					t.Error("patch.ID should not be nil")
				}
				if patch.InterventionID != tt.intervention.ID {
					t.Error("patch.InterventionID should match intervention")
				}
				if patch.SessionID != sessionID {
					t.Error("patch.SessionID should match")
				}
				if patch.File == "" {
					t.Error("patch.File should not be empty")
				}
				if patch.Status != domain.PatchStatusPending {
					t.Errorf("patch.Status = %v, want Pending", patch.Status)
				}
			}
		})
	}
}

func TestLanguageToExtension(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"go", ".go"},
		{"golang", ".go"},
		{"python", ".py"},
		{"py", ".py"},
		{"typescript", ".ts"},
		{"ts", ".ts"},
		{"javascript", ".js"},
		{"js", ".js"},
		{"tsx", ".tsx"},
		{"jsx", ".jsx"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := languageToExtension(tt.lang)
			if got != tt.want {
				t.Errorf("languageToExtension(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestGenerateDiff_NewFile(t *testing.T) {
	diff := generateDiff("main.go", "", "package main\n\nfunc main() {}\n")

	if diff == "" {
		t.Error("generateDiff() returned empty string")
	}

	// Check it has new file indicators
	if !contains(diff, "--- /dev/null") {
		t.Error("new file diff should have /dev/null source")
	}
	if !contains(diff, "+++ main.go") {
		t.Error("new file diff should have target filename")
	}
	if !contains(diff, "+package main") {
		t.Error("new file diff should have + prefixed lines")
	}
}

func TestGenerateDiff_ModifiedFile(t *testing.T) {
	original := "package main\n\nfunc old() {}\n"
	proposed := "package main\n\nfunc new() {}\n"

	diff := generateDiff("main.go", original, proposed)

	if diff == "" {
		t.Error("generateDiff() returned empty string")
	}

	// Check unified diff format
	if !contains(diff, "--- main.go") {
		t.Error("modified diff should have source filename")
	}
	if !contains(diff, "+++ main.go") {
		t.Error("modified diff should have target filename")
	}
	if !contains(diff, "-func old()") {
		t.Error("modified diff should have removed lines")
	}
	if !contains(diff, "+func new()") {
		t.Error("modified diff should have added lines")
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{9999, "9999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := itoa(tt.n)
			if got != tt.want {
				t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
