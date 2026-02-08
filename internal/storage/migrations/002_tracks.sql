-- 002_tracks.sql: Learning contract track presets

CREATE TABLE IF NOT EXISTS tracks (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    preset          TEXT NOT NULL DEFAULT 'custom',
    max_level       INTEGER NOT NULL DEFAULT 3,
    cooldown_seconds INTEGER NOT NULL DEFAULT 60,
    patching_enabled INTEGER NOT NULL DEFAULT 0,
    auto_progress   TEXT NOT NULL DEFAULT '{}',   -- JSON AutoProgressRules
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tracks_preset ON tracks(preset);

-- Seed built-in tracks
INSERT OR IGNORE INTO tracks (id, name, description, preset, max_level, cooldown_seconds, patching_enabled, auto_progress) VALUES
    ('beginner', 'Beginner', 'Maximum guidance. Generous hints and code snippets with short cooldowns.', 'beginner', 4, 30, 1, '{"enabled":true,"promote_after_streak":5,"demote_after_failures":3,"min_skill_for_promote":0.3}'),
    ('standard', 'Standard', 'Balanced learning. Hints up to constrained snippets with moderate cooldowns.', 'standard', 3, 60, 0, '{"enabled":true,"promote_after_streak":7,"demote_after_failures":5,"min_skill_for_promote":0.5}'),
    ('advanced', 'Advanced', 'Minimal guidance. Only clarifying questions and category hints with long cooldowns.', 'advanced', 1, 120, 0, '{"enabled":false}'),
    ('interview-prep', 'Interview Prep', 'Strict mode for interview practice. Location hints only, long cooldowns.', 'interview-prep', 2, 120, 0, '{"enabled":false}');

INSERT OR IGNORE INTO schema_migrations (version) VALUES (2);
