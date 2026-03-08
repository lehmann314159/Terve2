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
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cards (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			finnish      TEXT NOT NULL,
			lemma        TEXT NOT NULL,
			word_class   TEXT NOT NULL DEFAULT '',
			morphology   TEXT NOT NULL DEFAULT '',
			translation  TEXT NOT NULL DEFAULT '',
			explanation  TEXT NOT NULL DEFAULT '',
			context      TEXT NOT NULL DEFAULT '',
			source       TEXT NOT NULL DEFAULT 'user',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(lemma, finnish)
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_cards (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id       INTEGER NOT NULL REFERENCES users(id),
			card_id       INTEGER NOT NULL REFERENCES cards(id),
			focused       BOOLEAN NOT NULL DEFAULT 0,
			ease_factor   REAL    NOT NULL DEFAULT 2.5,
			interval_days INTEGER NOT NULL DEFAULT 0,
			repetitions   INTEGER NOT NULL DEFAULT 0,
			next_review   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_review   DATETIME,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, card_id)
		)
	`)
	if err != nil {
		return err
	}

	return db.seedCards()
}
