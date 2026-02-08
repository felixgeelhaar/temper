package docindex

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Index provides SQLite-backed document storage and vector search
type Index struct {
	db *sql.DB
}

// NewIndex creates a new document index backed by the given database
func NewIndex(db *sql.DB) *Index {
	return &Index{db: db}
}

// SaveDocument persists a document and its sections
func (idx *Index) SaveDocument(doc *domain.Document) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Upsert document
	_, err = tx.Exec(`
		INSERT INTO documents (id, path, title, doc_type, content, hash, discovered_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path, title=excluded.title, doc_type=excluded.doc_type,
			content=excluded.content, hash=excluded.hash, discovered_at=excluded.discovered_at`,
		doc.Hash, doc.Path, doc.Title, string(doc.Type),
		doc.Content, doc.Hash, doc.DiscoveredAt,
	)
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}

	// Delete existing sections for this document
	_, err = tx.Exec("DELETE FROM document_sections WHERE document_id = ?", doc.Hash)
	if err != nil {
		return fmt.Errorf("delete sections: %w", err)
	}

	// Insert sections
	for _, section := range doc.Sections {
		_, err = tx.Exec(`
			INSERT INTO document_sections (document_id, heading, level, content)
			VALUES (?, ?, ?, ?)`,
			doc.Hash, section.Heading, section.Level, section.Content,
		)
		if err != nil {
			return fmt.Errorf("insert section: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateSectionEmbedding stores the embedding for a document section
func (idx *Index) UpdateSectionEmbedding(sectionID int64, embedding []byte) error {
	_, err := idx.db.Exec(
		"UPDATE document_sections SET embedding = ? WHERE id = ?",
		embedding, sectionID,
	)
	if err != nil {
		return fmt.Errorf("update embedding: %w", err)
	}
	return nil
}

// MarkIndexed records that a document has been indexed
func (idx *Index) MarkIndexed(docID string) error {
	_, err := idx.db.Exec(
		"UPDATE documents SET indexed_at = ? WHERE id = ?",
		time.Now(), docID,
	)
	return err
}

// SectionRow represents a section with its database ID for embedding updates
type SectionRow struct {
	ID         int64
	DocumentID string
	Heading    string
	Level      int
	Content    string
	Embedding  []byte
}

// ListUnindexedSections returns sections without embeddings
func (idx *Index) ListUnindexedSections() ([]SectionRow, error) {
	rows, err := idx.db.Query(`
		SELECT ds.id, ds.document_id, ds.heading, ds.level, ds.content
		FROM document_sections ds
		WHERE ds.embedding IS NULL
		ORDER BY ds.id`)
	if err != nil {
		return nil, fmt.Errorf("query unindexed sections: %w", err)
	}
	defer rows.Close()

	var sections []SectionRow
	for rows.Next() {
		var s SectionRow
		if err := rows.Scan(&s.ID, &s.DocumentID, &s.Heading, &s.Level, &s.Content); err != nil {
			return nil, fmt.Errorf("scan section: %w", err)
		}
		sections = append(sections, s)
	}
	return sections, rows.Err()
}

// ListAllSectionsWithEmbeddings returns all sections that have embeddings
func (idx *Index) ListAllSectionsWithEmbeddings() ([]SectionRow, error) {
	rows, err := idx.db.Query(`
		SELECT ds.id, ds.document_id, ds.heading, ds.level, ds.content, ds.embedding
		FROM document_sections ds
		WHERE ds.embedding IS NOT NULL
		ORDER BY ds.id`)
	if err != nil {
		return nil, fmt.Errorf("query sections: %w", err)
	}
	defer rows.Close()

	var sections []SectionRow
	for rows.Next() {
		var s SectionRow
		if err := rows.Scan(&s.ID, &s.DocumentID, &s.Heading, &s.Level, &s.Content, &s.Embedding); err != nil {
			return nil, fmt.Errorf("scan section: %w", err)
		}
		sections = append(sections, s)
	}
	return sections, rows.Err()
}

// GetDocument retrieves a document by ID
func (idx *Index) GetDocument(id string) (*domain.Document, error) {
	var doc domain.Document
	var docType string

	err := idx.db.QueryRow(`
		SELECT id, path, title, doc_type, content, hash, discovered_at
		FROM documents WHERE id = ?`, id).Scan(
		&doc.Hash, &doc.Path, &doc.Title, &docType,
		&doc.Content, &doc.Hash, &doc.DiscoveredAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	doc.Type = domain.DocumentType(docType)

	// Load sections
	rows, err := idx.db.Query(`
		SELECT heading, level, content FROM document_sections
		WHERE document_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("get sections: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s domain.DocumentSection
		if err := rows.Scan(&s.Heading, &s.Level, &s.Content); err != nil {
			return nil, fmt.Errorf("scan section: %w", err)
		}
		doc.Sections = append(doc.Sections, s)
	}

	return &doc, rows.Err()
}

// ListDocuments returns all indexed documents
func (idx *Index) ListDocuments() ([]domain.Document, error) {
	rows, err := idx.db.Query(`
		SELECT id, path, title, doc_type, hash, discovered_at, indexed_at
		FROM documents ORDER BY discovered_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var docs []domain.Document
	for rows.Next() {
		var doc domain.Document
		var docType string
		var indexedAt sql.NullTime
		if err := rows.Scan(&doc.Hash, &doc.Path, &doc.Title, &docType, &doc.Hash, &doc.DiscoveredAt, &indexedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		doc.Type = domain.DocumentType(docType)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// DeleteDocument removes a document and its sections (cascading)
func (idx *Index) DeleteDocument(id string) error {
	_, err := idx.db.Exec("DELETE FROM documents WHERE id = ?", id)
	return err
}

// DocumentExists checks if a document with the given hash already exists
func (idx *Index) DocumentExists(hash string) bool {
	var count int
	idx.db.QueryRow("SELECT COUNT(*) FROM documents WHERE hash = ?", hash).Scan(&count)
	return count > 0
}

// Stats returns index statistics
type IndexStats struct {
	TotalDocuments    int `json:"total_documents"`
	IndexedDocuments  int `json:"indexed_documents"`
	TotalSections     int `json:"total_sections"`
	EmbeddedSections  int `json:"embedded_sections"`
}

// Stats returns statistics about the index
func (idx *Index) Stats() (*IndexStats, error) {
	var stats IndexStats

	idx.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&stats.TotalDocuments)
	idx.db.QueryRow("SELECT COUNT(*) FROM documents WHERE indexed_at IS NOT NULL").Scan(&stats.IndexedDocuments)
	idx.db.QueryRow("SELECT COUNT(*) FROM document_sections").Scan(&stats.TotalSections)
	idx.db.QueryRow("SELECT COUNT(*) FROM document_sections WHERE embedding IS NOT NULL").Scan(&stats.EmbeddedSections)

	return &stats, nil
}
