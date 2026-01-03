package docindex

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// DefaultDocPaths are common documentation locations to search
var DefaultDocPaths = []string{
	"docs",
	"doc",
	"documentation",
	"README.md",
	"README",
	"DESIGN.md",
	"ARCHITECTURE.md",
	"PRD.md",
	"SPEC.md",
}

// DefaultDocExtensions are file extensions to include
var DefaultDocExtensions = []string{
	".md",
	".markdown",
	".txt",
	".rst",
}

// Discoverer finds documentation files in a project
type Discoverer struct {
	basePath   string
	extensions []string
}

// NewDiscoverer creates a new document discoverer
func NewDiscoverer(basePath string) *Discoverer {
	return &Discoverer{
		basePath:   basePath,
		extensions: DefaultDocExtensions,
	}
}

// WithExtensions sets custom file extensions to search for
func (d *Discoverer) WithExtensions(exts []string) *Discoverer {
	d.extensions = exts
	return d
}

// DiscoverOptions configures document discovery
type DiscoverOptions struct {
	Paths     []string // Specific paths to search (relative to basePath)
	Recursive bool     // Search subdirectories
	MaxDepth  int      // Maximum directory depth (0 = unlimited)
}

// DefaultDiscoverOptions returns sensible defaults
func DefaultDiscoverOptions() DiscoverOptions {
	return DiscoverOptions{
		Paths:     DefaultDocPaths,
		Recursive: true,
		MaxDepth:  3,
	}
}

// Discover finds all documentation files matching the options
func (d *Discoverer) Discover(opts DiscoverOptions) ([]domain.Document, error) {
	var docs []domain.Document
	seen := make(map[string]bool)

	for _, searchPath := range opts.Paths {
		fullPath := filepath.Join(d.basePath, searchPath)

		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			continue // Path doesn't exist, skip
		}
		if err != nil {
			continue // Other error, skip
		}

		if info.IsDir() {
			// Search directory for docs
			found, err := d.discoverInDir(fullPath, opts.Recursive, opts.MaxDepth, 0)
			if err != nil {
				continue
			}
			for _, doc := range found {
				if !seen[doc.Path] {
					seen[doc.Path] = true
					docs = append(docs, doc)
				}
			}
		} else {
			// Single file
			if d.isDocFile(fullPath) && !seen[fullPath] {
				seen[fullPath] = true
				doc, err := d.loadDocument(fullPath)
				if err != nil {
					continue
				}
				docs = append(docs, doc)
			}
		}
	}

	return docs, nil
}

// discoverInDir recursively searches a directory for documentation
func (d *Discoverer) discoverInDir(dir string, recursive bool, maxDepth, currentDepth int) ([]domain.Document, error) {
	if maxDepth > 0 && currentDepth >= maxDepth {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var docs []domain.Document
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			if recursive && !isIgnoredDir(entry.Name()) {
				subDocs, err := d.discoverInDir(fullPath, recursive, maxDepth, currentDepth+1)
				if err != nil {
					continue
				}
				docs = append(docs, subDocs...)
			}
		} else if d.isDocFile(fullPath) {
			doc, err := d.loadDocument(fullPath)
			if err != nil {
				continue
			}
			docs = append(docs, doc)
		}
	}

	return docs, nil
}

// isDocFile checks if a file has a documentation extension
func (d *Discoverer) isDocFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, docExt := range d.extensions {
		if ext == docExt {
			return true
		}
	}
	// Also match files without extension that look like docs
	name := strings.ToLower(filepath.Base(path))
	return name == "readme" || name == "changelog" || name == "license"
}

// isIgnoredDir returns true for directories that should be skipped
func isIgnoredDir(name string) bool {
	ignored := []string{
		".git",
		".github",
		"node_modules",
		"vendor",
		".vscode",
		".idea",
		"__pycache__",
		".temper",
		".specs",
	}
	lower := strings.ToLower(name)
	for _, ig := range ignored {
		if lower == ig {
			return true
		}
	}
	return false
}

// loadDocument reads and parses a documentation file
func (d *Discoverer) loadDocument(path string) (domain.Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.Document{}, err
	}

	relPath, err := filepath.Rel(d.basePath, path)
	if err != nil {
		relPath = path
	}

	doc := domain.Document{
		Path:         relPath,
		Title:        inferTitle(path, string(content)),
		Type:         inferDocType(path, string(content)),
		Content:      string(content),
		DiscoveredAt: time.Now(),
	}
	doc.Hash = doc.ComputeHash()

	// Parse sections using the parser
	parser := NewParser()
	doc.Sections = parser.ParseSections(string(content))

	return doc, nil
}

// inferTitle extracts a title from the document
func inferTitle(path string, content string) string {
	// Try to find first H1 heading
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	// Fall back to filename
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.Title(strings.ReplaceAll(name, "-", " "))
}

// inferDocType determines the document type from path and content
func inferDocType(path string, content string) domain.DocumentType {
	lower := strings.ToLower(path)
	lowerContent := strings.ToLower(content)

	// Check path first
	switch {
	case strings.Contains(lower, "vision"):
		return domain.DocTypeVision
	case strings.Contains(lower, "prd") || strings.Contains(lower, "product"):
		return domain.DocTypePRD
	case strings.Contains(lower, "tdd") || strings.Contains(lower, "technical"):
		return domain.DocTypeTDD
	case strings.Contains(lower, "roadmap"):
		return domain.DocTypeRoadmap
	case strings.Contains(lower, "readme"):
		return domain.DocTypeReadme
	case strings.Contains(lower, "design") || strings.Contains(lower, "architecture"):
		return domain.DocTypeDesign
	case strings.Contains(lower, "api"):
		return domain.DocTypeAPI
	}

	// Check content for hints
	switch {
	case strings.Contains(lowerContent, "product requirements") ||
		strings.Contains(lowerContent, "user stories"):
		return domain.DocTypePRD
	case strings.Contains(lowerContent, "technical design") ||
		strings.Contains(lowerContent, "system design"):
		return domain.DocTypeTDD
	case strings.Contains(lowerContent, "vision") && strings.Contains(lowerContent, "mission"):
		return domain.DocTypeVision
	case strings.Contains(lowerContent, "milestone") || strings.Contains(lowerContent, "timeline"):
		return domain.DocTypeRoadmap
	}

	return domain.DocTypeOther
}
