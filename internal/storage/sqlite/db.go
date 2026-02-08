package sqlite

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/felixgeelhaar/temper/internal/storage/migrations"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a sql.DB connection to a SQLite database with migration support.
type DB struct {
	*sql.DB
}

// Open creates a new SQLite connection with WAL mode and foreign keys enabled.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Verify connectivity
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Set connection pool for single-writer SQLite
	db.SetMaxOpenConns(1)

	return &DB{DB: db}, nil
}

// Migrate applies all pending SQL migrations from the embedded filesystem.
func (db *DB) Migrate() error {
	// Ensure schema_migrations table exists (bootstrap)
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	// List migration files
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort and filter SQL files
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	applied := 0
	for _, name := range files {
		// Extract version number from filename (e.g., "001_initial.sql" -> 1)
		version, err := parseVersion(name)
		if err != nil {
			slog.Warn("skipping non-migration file", "name", name, "error", err)
			continue
		}

		if version <= currentVersion {
			continue
		}

		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %s: %w", name, err)
		}

		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT OR REPLACE INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}

		applied++
		slog.Info("applied migration", "name", name, "version", version)
	}

	if applied > 0 {
		slog.Info("migrations complete", "applied", applied)
	}

	return nil
}

// Version returns the current schema version.
func (db *DB) Version() (int, error) {
	var version int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}

// parseVersion extracts the version number from a migration filename like "001_initial.sql".
func parseVersion(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid migration filename: %s", name)
	}
	var version int
	_, err := fmt.Sscanf(parts[0], "%d", &version)
	if err != nil {
		return 0, fmt.Errorf("parse version from %s: %w", name, err)
	}
	return version, nil
}
