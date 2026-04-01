package db

import (
	"database/sql"
	"log"
	"time"
)

// Book represents a row in the books table.
type Book struct {
	ID           int64
	Title        string
	Author       string
	Description  string
	GutenbergID  *int
	Source       string
	Difficulty   string
	CreatedAt    time.Time
	ChapterCount int // populated by list queries
}

// BookChapter represents a row in the book_chapters table.
type BookChapter struct {
	ID            int64
	BookID        int64
	ChapterNumber int
	Title         string
	Content       string
}

// UserBookmark represents a row in the user_bookmarks table.
type UserBookmark struct {
	ID        int64
	UserID    int64
	BookID    int64
	ChapterID int64
	Paragraph int
	UpdatedAt time.Time
}

// InsertBook inserts a book. If a gutenberg_id conflict exists, returns the existing ID.
func (db *DB) InsertBook(title, author, description string, gutenbergID *int, source string) (int64, error) {
	var res sql.Result
	var err error
	if gutenbergID != nil {
		res, err = db.Exec(`
			INSERT INTO books (title, author, description, gutenberg_id, source)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(gutenberg_id) DO NOTHING
		`, title, author, description, *gutenbergID, source)
	} else {
		res, err = db.Exec(`
			INSERT INTO books (title, author, description, source)
			VALUES (?, ?, ?, ?)
		`, title, author, description, source)
	}
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 && gutenbergID != nil {
		err = db.QueryRow(`SELECT id FROM books WHERE gutenberg_id = ?`, *gutenbergID).Scan(&id)
	}
	return id, err
}

// InsertChapter inserts a book chapter (idempotent via UNIQUE constraint).
func (db *DB) InsertChapter(bookID int64, chapterNumber int, title, content string) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO book_chapters (book_id, chapter_number, title, content)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(book_id, chapter_number) DO NOTHING
	`, bookID, chapterNumber, title, content)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		err = db.QueryRow(`SELECT id FROM book_chapters WHERE book_id = ? AND chapter_number = ?`, bookID, chapterNumber).Scan(&id)
	}
	return id, err
}

// ListBooks returns all books with chapter counts.
func (db *DB) ListBooks() ([]Book, error) {
	rows, err := db.Query(`
		SELECT b.id, b.title, b.author, b.description, b.gutenberg_id, b.source, b.difficulty, b.created_at,
		       (SELECT COUNT(*) FROM book_chapters bc WHERE bc.book_id = b.id) AS chapter_count
		FROM books b
		ORDER BY b.title ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := []Book{}
	for rows.Next() {
		var b Book
		var gid sql.NullInt64
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Description, &gid, &b.Source, &b.Difficulty, &b.CreatedAt, &b.ChapterCount); err != nil {
			return nil, err
		}
		if gid.Valid {
			v := int(gid.Int64)
			b.GutenbergID = &v
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

// UpdateBookDifficulty sets the CEFR difficulty level for a book.
func (db *DB) UpdateBookDifficulty(bookID int64, difficulty string) error {
	_, err := db.Exec(`UPDATE books SET difficulty = ? WHERE id = ?`, difficulty, bookID)
	return err
}

// GetBook returns a single book by ID.
func (db *DB) GetBook(bookID int64) (*Book, error) {
	var b Book
	var gid sql.NullInt64
	err := db.QueryRow(`
		SELECT b.id, b.title, b.author, b.description, b.gutenberg_id, b.source, b.difficulty, b.created_at,
		       (SELECT COUNT(*) FROM book_chapters bc WHERE bc.book_id = b.id) AS chapter_count
		FROM books b WHERE b.id = ?
	`, bookID).Scan(&b.ID, &b.Title, &b.Author, &b.Description, &gid, &b.Source, &b.Difficulty, &b.CreatedAt, &b.ChapterCount)
	if err != nil {
		return nil, err
	}
	if gid.Valid {
		v := int(gid.Int64)
		b.GutenbergID = &v
	}
	return &b, nil
}

// GetChapter returns a chapter by book ID and chapter number.
func (db *DB) GetChapter(bookID int64, chapterNumber int) (*BookChapter, error) {
	var ch BookChapter
	err := db.QueryRow(`
		SELECT id, book_id, chapter_number, title, content
		FROM book_chapters WHERE book_id = ? AND chapter_number = ?
	`, bookID, chapterNumber).Scan(&ch.ID, &ch.BookID, &ch.ChapterNumber, &ch.Title, &ch.Content)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

// GetChapters returns all chapters for a book, ordered by number.
func (db *DB) GetChapters(bookID int64) ([]BookChapter, error) {
	rows, err := db.Query(`
		SELECT id, book_id, chapter_number, title, content
		FROM book_chapters WHERE book_id = ? ORDER BY chapter_number ASC
	`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chapters := []BookChapter{}
	for rows.Next() {
		var ch BookChapter
		if err := rows.Scan(&ch.ID, &ch.BookID, &ch.ChapterNumber, &ch.Title, &ch.Content); err != nil {
			return nil, err
		}
		chapters = append(chapters, ch)
	}
	return chapters, rows.Err()
}

// SaveBookmark saves or updates a user's bookmark for a book.
func (db *DB) SaveBookmark(userID, bookID, chapterID int64, paragraph int) error {
	_, err := db.Exec(`
		INSERT INTO user_bookmarks (user_id, book_id, chapter_id, paragraph)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, book_id) DO UPDATE SET
			chapter_id = excluded.chapter_id,
			paragraph  = excluded.paragraph,
			updated_at = CURRENT_TIMESTAMP
	`, userID, bookID, chapterID, paragraph)
	return err
}

// GetBookmark returns the bookmarked chapter ID and paragraph for a user+book, or (0, 0) if none.
func (db *DB) GetBookmark(userID, bookID int64) (int64, int) {
	var chapterID int64
	var paragraph int
	err := db.QueryRow(`SELECT chapter_id, paragraph FROM user_bookmarks WHERE user_id = ? AND book_id = ?`, userID, bookID).Scan(&chapterID, &paragraph)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("get bookmark (user=%d, book=%d): %v", userID, bookID, err)
	}
	return chapterID, paragraph
}

// GetUserBookmarks returns a map of bookID→chapterID for the given user.
func (db *DB) GetUserBookmarks(userID int64) map[int64]int64 {
	result := make(map[int64]int64)
	rows, err := db.Query(`SELECT book_id, chapter_id FROM user_bookmarks WHERE user_id = ?`, userID)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var bookID, chapterID int64
		if err := rows.Scan(&bookID, &chapterID); err == nil {
			result[bookID] = chapterID
		}
	}
	return result
}

// BookExistsByGutenbergID checks if a book with the given Gutenberg ID already exists.
func (db *DB) BookExistsByGutenbergID(gutenbergID int) bool {
	var exists int
	err := db.QueryRow(`SELECT 1 FROM books WHERE gutenberg_id = ?`, gutenbergID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("check book exists (gutenberg_id=%d): %v", gutenbergID, err)
	}
	return exists == 1
}
