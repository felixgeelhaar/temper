-- Document embeddings for external context providers
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

CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path);
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
