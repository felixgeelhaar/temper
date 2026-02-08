package docindex

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

// openTestDB creates a fresh in-memory SQLite database with the document schema
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id              TEXT PRIMARY KEY,
		path            TEXT NOT NULL,
		title           TEXT NOT NULL DEFAULT '',
		doc_type        TEXT NOT NULL DEFAULT 'other',
		content         TEXT NOT NULL DEFAULT '',
		hash            TEXT NOT NULL DEFAULT '',
		discovered_at   DATETIME NOT NULL DEFAULT (datetime('now')),
		indexed_at      DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(hash);

	CREATE TABLE IF NOT EXISTS document_sections (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id     TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		heading         TEXT NOT NULL DEFAULT '',
		level           INTEGER NOT NULL DEFAULT 0,
		content         TEXT NOT NULL DEFAULT '',
		embedding       BLOB,
		created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_doc_sections_document ON document_sections(document_id);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func testDocument() *domain.Document {
	return &domain.Document{
		Path:         "docs/test.md",
		Title:        "Test Document",
		Type:         domain.DocTypeOther,
		Content:      "# Test\n\nSome content.",
		Hash:         "abc123",
		DiscoveredAt: time.Now(),
		Sections: []domain.DocumentSection{
			{Heading: "Test", Level: 1, Content: "Some content."},
			{Heading: "Details", Level: 2, Content: "More detailed content here."},
		},
	}
}

func TestIndex_SaveDocument(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	if err := idx.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}

	// Verify document was saved
	var count int
	db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 document; got %d", count)
	}

	// Verify sections were saved
	db.QueryRow("SELECT COUNT(*) FROM document_sections").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 sections; got %d", count)
	}
}

func TestIndex_SaveDocument_Upsert(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	if err := idx.SaveDocument(doc); err != nil {
		t.Fatalf("first SaveDocument() error = %v", err)
	}

	// Save again — should upsert
	doc.Title = "Updated Title"
	if err := idx.SaveDocument(doc); err != nil {
		t.Fatalf("second SaveDocument() error = %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 document after upsert; got %d", count)
	}
}

func TestIndex_DocumentExists(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	if idx.DocumentExists(doc.Hash) {
		t.Error("DocumentExists() should return false before saving")
	}

	idx.SaveDocument(doc)

	if !idx.DocumentExists(doc.Hash) {
		t.Error("DocumentExists() should return true after saving")
	}
}

func TestIndex_ListUnindexedSections(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	idx.SaveDocument(doc)

	sections, err := idx.ListUnindexedSections()
	if err != nil {
		t.Fatalf("ListUnindexedSections() error = %v", err)
	}
	if len(sections) != 2 {
		t.Errorf("ListUnindexedSections() returned %d; want 2", len(sections))
	}

	// Update one with an embedding
	embedding := EncodeEmbedding([]float32{1.0, 0.0, 0.0})
	idx.UpdateSectionEmbedding(sections[0].ID, embedding)

	sections, _ = idx.ListUnindexedSections()
	if len(sections) != 1 {
		t.Errorf("after embedding, ListUnindexedSections() returned %d; want 1", len(sections))
	}
}

func TestIndex_ListAllSectionsWithEmbeddings(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	idx.SaveDocument(doc)

	// Initially no sections with embeddings
	sections, err := idx.ListAllSectionsWithEmbeddings()
	if err != nil {
		t.Fatalf("ListAllSectionsWithEmbeddings() error = %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections with embeddings; got %d", len(sections))
	}

	// Add embedding to both sections
	unindexed, _ := idx.ListUnindexedSections()
	for _, s := range unindexed {
		embedding := EncodeEmbedding([]float32{1.0, 0.5, 0.0})
		idx.UpdateSectionEmbedding(s.ID, embedding)
	}

	sections, _ = idx.ListAllSectionsWithEmbeddings()
	if len(sections) != 2 {
		t.Errorf("expected 2 sections with embeddings; got %d", len(sections))
	}
}

func TestIndex_MarkIndexed(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	idx.SaveDocument(doc)

	if err := idx.MarkIndexed(doc.Hash); err != nil {
		t.Fatalf("MarkIndexed() error = %v", err)
	}

	// Verify indexed_at is set
	var indexedAt sql.NullTime
	db.QueryRow("SELECT indexed_at FROM documents WHERE id = ?", doc.Hash).Scan(&indexedAt)
	if !indexedAt.Valid {
		t.Error("MarkIndexed() should set indexed_at")
	}
}

func TestIndex_GetDocument(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	idx.SaveDocument(doc)

	got, err := idx.GetDocument(doc.Hash)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if got.Title != doc.Title {
		t.Errorf("GetDocument().Title = %q; want %q", got.Title, doc.Title)
	}
	if len(got.Sections) != 2 {
		t.Errorf("GetDocument() sections = %d; want 2", len(got.Sections))
	}
}

func TestIndex_GetDocument_NotFound(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)

	_, err := idx.GetDocument("nonexistent")
	if err == nil {
		t.Error("GetDocument() should return error for nonexistent document")
	}
}

func TestIndex_DeleteDocument(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	idx.SaveDocument(doc)

	if err := idx.DeleteDocument(doc.Hash); err != nil {
		t.Fatalf("DeleteDocument() error = %v", err)
	}

	if idx.DocumentExists(doc.Hash) {
		t.Error("document should not exist after delete")
	}

	// Sections should be cascade deleted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM document_sections WHERE document_id = ?", doc.Hash).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sections after cascade delete; got %d", count)
	}
}

func TestIndex_ListDocuments(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)

	// Save two documents
	doc1 := testDocument()
	doc2 := &domain.Document{
		Path:         "docs/other.md",
		Title:        "Other Doc",
		Type:         domain.DocTypePRD,
		Content:      "# Other",
		Hash:         "def456",
		DiscoveredAt: time.Now(),
		Sections: []domain.DocumentSection{
			{Heading: "Other", Level: 1, Content: "Other content."},
		},
	}

	idx.SaveDocument(doc1)
	idx.SaveDocument(doc2)

	docs, err := idx.ListDocuments()
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("ListDocuments() returned %d; want 2", len(docs))
	}
}

func TestIndex_Stats(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	doc := testDocument()

	idx.SaveDocument(doc)
	idx.MarkIndexed(doc.Hash)

	// Embed one section
	sections, _ := idx.ListUnindexedSections()
	if len(sections) > 0 {
		idx.UpdateSectionEmbedding(sections[0].ID, EncodeEmbedding([]float32{1.0}))
	}

	stats, err := idx.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.TotalDocuments != 1 {
		t.Errorf("Stats().TotalDocuments = %d; want 1", stats.TotalDocuments)
	}
	if stats.IndexedDocuments != 1 {
		t.Errorf("Stats().IndexedDocuments = %d; want 1", stats.IndexedDocuments)
	}
	if stats.TotalSections != 2 {
		t.Errorf("Stats().TotalSections = %d; want 2", stats.TotalSections)
	}
	if stats.EmbeddedSections != 1 {
		t.Errorf("Stats().EmbeddedSections = %d; want 1", stats.EmbeddedSections)
	}
}

func TestRetriever_Search(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	embedder := NewKeywordEmbedder(64)
	ctx := context.Background()

	// Save and embed a document
	doc := &domain.Document{
		Path:         "docs/go.md",
		Title:        "Go Programming",
		Type:         domain.DocTypeOther,
		Content:      "# Go\n\nGo is a programming language.",
		Hash:         "go123",
		DiscoveredAt: time.Now(),
		Sections: []domain.DocumentSection{
			{Heading: "Go Programming", Level: 1, Content: "Go is a statically typed compiled language."},
			{Heading: "Concurrency", Level: 2, Content: "Go has goroutines and channels for concurrency."},
			{Heading: "Error Handling", Level: 2, Content: "Go uses explicit error returns instead of exceptions."},
		},
	}
	idx.SaveDocument(doc)

	// Embed all sections
	sections, _ := idx.ListUnindexedSections()
	for _, sec := range sections {
		text := sec.Heading + "\n" + sec.Content
		vec, _ := embedder.Embed(ctx, text)
		idx.UpdateSectionEmbedding(sec.ID, EncodeEmbedding(vec))
	}

	// Search
	retriever := NewRetriever(idx, embedder)
	results, err := retriever.Search(ctx, "goroutines concurrency", 2)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Search() returned 0 results")
	}
	if len(results) > 2 {
		t.Errorf("Search() returned %d results; want ≤2", len(results))
	}

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: score[%d]=%f > score[%d]=%f", i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestRetriever_SearchWithThreshold(t *testing.T) {
	db := openTestDB(t)
	idx := NewIndex(db)
	embedder := NewKeywordEmbedder(64)
	ctx := context.Background()

	doc := &domain.Document{
		Path:    "docs/test.md",
		Title:   "Test",
		Type:    domain.DocTypeOther,
		Content: "# Test",
		Hash:    "test123",
		Sections: []domain.DocumentSection{
			{Heading: "Alpha", Level: 1, Content: "alpha beta gamma"},
			{Heading: "Delta", Level: 1, Content: "completely different content about epsilon zeta"},
		},
		DiscoveredAt: time.Now(),
	}
	idx.SaveDocument(doc)

	sections, _ := idx.ListUnindexedSections()
	for _, sec := range sections {
		vec, _ := embedder.Embed(ctx, sec.Heading+"\n"+sec.Content)
		idx.UpdateSectionEmbedding(sec.ID, EncodeEmbedding(vec))
	}

	retriever := NewRetriever(idx, embedder)

	// High threshold should filter out low-score results
	results, err := retriever.SearchWithThreshold(ctx, "alpha beta", 10, 0.99)
	if err != nil {
		t.Fatalf("SearchWithThreshold() error = %v", err)
	}

	// With a very high threshold, most results should be filtered
	for _, r := range results {
		if r.Score < 0.99 {
			t.Errorf("result score %f below threshold 0.99", r.Score)
		}
	}
}

func TestService_IndexAndSearch(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db, nil) // Uses default KeywordEmbedder

	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.TotalDocuments != 0 {
		t.Errorf("initial Stats().TotalDocuments = %d; want 0", stats.TotalDocuments)
	}

	docs, err := svc.ListDocuments()
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("initial ListDocuments() = %d; want 0", len(docs))
	}
}

func TestService_ReindexAll(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db, nil)

	result, err := svc.ReindexAll(context.Background())
	if err != nil {
		t.Fatalf("ReindexAll() error = %v", err)
	}

	// No sections to reindex
	if result.SectionsEmbedded != 0 {
		t.Errorf("ReindexAll() embedded %d; want 0 for empty index", result.SectionsEmbedded)
	}
}
