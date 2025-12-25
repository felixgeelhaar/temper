-- Rollback initial schema

DROP TRIGGER IF EXISTS update_learning_profiles_updated_at ON learning_profiles;
DROP TRIGGER IF EXISTS update_artifacts_updated_at ON artifacts;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS learning_profiles;
DROP TABLE IF EXISTS interventions;
DROP TABLE IF EXISTS pairing_sessions;
DROP TABLE IF EXISTS runs;
DROP TABLE IF EXISTS artifact_versions;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
