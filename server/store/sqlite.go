package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// migration is the full DDL to create the ci schema.
const migration = `
CREATE TABLE IF NOT EXISTS runs (
	id          TEXT PRIMARY KEY,
	created_at  TEXT NOT NULL,
	finished_at TEXT,
	status      TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS run_services (
	run_id       TEXT NOT NULL REFERENCES runs(id),
	service_name TEXT NOT NULL,
	suite        TEXT NOT NULL,
	status       TEXT NOT NULL DEFAULT 'pending',
	exit_code    INTEGER,
	start_time   TEXT,
	end_time     TEXT,
	PRIMARY KEY (run_id, service_name)
);

CREATE TABLE IF NOT EXISTS log_entries (
	run_id       TEXT NOT NULL,
	service_name TEXT NOT NULL,
	line_number  INTEGER NOT NULL,
	timestamp    TEXT NOT NULL,
	level        TEXT NOT NULL,
	content      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_log_entries_run_service
	ON log_entries (run_id, service_name);

CREATE INDEX IF NOT EXISTS idx_log_entries_level
	ON log_entries (level);
`

// Store wraps a database connection.
type Store struct {
	db *sql.DB
}

// Open opens the SQLite database at the given path and returns a Store.
// For ":memory:", an in-memory database is created and the schema is applied.
// For file paths, non-existent parent directories are created automatically.
func Open(path string) (*Store, error) {
	// Create parent directories for file-backed databases to avoid silent
	// fallback to in-memory when the directory doesn't exist.
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db dir %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Pin to one connection so in-memory databases don't spawn a new
	// private DB per connection. This avoids "no such table" errors
	// when queries land on uninitialized connections.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(migration); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

// DB returns the underlying database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
