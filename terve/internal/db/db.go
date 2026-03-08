package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB connection to the SQLite database.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at path and runs schema migrations.
func Open(path string) (*DB, error) {
	// Ensure the parent directory exists.
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// migrate runs CREATE TABLE IF NOT EXISTS statements.
func (db *DB) migrate() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			provider    TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			name        TEXT NOT NULL,
			email       TEXT NOT NULL DEFAULT '',
			avatar_url  TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider, provider_id)
		)
	`)
	return err
}
