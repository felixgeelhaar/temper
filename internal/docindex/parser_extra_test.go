package docindex

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestParser_ExtractKeywords(t *testing.T) {
	parser := NewParser()
	section := domain.DocumentSection{
		Heading: "API Design",
		Content: "This covers database access and authentication flows.",
	}

	keywords := parser.ExtractKeywords(section)

	assertContains := func(target string) {
		for _, kw := range keywords {
			if kw == target {
				return
			}
		}
		t.Errorf("expected keyword %q to be present", target)
	}

	assertContains("api")
	assertContains("database")
	assertContains("authentication")
}

func TestTruncateContent_ParagraphBoundary(t *testing.T) {
	content := "first paragraph\n\nsecond paragraph\n\nthird paragraph"
	maxLen := len("first paragraph\n\nsecond paragraph") + 1

	truncated := TruncateContent(content, maxLen)
	expected := "first paragraph\n\nsecond paragraph"

	if truncated != expected {
		t.Errorf("TruncateContent() = %q; want %q", truncated, expected)
	}
}

func TestTruncateContent_FallbackEllipsis(t *testing.T) {
	content := "this is a long paragraph"
	truncated := TruncateContent(content, 10)

	if !strings.HasSuffix(truncated, "...") {
		t.Errorf("TruncateContent() should add ellipsis, got %q", truncated)
	}
	if len(truncated) != 10 {
		t.Errorf("TruncateContent() length = %d; want 10", len(truncated))
	}
}

func TestFormatForLLM_TruncatesContent(t *testing.T) {
	docs := []domain.Document{
		{
			Title: "Doc",
			Path:  "docs/doc.md",
			Sections: []domain.DocumentSection{
				{
					Heading: "Intro",
					Level:   1,
					Content: strings.Repeat("a", 100),
				},
			},
		},
	}

	formatted := FormatForLLM(docs, 10)

	if !strings.Contains(formatted, "# Doc") {
		t.Errorf("FormatForLLM() should include title, got %q", formatted)
	}
	if !strings.Contains(formatted, "Intro") {
		t.Errorf("FormatForLLM() should include section heading, got %q", formatted)
	}
	if !strings.Contains(formatted, "...") {
		t.Errorf("FormatForLLM() should truncate content with ellipsis, got %q", formatted)
	}
}

func TestSummarizeDocument(t *testing.T) {
	doc := domain.Document{
		Title: "Summary",
		Type:  domain.DocTypePRD,
		Sections: []domain.DocumentSection{
			{Heading: "One", Level: 1},
			{Heading: "Two", Level: 2},
			{Heading: "Three", Level: 2},
			{Heading: "Four", Level: 1},
			{Heading: "Five", Level: 1},
			{Heading: "Six", Level: 1},
			{Heading: "Deep", Level: 3},
		},
	}

	summary := SummarizeDocument(doc)

	if !strings.Contains(summary, "Summary (prd):") {
		t.Errorf("SummarizeDocument() missing title/type, got %q", summary)
	}
	if strings.Contains(summary, "Deep") {
		t.Errorf("SummarizeDocument() should skip level 3 headings, got %q", summary)
	}
	if !strings.Contains(summary, "...") {
		t.Errorf("SummarizeDocument() should include ellipsis when trimmed, got %q", summary)
	}
}
