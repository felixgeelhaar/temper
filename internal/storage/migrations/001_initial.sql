-- 001_initial.sql: Core schema for Temper SQLite storage
-- Replaces JSON file storage with structured relational tables.

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    exercise_id TEXT NOT NULL DEFAULT '',
    intent      TEXT NOT NULL DEFAULT 'training',
    spec_path   TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    code        TEXT NOT NULL DEFAULT '{}',            -- JSON map[string]string
    policy      TEXT NOT NULL DEFAULT '{}',            -- JSON LearningPolicy

    -- Authoring fields
    authoring_docs    TEXT NOT NULL DEFAULT '[]',      -- JSON []string
    authoring_section TEXT NOT NULL DEFAULT '',

    -- Statistics
    run_count            INTEGER NOT NULL DEFAULT 0,
    hint_count           INTEGER NOT NULL DEFAULT 0,
    last_run_at          DATETIME,
    last_intervention_at DATETIME,

    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_intent ON sessions(intent);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);

CREATE TABLE IF NOT EXISTS runs (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    code       TEXT NOT NULL DEFAULT '{}',             -- JSON map[string]string
    result     TEXT,                                    -- JSON RunResult (nullable)
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_runs_session_id ON runs(session_id);
CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at);

CREATE TABLE IF NOT EXISTS interventions (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id     TEXT,
    intent     TEXT NOT NULL DEFAULT '',
    level      INTEGER NOT NULL DEFAULT 0,
    type       TEXT NOT NULL DEFAULT '',
    content    TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_interventions_session_id ON interventions(session_id);
CREATE INDEX IF NOT EXISTS idx_interventions_created_at ON interventions(created_at);

CREATE TABLE IF NOT EXISTS profiles (
    id                    TEXT PRIMARY KEY,
    topic_skills          TEXT NOT NULL DEFAULT '{}',    -- JSON map[string]StoredSkill
    total_exercises       INTEGER NOT NULL DEFAULT 0,
    total_sessions        INTEGER NOT NULL DEFAULT 0,
    completed_sessions    INTEGER NOT NULL DEFAULT 0,
    total_runs            INTEGER NOT NULL DEFAULT 0,
    hint_requests         INTEGER NOT NULL DEFAULT 0,
    avg_time_to_green_ms  INTEGER NOT NULL DEFAULT 0,
    exercise_history      TEXT NOT NULL DEFAULT '[]',    -- JSON []ExerciseAttempt
    error_patterns        TEXT NOT NULL DEFAULT '{}',    -- JSON map[string]int
    hint_dependency_trend TEXT NOT NULL DEFAULT '[]',    -- JSON []HintDependencyPoint
    created_at            DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at            DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS analytics_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    session_id TEXT,
    data       TEXT NOT NULL DEFAULT '{}',               -- JSON payload
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_type ON analytics_events(event_type);
CREATE INDEX IF NOT EXISTS idx_analytics_events_session ON analytics_events(session_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_created_at ON analytics_events(created_at);

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO schema_migrations (version) VALUES (1);
