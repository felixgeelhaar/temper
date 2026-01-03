package docindex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverer_Discover(t *testing.T) {
	// Create a temp directory with test docs
	tmpDir, err := os.MkdirTemp("", "docindex-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create docs/ directory
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test markdown files
	visionContent := `# Vision

## Mission
Our mission is to build great software.

## Goals
- Goal 1
- Goal 2
`
	if err := os.WriteFile(filepath.Join(docsDir, "vision.md"), []byte(visionContent), 0644); err != nil {
		t.Fatal(err)
	}

	prdContent := `# Product Requirements

## Overview
This is the product requirements document.

## Features
### Feature 1
Description of feature 1.
`
	if err := os.WriteFile(filepath.Join(docsDir, "prd.md"), []byte(prdContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create README
	readmeContent := `# My Project

A cool project.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test discovery
	discoverer := NewDiscoverer(tmpDir)
	docs, err := discoverer.Discover(DiscoverOptions{
		Paths: []string{"docs/", "README.md"},
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(docs) != 3 {
		t.Errorf("Expected 3 documents, got %d", len(docs))
	}

	// Verify document types
	foundVision := false
	foundPRD := false
	foundReadme := false
	for _, doc := range docs {
		switch doc.Type {
		case "vision":
			foundVision = true
			if len(doc.Sections) == 0 {
				t.Error("Vision doc should have sections")
			}
		case "prd":
			foundPRD = true
		case "readme":
			foundReadme = true
		}
	}

	if !foundVision {
		t.Error("Should have found vision document")
	}
	if !foundPRD {
		t.Error("Should have found PRD document")
	}
	if !foundReadme {
		t.Error("Should have found README document")
	}
}

func TestParser_ParseSections(t *testing.T) {
	parser := NewParser()

	content := `# Main Title

Some intro text.

## Section 1

Content for section 1.

### Subsection 1.1

More detailed content.

## Section 2

Content for section 2.
`

	sections := parser.ParseSections(content)

	if len(sections) < 4 {
		t.Errorf("Expected at least 4 sections, got %d", len(sections))
	}

	// Check first section
	if sections[0].Heading != "Main Title" {
		t.Errorf("Expected 'Main Title', got '%s'", sections[0].Heading)
	}
	if sections[0].Level != 1 {
		t.Errorf("Expected level 1, got %d", sections[0].Level)
	}
}

func TestInferDocType(t *testing.T) {
	tests := []struct {
		path     string
		content  string
		expected string
	}{
		{"docs/vision.md", "# Vision\nOur mission", "vision"},
		{"docs/prd.md", "# Product Requirements", "prd"},
		{"docs/tdd.md", "# Technical Design", "tdd"},
		{"docs/roadmap.md", "# Roadmap", "roadmap"},
		{"README.md", "# My Project", "readme"},
		{"docs/random.md", "Some content", "other"},
	}

	for _, tt := range tests {
		result := inferDocType(tt.path, tt.content)
		if string(result) != tt.expected {
			t.Errorf("inferDocType(%s) = %s, want %s", tt.path, result, tt.expected)
		}
	}
}

func TestFormatForLLM(t *testing.T) {
	// The FormatForLLM function would be tested here
	// For now, just verify it doesn't panic
	formatted := FormatForLLM(nil, 1000)
	if formatted != "" {
		t.Error("Expected empty string for nil docs")
	}
}
