package domain

import (
	"strings"
	"testing"
)

func TestDocument_ContentHelpers(t *testing.T) {
	doc := Document{
		Content: "Hello\nWorld",
		Sections: []DocumentSection{
			{Heading: "Intro", Level: 1, Content: "Hello"},
			{Heading: "Details", Level: 2, Content: "More"},
			{Heading: "Appendix", Level: 3, Content: "Extra"},
		},
	}

	if doc.ComputeHash() == "" {
		t.Error("ComputeHash() should return a non-empty hash")
	}

	if doc.TotalContentLength() != len("Hello")+len("More")+len("Extra") {
		t.Errorf("TotalContentLength() = %d; want %d", doc.TotalContentLength(), len("Hello")+len("More")+len("Extra"))
	}

	sections := doc.GetSectionsByLevel(2)
	if len(sections) != 1 || sections[0].Heading != "Details" {
		t.Errorf("GetSectionsByLevel(2) = %v; want Details", sections)
	}

	found := doc.FindSection("intro")
	if found == nil || found.Heading != "Intro" {
		t.Errorf("FindSection() = %v; want Intro", found)
	}
}

func TestAuthoringContext_DocumentAccessors(t *testing.T) {
	doc := Document{
		Path:  "docs/spec.md",
		Title: "Spec",
		Type:  DocTypePRD,
		Sections: []DocumentSection{
			{Heading: "Overview", Level: 1, Content: "Content one"},
			{Heading: "Scope", Level: 1, Content: "Content two"},
		},
	}

	ctx := AuthoringContext{Documents: []Document{doc}}

	if !ctx.HasDocuments() {
		t.Error("HasDocuments() should return true")
	}

	byType := ctx.GetDocumentByType(DocTypePRD)
	if byType == nil || byType.Title != "Spec" {
		t.Errorf("GetDocumentByType() = %v; want Spec", byType)
	}

	content := ctx.GetAllContent(20)
	if len(content) != 20 {
		t.Errorf("GetAllContent() length = %d; want 20", len(content))
	}
	if !strings.HasPrefix(content, "## Overview\n") {
		t.Errorf("GetAllContent() prefix mismatch, got %q", content)
	}
}
