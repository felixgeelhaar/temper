package patch

import (
	"regexp"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// Extractor parses intervention content to extract code patches
type Extractor struct {
	codeBlockRegex *regexp.Regexp
	fileHintRegex  *regexp.Regexp
}

// NewExtractor creates a new patch extractor
func NewExtractor() *Extractor {
	return &Extractor{
		// Matches fenced code blocks with optional language
		codeBlockRegex: regexp.MustCompile("(?s)```(\\w+)?\\s*\\n(.+?)```"),
		// Matches file hints like "// file: main.go" or "# file: main.py"
		fileHintRegex: regexp.MustCompile(`(?m)^(?://|#)\s*file:\s*(.+)$`),
	}
}

// CodeBlock represents an extracted code block
type CodeBlock struct {
	Language string
	Content  string
	File     string // inferred or specified file name
}

// ExtractPatches extracts patches from an intervention's content
func (e *Extractor) ExtractPatches(intervention *domain.Intervention, sessionID uuid.UUID, currentCode map[string]string) []*domain.Patch {
	blocks := e.extractCodeBlocks(intervention.Content)
	if len(blocks) == 0 {
		return nil
	}

	var patches []*domain.Patch
	for _, block := range blocks {
		file := e.determineFile(block, intervention.Targets, currentCode)
		if file == "" {
			continue // can't determine target file
		}

		original := ""
		if currentCode != nil {
			original = currentCode[file]
		}

		patch := &domain.Patch{
			ID:             uuid.New(),
			InterventionID: intervention.ID,
			SessionID:      sessionID,
			File:           file,
			Original:       original,
			Proposed:       block.Content,
			Diff:           generateDiff(file, original, block.Content),
			Description:    e.generateDescription(intervention, block),
			Status:         domain.PatchStatusPending,
		}
		patches = append(patches, patch)
	}

	return patches
}

func (e *Extractor) extractCodeBlocks(content string) []CodeBlock {
	matches := e.codeBlockRegex.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}

	var blocks []CodeBlock
	for _, match := range matches {
		lang := ""
		if len(match) > 1 {
			lang = match[1]
		}
		code := ""
		if len(match) > 2 {
			code = strings.TrimSpace(match[2])
		}

		if code == "" {
			continue
		}

		// Check for file hint in the code block
		file := ""
		if fileMatch := e.fileHintRegex.FindStringSubmatch(code); fileMatch != nil {
			file = strings.TrimSpace(fileMatch[1])
			// Remove the file hint from the code
			code = e.fileHintRegex.ReplaceAllString(code, "")
			code = strings.TrimSpace(code)
		}

		blocks = append(blocks, CodeBlock{
			Language: lang,
			Content:  code,
			File:     file,
		})
	}

	return blocks
}

func (e *Extractor) determineFile(block CodeBlock, targets []domain.Target, currentCode map[string]string) string {
	// 1. Check if file was specified in the code block
	if block.File != "" {
		return block.File
	}

	// 2. Check intervention targets
	if len(targets) > 0 {
		return targets[0].File
	}

	// 3. Infer from language and existing files
	ext := languageToExtension(block.Language)
	if ext != "" && currentCode != nil {
		for file := range currentCode {
			if strings.HasSuffix(file, ext) {
				return file
			}
		}
	}

	// 4. Default to main file based on language
	if ext != "" {
		return "main" + ext
	}

	return ""
}

func (e *Extractor) generateDescription(intervention *domain.Intervention, block CodeBlock) string {
	// Use intervention rationale or generate based on intent
	switch intervention.Intent {
	case domain.IntentHint:
		return "Suggested code change"
	case domain.IntentReview:
		return "Recommended improvement"
	case domain.IntentStuck:
		return "Solution to unblock progress"
	case domain.IntentExplain:
		return "Example code for explanation"
	default:
		return "Code patch from AI pairing"
	}
}

func languageToExtension(lang string) string {
	switch strings.ToLower(lang) {
	case "go", "golang":
		return ".go"
	case "python", "py":
		return ".py"
	case "typescript", "ts":
		return ".ts"
	case "javascript", "js":
		return ".js"
	case "tsx":
		return ".tsx"
	case "jsx":
		return ".jsx"
	default:
		return ""
	}
}

func generateDiff(file, original, proposed string) string {
	if original == "" {
		// New file - show all lines as additions
		lines := strings.Split(proposed, "\n")
		var diff strings.Builder
		diff.WriteString("--- /dev/null\n")
		diff.WriteString("+++ " + file + "\n")
		diff.WriteString("@@ -0,0 +1," + itoa(len(lines)) + " @@\n")
		for _, line := range lines {
			diff.WriteString("+" + line + "\n")
		}
		return diff.String()
	}

	// Generate unified diff
	return unifiedDiff(file, original, proposed)
}

func unifiedDiff(file, original, proposed string) string {
	origLines := strings.Split(original, "\n")
	newLines := strings.Split(proposed, "\n")

	var diff strings.Builder
	diff.WriteString("--- " + file + "\n")
	diff.WriteString("+++ " + file + "\n")

	// Simple diff: show full file replacement for now
	// A proper diff algorithm (Myers diff) would be more sophisticated
	diff.WriteString("@@ -1," + itoa(len(origLines)) + " +1," + itoa(len(newLines)) + " @@\n")

	for _, line := range origLines {
		diff.WriteString("-" + line + "\n")
	}
	for _, line := range newLines {
		diff.WriteString("+" + line + "\n")
	}

	return diff.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}
