package docindex

import (
	"regexp"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Parser extracts structured sections from markdown documents
type Parser struct {
	headingRegex *regexp.Regexp
}

// NewParser creates a new markdown parser
func NewParser() *Parser {
	return &Parser{
		headingRegex: regexp.MustCompile(`^(#{1,6})\s+(.+)$`),
	}
}

// ParseSections extracts all sections from markdown content
func (p *Parser) ParseSections(content string) []domain.DocumentSection {
	lines := strings.Split(content, "\n")
	var sections []domain.DocumentSection

	var currentSection *domain.DocumentSection
	var contentBuilder strings.Builder

	for _, line := range lines {
		if matches := p.headingRegex.FindStringSubmatch(line); matches != nil {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				if currentSection.Content != "" || currentSection.Heading != "" {
					sections = append(sections, *currentSection)
				}
				contentBuilder.Reset()
			}

			// Start new section
			level := len(matches[1])
			heading := strings.TrimSpace(matches[2])
			currentSection = &domain.DocumentSection{
				Heading: heading,
				Level:   level,
			}
		} else if currentSection != nil {
			// Add line to current section content
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		} else {
			// Content before first heading - create implicit section
			if strings.TrimSpace(line) != "" {
				if currentSection == nil {
					currentSection = &domain.DocumentSection{
						Heading: "Introduction",
						Level:   0,
					}
				}
				contentBuilder.WriteString(line)
				contentBuilder.WriteString("\n")
			}
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuilder.String())
		if currentSection.Content != "" || currentSection.Heading != "" {
			sections = append(sections, *currentSection)
		}
	}

	return sections
}

// ExtractKeywords extracts potential keywords/tags from section content
func (p *Parser) ExtractKeywords(section domain.DocumentSection) []string {
	// Common tech/business keywords to look for
	keywords := []string{
		"api", "rest", "graphql", "database", "authentication", "authorization",
		"performance", "security", "scalability", "availability", "reliability",
		"user", "admin", "dashboard", "integration", "sync", "async",
		"real-time", "batch", "queue", "cache", "storage", "backup",
	}

	content := strings.ToLower(section.Content + " " + section.Heading)
	var found []string

	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			found = append(found, kw)
		}
	}

	return found
}

// TruncateContent limits content length while preserving paragraph boundaries
func TruncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	// Try to break at paragraph
	paragraphs := strings.Split(content, "\n\n")
	var result strings.Builder
	for _, para := range paragraphs {
		if result.Len()+len(para)+2 > maxLen {
			break
		}
		if result.Len() > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(para)
	}

	if result.Len() == 0 {
		// No complete paragraph fits, just truncate
		return content[:maxLen-3] + "..."
	}

	return result.String()
}

// FormatForLLM formats documents for inclusion in LLM prompts
func FormatForLLM(docs []domain.Document, maxTokens int) string {
	// Rough estimate: 1 token ~= 4 chars
	maxChars := maxTokens * 4
	var result strings.Builder

	for _, doc := range docs {
		if result.Len() >= maxChars {
			break
		}

		result.WriteString("---\n")
		result.WriteString("# ")
		result.WriteString(doc.Title)
		result.WriteString(" (")
		result.WriteString(doc.Path)
		result.WriteString(")\n\n")

		for _, section := range doc.Sections {
			if result.Len() >= maxChars {
				break
			}

			// Add heading with appropriate level
			for i := 0; i < section.Level; i++ {
				result.WriteString("#")
			}
			if section.Level > 0 {
				result.WriteString(" ")
			}
			result.WriteString(section.Heading)
			result.WriteString("\n\n")

			// Add content (truncated if needed)
			remaining := maxChars - result.Len()
			if remaining > 0 {
				content := section.Content
				if len(content) > remaining {
					content = TruncateContent(content, remaining)
				}
				result.WriteString(content)
				result.WriteString("\n\n")
			}
		}
	}

	return result.String()
}

// SummarizeDocument creates a brief summary of document structure
func SummarizeDocument(doc domain.Document) string {
	var sb strings.Builder
	sb.WriteString(doc.Title)
	sb.WriteString(" (")
	sb.WriteString(string(doc.Type))
	sb.WriteString("): ")

	// List top-level headings
	var headings []string
	for _, section := range doc.Sections {
		if section.Level <= 2 {
			headings = append(headings, section.Heading)
		}
	}

	if len(headings) > 5 {
		headings = headings[:5]
		headings = append(headings, "...")
	}

	sb.WriteString(strings.Join(headings, ", "))
	return sb.String()
}
