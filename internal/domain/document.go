package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// DocumentType categorizes documents by purpose
type DocumentType string

const (
	DocTypeVision   DocumentType = "vision"
	DocTypePRD      DocumentType = "prd"
	DocTypeTDD      DocumentType = "tdd"
	DocTypeRoadmap  DocumentType = "roadmap"
	DocTypeReadme   DocumentType = "readme"
	DocTypeDesign   DocumentType = "design"
	DocTypeAPI      DocumentType = "api"
	DocTypeOther    DocumentType = "other"
)

// Document represents a project documentation file
type Document struct {
	Path         string            `json:"path"`
	Title        string            `json:"title"`
	Type         DocumentType      `json:"type"`
	Sections     []DocumentSection `json:"sections"`
	Content      string            `json:"content,omitempty"`
	Hash         string            `json:"hash"`
	DiscoveredAt time.Time         `json:"discovered_at"`
}

// DocumentSection represents a parsed section of a document
type DocumentSection struct {
	Heading string   `json:"heading"`
	Level   int      `json:"level"` // h1=1, h2=2, etc.
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"` // extracted keywords
}

// ComputeHash calculates the SHA256 hash of the document content
func (d *Document) ComputeHash() string {
	h := sha256.Sum256([]byte(d.Content))
	return hex.EncodeToString(h[:])
}

// TotalContentLength returns the total length of all section content
func (d *Document) TotalContentLength() int {
	total := 0
	for _, s := range d.Sections {
		total += len(s.Content)
	}
	return total
}

// GetSectionsByLevel returns sections at a specific heading level
func (d *Document) GetSectionsByLevel(level int) []DocumentSection {
	var result []DocumentSection
	for _, s := range d.Sections {
		if s.Level == level {
			result = append(result, s)
		}
	}
	return result
}

// FindSection returns the first section matching the heading (case-insensitive contains)
func (d *Document) FindSection(heading string) *DocumentSection {
	for i, s := range d.Sections {
		if containsIgnoreCase(s.Heading, heading) {
			return &d.Sections[i]
		}
	}
	return nil
}

// AuthoringSuggestion represents an AI-generated suggestion for a spec section
type AuthoringSuggestion struct {
	ID         string  `json:"id"`
	Section    string  `json:"section"`    // goals, features, acceptance_criteria, non_functional
	Value      any     `json:"value"`      // string for goals, Feature for features, etc.
	Source     string  `json:"source"`     // e.g., "docs/vision.md#Mission"
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reasoning  string  `json:"reasoning,omitempty"`
}

// AuthoringContext holds the state for a spec authoring session
type AuthoringContext struct {
	Documents      []Document            `json:"documents"`
	Spec           *ProductSpec          `json:"spec"`
	CurrentSection string                `json:"current_section"`
	Suggestions    []AuthoringSuggestion `json:"suggestions,omitempty"`
	Applied        []string              `json:"applied,omitempty"` // suggestion IDs that were applied
}

// HasDocuments returns true if there are discovered documents
func (ac *AuthoringContext) HasDocuments() bool {
	return len(ac.Documents) > 0
}

// GetDocumentByType returns the first document of the given type
func (ac *AuthoringContext) GetDocumentByType(docType DocumentType) *Document {
	for i, doc := range ac.Documents {
		if doc.Type == docType {
			return &ac.Documents[i]
		}
	}
	return nil
}

// GetAllContent returns concatenated content from all documents (for LLM context)
func (ac *AuthoringContext) GetAllContent(maxLength int) string {
	var content string
	for _, doc := range ac.Documents {
		for _, section := range doc.Sections {
			content += "## " + section.Heading + "\n"
			content += section.Content + "\n\n"
			if maxLength > 0 && len(content) >= maxLength {
				return content[:maxLength]
			}
		}
	}
	return content
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			(len(s) > 0 && containsIgnoreCaseImpl(s, substr)))
}

func containsIgnoreCaseImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, substr, i) {
			return true
		}
	}
	return false
}

func equalFoldAt(s, substr string, start int) bool {
	for i := 0; i < len(substr); i++ {
		c1 := s[start+i]
		c2 := substr[i]
		if c1 == c2 {
			continue
		}
		// Simple ASCII case folding
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}
