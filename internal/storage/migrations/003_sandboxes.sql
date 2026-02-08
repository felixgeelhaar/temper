-- 003_sandboxes.sql: Persistent sandbox containers

CREATE TABLE IF NOT EXISTS sandboxes (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    container_id    TEXT NOT NULL DEFAULT '',
    language        TEXT NOT NULL DEFAULT 'go',
    image           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'creating',  -- creating, ready, running, paused, destroyed
    memory_mb       INTEGER NOT NULL DEFAULT 256,
    cpu_limit       REAL NOT NULL DEFAULT 0.5,
    network_off     INTEGER NOT NULL DEFAULT 1,
    last_exec_at    DATETIME,
    expires_at      DATETIME NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sandboxes_session ON sandboxes(session_id);
CREATE INDEX IF NOT EXISTS idx_sandboxes_status ON sandboxes(status);
CREATE INDEX IF NOT EXISTS idx_sandboxes_expires ON sandboxes(expires_at);

INSERT OR IGNORE INTO schema_migrations (version) VALUES (3);
