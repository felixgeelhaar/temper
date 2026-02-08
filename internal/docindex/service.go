package docindex

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// Service orchestrates the document indexing pipeline:
// discover → parse → embed → store
type Service struct {
	index     *Index
	retriever *Retriever
	embedder  Embedder
	logger    *slog.Logger
}

// NewService creates a new docindex service
func NewService(db *sql.DB, embedder Embedder) *Service {
	if embedder == nil {
		embedder = NewKeywordEmbedder(256)
	}
	idx := NewIndex(db)
	ret := NewRetriever(idx, embedder)

	return &Service{
		index:     idx,
		retriever: ret,
		embedder:  embedder,
		logger:    slog.Default(),
	}
}

// IndexResult holds the result of an indexing operation
type IndexResult struct {
	DocumentsFound    int `json:"documents_found"`
	DocumentsIndexed  int `json:"documents_indexed"`
	DocumentsSkipped  int `json:"documents_skipped"`
	SectionsEmbedded  int `json:"sections_embedded"`
	Errors            int `json:"errors"`
}

// IndexDirectory discovers and indexes documents in the given directory
func (s *Service) IndexDirectory(ctx context.Context, basePath string) (*IndexResult, error) {
	result := &IndexResult{}

	// Discover documents
	discoverer := NewDiscoverer(basePath)
	docs, err := discoverer.Discover(DefaultDiscoverOptions())
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}
	result.DocumentsFound = len(docs)

	// Store and embed each document
	for i := range docs {
		doc := &docs[i]

		// Skip if already indexed with same content
		if s.index.DocumentExists(doc.Hash) {
			result.DocumentsSkipped++
			continue
		}

		// Store document and sections
		if err := s.index.SaveDocument(doc); err != nil {
			s.logger.Error("failed to save document", "path", doc.Path, "error", err)
			result.Errors++
			continue
		}
		result.DocumentsIndexed++

		// Embed the sections
		embedded, err := s.embedSections(ctx, doc.Hash)
		if err != nil {
			s.logger.Error("failed to embed sections", "path", doc.Path, "error", err)
			result.Errors++
			continue
		}
		result.SectionsEmbedded += embedded

		// Mark as indexed
		s.index.MarkIndexed(doc.Hash)
	}

	return result, nil
}

// embedSections embeds all unindexed sections for a document
func (s *Service) embedSections(ctx context.Context, docID string) (int, error) {
	sections, err := s.index.ListUnindexedSections()
	if err != nil {
		return 0, err
	}

	// Filter to sections belonging to this document
	var toEmbed []SectionRow
	for _, sec := range sections {
		if sec.DocumentID == docID {
			toEmbed = append(toEmbed, sec)
		}
	}

	if len(toEmbed) == 0 {
		return 0, nil
	}

	// Batch embed
	texts := make([]string, len(toEmbed))
	for i, sec := range toEmbed {
		// Combine heading and content for richer embedding
		texts[i] = sec.Heading + "\n" + sec.Content
	}

	embeddings, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("embed batch: %w", err)
	}

	// Store embeddings
	count := 0
	for i, sec := range toEmbed {
		encoded := EncodeEmbedding(embeddings[i])
		if err := s.index.UpdateSectionEmbedding(sec.ID, encoded); err != nil {
			s.logger.Error("failed to store embedding", "section_id", sec.ID, "error", err)
			continue
		}
		count++
	}

	return count, nil
}

// Search performs a similarity search
func (s *Service) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	return s.retriever.Search(ctx, query, topK)
}

// Stats returns indexing statistics
func (s *Service) Stats() (*IndexStats, error) {
	return s.index.Stats()
}

// ListDocuments returns all indexed documents
func (s *Service) ListDocuments() ([]DocumentSummary, error) {
	docs, err := s.index.ListDocuments()
	if err != nil {
		return nil, err
	}

	summaries := make([]DocumentSummary, len(docs))
	for i, doc := range docs {
		summaries[i] = DocumentSummary{
			ID:           doc.Hash,
			Path:         doc.Path,
			Title:        doc.Title,
			Type:         string(doc.Type),
			SectionCount: len(doc.Sections),
		}
	}
	return summaries, nil
}

// DocumentSummary is a lightweight document representation
type DocumentSummary struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	SectionCount int    `json:"section_count"`
}

// ReindexAll re-embeds all sections (useful when switching embedding providers)
func (s *Service) ReindexAll(ctx context.Context) (*IndexResult, error) {
	result := &IndexResult{}

	sections, err := s.index.ListUnindexedSections()
	if err != nil {
		return nil, err
	}

	if len(sections) == 0 {
		return result, nil
	}

	texts := make([]string, len(sections))
	for i, sec := range sections {
		texts[i] = sec.Heading + "\n" + sec.Content
	}

	embeddings, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed batch: %w", err)
	}

	for i, sec := range sections {
		encoded := EncodeEmbedding(embeddings[i])
		if err := s.index.UpdateSectionEmbedding(sec.ID, encoded); err != nil {
			result.Errors++
			continue
		}
		result.SectionsEmbedded++
	}

	return result, nil
}
