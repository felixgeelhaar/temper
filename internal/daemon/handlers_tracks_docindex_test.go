package daemon

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/docindex"
	"github.com/felixgeelhaar/temper/internal/domain"
	sqlitestore "github.com/felixgeelhaar/temper/internal/storage/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func setupTrackServer(t *testing.T) (*serverWithMocks, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "tracks.db")

	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		t.Fatalf("migrate sqlite: %v", err)
	}

	m := newServerWithMocks()
	m.server.trackStore = sqlitestore.NewTrackStore(db)

	cleanup := func() {
		db.Close()
	}

	return m, cleanup
}

func openDocIndexDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

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
		db.Close()
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func setupDocIndexServer(t *testing.T) (*serverWithMocks, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Intro\n\nHello world."), 0644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	db := openDocIndexDB(t)
	service := docindex.NewService(db, nil)

	m := newServerWithMocks()
	m.server.docindexService = service
	m.specs.getWorkspaceRootFn = func() string { return tmpDir }

	return m, func() {}
}

func TestTrackHandlers_CRUD(t *testing.T) {
	m, cleanup := setupTrackServer(t)
	defer cleanup()

	track := domain.Track{
		ID:              "custom-track",
		Name:            "Custom",
		Description:     "Custom track",
		MaxLevel:        domain.L2LocationConcept,
		CooldownSeconds: 10,
	}
	body, _ := json.Marshal(track)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/tracks", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("CreateTrack status = %d; want %d", createRec.Code, http.StatusCreated)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/tracks/custom-track", nil)
	getRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GetTrack status = %d; want %d", getRec.Code, http.StatusOK)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/tracks", nil)
	listRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListTracks status = %d; want %d", listRec.Code, http.StatusOK)
	}

	update := track
	update.Name = "Updated"
	updateBody, _ := json.Marshal(update)
	updateReq := httptest.NewRequest(http.MethodPut, "/v1/tracks/custom-track", bytes.NewReader(updateBody))
	updateRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("UpdateTrack status = %d; want %d", updateRec.Code, http.StatusOK)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/tracks/custom-track", nil)
	deleteRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("DeleteTrack status = %d; want %d", deleteRec.Code, http.StatusOK)
	}
}

func TestTrackHandlers_ExportImport(t *testing.T) {
	m, cleanup := setupTrackServer(t)
	defer cleanup()

	track := domain.Track{
		ID:              "export-track",
		Name:            "Export",
		Description:     "Export",
		MaxLevel:        domain.L2LocationConcept,
		CooldownSeconds: 10,
	}
	createBody, _ := json.Marshal(track)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/tracks", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(createRec, createReq)

	exportBody, _ := json.Marshal(map[string]string{"id": "export-track"})
	exportReq := httptest.NewRequest(http.MethodPost, "/v1/tracks/export", bytes.NewReader(exportBody))
	exportRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusOK {
		t.Fatalf("ExportTrack status = %d; want %d", exportRec.Code, http.StatusOK)
	}
	if ct := exportRec.Header().Get("Content-Type"); ct == "" {
		t.Error("ExportTrack should set Content-Type")
	}

	importReq := httptest.NewRequest(http.MethodPost, "/v1/tracks/import", bytes.NewReader(exportRec.Body.Bytes()))
	importRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(importRec, importReq)

	if importRec.Code != http.StatusCreated {
		t.Fatalf("ImportTrack status = %d; want %d", importRec.Code, http.StatusCreated)
	}
}

func TestTrackHandlers_Errors(t *testing.T) {
	m, cleanup := setupTrackServer(t)
	defer cleanup()

	badReq := httptest.NewRequest(http.MethodPost, "/v1/tracks", bytes.NewReader([]byte("{invalid")))
	badRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("CreateTrack invalid JSON status = %d; want %d", badRec.Code, http.StatusBadRequest)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/v1/tracks", bytes.NewReader([]byte(`{}`)))
	missingRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("CreateTrack invalid track status = %d; want %d", missingRec.Code, http.StatusBadRequest)
	}

	track := domain.Track{ID: "dup", Name: "Dup", MaxLevel: domain.L1CategoryHint}
	body, _ := json.Marshal(track)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/tracks", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(createRec, createReq)

	dupReq := httptest.NewRequest(http.MethodPost, "/v1/tracks", bytes.NewReader(body))
	dupRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusConflict {
		t.Fatalf("CreateTrack conflict status = %d; want %d", dupRec.Code, http.StatusConflict)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/tracks/missing", nil)
	getRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("GetTrack missing status = %d; want %d", getRec.Code, http.StatusNotFound)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/v1/tracks/missing", bytes.NewReader([]byte(`{"name":"x"}`)))
	updateRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNotFound {
		t.Fatalf("UpdateTrack missing status = %d; want %d", updateRec.Code, http.StatusNotFound)
	}

	updateBadReq := httptest.NewRequest(http.MethodPut, "/v1/tracks/dup", bytes.NewReader([]byte("{invalid")))
	updateBadRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(updateBadRec, updateBadReq)
	if updateBadRec.Code != http.StatusBadRequest {
		t.Fatalf("UpdateTrack invalid body status = %d; want %d", updateBadRec.Code, http.StatusBadRequest)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/tracks/missing", nil)
	deleteRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNotFound {
		t.Fatalf("DeleteTrack missing status = %d; want %d", deleteRec.Code, http.StatusNotFound)
	}

	exportReq := httptest.NewRequest(http.MethodPost, "/v1/tracks/export", bytes.NewReader([]byte(`{"id":"missing"}`)))
	exportRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusNotFound {
		t.Fatalf("ExportTrack missing status = %d; want %d", exportRec.Code, http.StatusNotFound)
	}

	importReq := httptest.NewRequest(http.MethodPost, "/v1/tracks/import", bytes.NewReader([]byte("invalid: [yaml")))
	importRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusBadRequest {
		t.Fatalf("ImportTrack invalid status = %d; want %d", importRec.Code, http.StatusBadRequest)
	}
}

func TestDocIndexHandlers(t *testing.T) {
	m, _ := setupDocIndexServer(t)

	indexReq := httptest.NewRequest(http.MethodPost, "/v1/docindex/index", bytes.NewReader([]byte(`{}`)))
	indexRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(indexRec, indexReq)
	if indexRec.Code != http.StatusOK {
		t.Fatalf("DocIndex index status = %d; want %d", indexRec.Code, http.StatusOK)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/v1/docindex/status", nil)
	statusRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("DocIndex status = %d; want %d", statusRec.Code, http.StatusOK)
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/v1/docindex/search", bytes.NewReader([]byte(`{"query":"hello","top_k":3}`)))
	searchRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("DocIndex search status = %d; want %d", searchRec.Code, http.StatusOK)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/docindex/documents", nil)
	listRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("DocIndex list status = %d; want %d", listRec.Code, http.StatusOK)
	}

	reindexReq := httptest.NewRequest(http.MethodPost, "/v1/docindex/reindex", nil)
	reindexRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(reindexRec, reindexReq)
	if reindexRec.Code != http.StatusOK {
		t.Fatalf("DocIndex reindex status = %d; want %d", reindexRec.Code, http.StatusOK)
	}
}

func TestDocIndexHandlers_Errors(t *testing.T) {
	m, _ := setupDocIndexServer(t)

	badReq := httptest.NewRequest(http.MethodPost, "/v1/docindex/index", bytes.NewReader([]byte("{invalid")))
	badRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("DocIndex index invalid status = %d; want %d", badRec.Code, http.StatusBadRequest)
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/v1/docindex/search", bytes.NewReader([]byte(`{"query":""}`)))
	searchRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusBadRequest {
		t.Fatalf("DocIndex search missing query status = %d; want %d", searchRec.Code, http.StatusBadRequest)
	}

	m.server.docindexService = nil
	statusReq := httptest.NewRequest(http.MethodGet, "/v1/docindex/status", nil)
	statusRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("DocIndex status missing service = %d; want %d", statusRec.Code, http.StatusServiceUnavailable)
	}
}
