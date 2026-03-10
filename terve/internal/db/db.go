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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			title        TEXT NOT NULL,
			author       TEXT NOT NULL,
			description  TEXT NOT NULL DEFAULT '',
			gutenberg_id INTEGER,
			source       TEXT NOT NULL DEFAULT 'seed',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(gutenberg_id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS book_chapters (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id        INTEGER NOT NULL REFERENCES books(id),
			chapter_number INTEGER NOT NULL,
			title          TEXT NOT NULL DEFAULT '',
			content        TEXT NOT NULL,
			UNIQUE(book_id, chapter_number)
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_bookmarks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id),
			book_id    INTEGER NOT NULL REFERENCES books(id),
			chapter_id INTEGER NOT NULL REFERENCES book_chapters(id),
			paragraph  INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, book_id)
		)
	`)
	if err != nil {
		return err
	}

	// Migration for existing DBs: add paragraph column if missing.
	db.Exec(`ALTER TABLE user_bookmarks ADD COLUMN paragraph INTEGER NOT NULL DEFAULT 0`)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS quiz_results (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id),
			quiz_type  TEXT NOT NULL,
			total      INTEGER NOT NULL,
			correct    INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS paradigms (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			lemma      TEXT NOT NULL,
			word_class TEXT NOT NULL,
			tense      TEXT NOT NULL DEFAULT '',
			forms_json TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(lemma, word_class, tense)
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sentence_cache (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			lemma       TEXT NOT NULL,
			finnish     TEXT NOT NULL,
			english     TEXT NOT NULL,
			target_form TEXT NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sentence_cache_lemma ON sentence_cache(lemma)`)
	if err != nil {
		return err
	}

	if err := db.seedCards(); err != nil {
		return err
	}
	return db.seedBooks()
}
