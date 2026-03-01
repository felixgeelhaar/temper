package docindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func TestService_IndexDirectory_SkipsDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("create docs dir: %v", err)
	}

	content := "# Title\n\nSome content."
	writeTestFile(t, filepath.Join(docsDir, "alpha.md"), content)
	writeTestFile(t, filepath.Join(docsDir, "beta.md"), content)

	db := openTestDB(t)
	svc := NewService(db, NewKeywordEmbedder(8))

	result, err := svc.IndexDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory() error = %v", err)
	}

	if result.DocumentsFound != 2 {
		t.Errorf("DocumentsFound = %d; want 2", result.DocumentsFound)
	}
	if result.DocumentsIndexed != 1 {
		t.Errorf("DocumentsIndexed = %d; want 1", result.DocumentsIndexed)
	}
	if result.DocumentsSkipped != 1 {
		t.Errorf("DocumentsSkipped = %d; want 1", result.DocumentsSkipped)
	}
	if result.SectionsEmbedded == 0 {
		t.Error("SectionsEmbedded should be > 0")
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d; want 0", result.Errors)
	}

	docs, err := svc.ListDocuments()
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("ListDocuments() length = %d; want 1", len(docs))
	}
}

func TestService_ReindexAll_Embeds(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db, NewKeywordEmbedder(8))

	doc := &domain.Document{
		Path:         "docs/reindex.md",
		Title:        "Reindex",
		Type:         domain.DocTypeOther,
		Content:      "# Reindex\n\nSection content.",
		DiscoveredAt: time.Now(),
		Sections: []domain.DocumentSection{
			{Heading: "Reindex", Level: 1, Content: "Section content."},
			{Heading: "Details", Level: 2, Content: "More detail."},
		},
	}
	doc.Hash = doc.ComputeHash()

	if err := svc.index.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument() error = %v", err)
	}

	result, err := svc.ReindexAll(context.Background())
	if err != nil {
		t.Fatalf("ReindexAll() error = %v", err)
	}

	if result.SectionsEmbedded != len(doc.Sections) {
		t.Errorf("SectionsEmbedded = %d; want %d", result.SectionsEmbedded, len(doc.Sections))
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d; want 0", result.Errors)
	}

	remaining, err := svc.index.ListUnindexedSections()
	if err != nil {
		t.Fatalf("ListUnindexedSections() error = %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("unindexed sections = %d; want 0", len(remaining))
	}
}
