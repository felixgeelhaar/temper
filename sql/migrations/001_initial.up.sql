-- Initial database schema for Temper MVP

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

-- Sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Artifacts (user workspaces)
CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id VARCHAR(255), -- nullable for freeform workspaces
    name VARCHAR(255) NOT NULL,
    content JSONB NOT NULL DEFAULT '{}', -- {filename: content}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifacts_user_id ON artifacts(user_id);
CREATE INDEX idx_artifacts_exercise_id ON artifacts(exercise_id);

-- Artifact versions (for undo/history)
CREATE TABLE artifact_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    version INT NOT NULL,
    content JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(artifact_id, version)
);

CREATE INDEX idx_artifact_versions_artifact_id ON artifact_versions(artifact_id);

-- Runs (code execution)
CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    recipe JSONB NOT NULL,
    output JSONB, -- RunOutput as JSON
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_runs_artifact_id ON runs(artifact_id);
CREATE INDEX idx_runs_user_id ON runs(user_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_runs_created_at ON runs(created_at);

-- Pairing sessions
CREATE TABLE pairing_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    artifact_id UUID REFERENCES artifacts(id) ON DELETE SET NULL,
    exercise_id VARCHAR(255),
    policy JSONB NOT NULL, -- LearningPolicy as JSON
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ
);

CREATE INDEX idx_pairing_sessions_user_id ON pairing_sessions(user_id);
CREATE INDEX idx_pairing_sessions_artifact_id ON pairing_sessions(artifact_id);

-- Interventions
CREATE TABLE interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES pairing_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    run_id UUID REFERENCES runs(id) ON DELETE SET NULL,
    intent VARCHAR(50) NOT NULL,
    level INT NOT NULL,
    type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    targets JSONB, -- [{file, startLine, endLine}]
    rationale TEXT,
    requested_at TIMESTAMPTZ NOT NULL,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_interventions_session_id ON interventions(session_id);
CREATE INDEX idx_interventions_user_id ON interventions(user_id);
CREATE INDEX idx_interventions_delivered_at ON interventions(delivered_at);

-- Learning profiles
CREATE TABLE learning_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_skills JSONB NOT NULL DEFAULT '{}',
    total_exercises INT NOT NULL DEFAULT 0,
    total_runs INT NOT NULL DEFAULT 0,
    hint_requests INT NOT NULL DEFAULT 0,
    avg_time_to_green_ms BIGINT,
    common_errors TEXT[],
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_learning_profiles_user_id ON learning_profiles(user_id);

-- Events (telemetry)
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_event_type ON events(event_type);
CREATE INDEX idx_events_user_id ON events(user_id);
CREATE INDEX idx_events_created_at ON events(created_at);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_artifacts_updated_at
    BEFORE UPDATE ON artifacts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_learning_profiles_updated_at
    BEFORE UPDATE ON learning_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
