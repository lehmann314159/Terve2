package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/db"
	"github.com/lehmann314159/terve2/internal/gutenberg"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// BooksPageData is passed to the books list page.
type BooksPageData struct {
	PageData
	Books     []db.Book
	Bookmarks map[int64]int64 // bookID → chapterID
}

// BookReaderData is passed to the book reader page.
type BookReaderData struct {
	PageData
	Book           *db.Book
	Chapters       []db.BookChapter
	CurrentChapter *db.BookChapter
	ChapterNumber  int
	Tokens         []voikko.TokenAnalysis
	PlainText      string
	Bookmark       int64 // bookmarked chapter_id, 0 if none
}

// BooksPage renders the full book list page.
func (h *Handlers) BooksPage(w http.ResponseWriter, r *http.Request) {
	books, err := h.db.ListBooks()
	if err != nil {
		log.Printf("list books: %v", err)
		http.Error(w, "Failed to load books", http.StatusInternalServerError)
		return
	}

	sess := auth.GetSession(r.Context())
	bookmarks := make(map[int64]int64)
	if sess != nil {
		bookmarks = h.db.GetUserBookmarks(sess.DBUserID)
	}

	h.render(w, "base", BooksPageData{
		PageData:  pageData(r, "Terve — Books", "books"),
		Books:     books,
		Bookmarks: bookmarks,
	})
}

// BookReader renders the book reader page with the first (or bookmarked) chapter.
func (h *Handlers) BookReader(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.ParseInt(chi.URLParam(r, "bookID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	book, err := h.db.GetBook(bookID)
	if err != nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	chapters, err := h.db.GetChapters(bookID)
	if err != nil {
		log.Printf("get chapters: %v", err)
		http.Error(w, "Failed to load chapters", http.StatusInternalServerError)
		return
	}

	if len(chapters) == 0 {
		http.Error(w, "Book has no chapters", http.StatusNotFound)
		return
	}

	// Determine starting chapter: bookmark or first
	chapterNum := 1
	sess := auth.GetSession(r.Context())
	var bookmark int64
	if sess != nil {
		bookmark = h.db.GetBookmark(sess.DBUserID, bookID)
		if bookmark != 0 {
			// Find chapter number from chapter ID
			for _, ch := range chapters {
				if ch.ID == bookmark {
					chapterNum = ch.ChapterNumber
					break
				}
			}
		}
	}

	// Find the chapter
	var currentChapter *db.BookChapter
	for i := range chapters {
		if chapters[i].ChapterNumber == chapterNum {
			currentChapter = &chapters[i]
			break
		}
	}
	if currentChapter == nil {
		currentChapter = &chapters[0]
		chapterNum = currentChapter.ChapterNumber
	}

	// Tokenize chapter text
	tokens, plainText := h.tokenizeText(currentChapter.Content)

	h.render(w, "base", BookReaderData{
		PageData:       pageData(r, fmt.Sprintf("Terve — %s", book.Title), "book-reader"),
		Book:           book,
		Chapters:       chapters,
		CurrentChapter: currentChapter,
		ChapterNumber:  chapterNum,
		Tokens:         tokens,
		PlainText:      plainText,
		Bookmark:       bookmark,
	})
}

// BookChapter loads a chapter. Serves a partial for HTMX requests or a full page for direct navigation.
func (h *Handlers) BookChapter(w http.ResponseWriter, r *http.Request) {
	bookID, err := strconv.ParseInt(chi.URLParam(r, "bookID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	chapterNum, err := strconv.Atoi(chi.URLParam(r, "num"))
	if err != nil {
		http.Error(w, "Invalid chapter number", http.StatusBadRequest)
		return
	}

	chapter, err := h.db.GetChapter(bookID, chapterNum)
	if err != nil {
		http.Error(w, "Chapter not found", http.StatusNotFound)
		return
	}

	// Auto-save bookmark if logged in
	sess := auth.GetSession(r.Context())
	if sess != nil {
		if err := h.db.SaveBookmark(sess.DBUserID, bookID, chapter.ID); err != nil {
			log.Printf("save bookmark: %v", err)
		}
	}

	book, err := h.db.GetBook(bookID)
	if err != nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	tokens, plainText := h.tokenizeText(chapter.Content)

	// Direct browser request: render full reader page
	if r.Header.Get("HX-Request") == "" {
		chapters, err := h.db.GetChapters(bookID)
		if err != nil {
			log.Printf("get chapters: %v", err)
			http.Error(w, "Failed to load chapters", http.StatusInternalServerError)
			return
		}
		var bookmark int64
		if sess != nil {
			bookmark = h.db.GetBookmark(sess.DBUserID, bookID)
		}
		h.render(w, "base", BookReaderData{
			PageData:       pageData(r, fmt.Sprintf("Terve — %s", book.Title), "book-reader"),
			Book:           book,
			Chapters:       chapters,
			CurrentChapter: chapter,
			ChapterNumber:  chapterNum,
			Tokens:         tokens,
			PlainText:      plainText,
			Bookmark:       bookmark,
		})
		return
	}

	// HTMX request: render partial
	h.renderPartial(w, "book-chapter", map[string]any{
		"Chapter":       chapter,
		"ChapterNumber": chapterNum,
		"TotalChapters": book.ChapterCount,
		"BookID":        bookID,
		"Tokens":        tokens,
		"PlainText":     plainText,
	})
}

// SaveBookmark saves a bookmark (called via HTMX POST).
func (h *Handlers) SaveBookmark(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	bookID, err := strconv.ParseInt(chi.URLParam(r, "bookID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	chapterIDStr := r.FormValue("chapter_id")
	chapterID, err := strconv.ParseInt(chapterIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid chapter ID", http.StatusBadRequest)
		return
	}

	if err := h.db.SaveBookmark(sess.DBUserID, bookID, chapterID); err != nil {
		log.Printf("save bookmark: %v", err)
		http.Error(w, "Failed to save bookmark", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SearchGutenberg returns Gutendex search results as an HTMX partial.
func (h *Handlers) SearchGutenberg(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.renderPartial(w, "gutenberg-results", map[string]any{"Error": "Please enter a search query."})
		return
	}

	results, err := gutenberg.Search(query)
	if err != nil {
		log.Printf("gutenberg search: %v", err)
		h.renderPartial(w, "gutenberg-results", map[string]any{"Error": "Search failed. Please try again."})
		return
	}

	h.renderPartial(w, "gutenberg-results", map[string]any{
		"Results": results,
		"Query":   query,
	})
}

// ImportBook downloads and imports a Gutenberg book.
func (h *Handlers) ImportBook(w http.ResponseWriter, r *http.Request) {
	gutenbergIDStr := r.FormValue("gutenberg_id")
	gutenbergID, err := strconv.Atoi(gutenbergIDStr)
	if err != nil {
		h.renderPartial(w, "import-result", map[string]any{"Error": "Invalid Gutenberg ID."})
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")

	// Check if already imported
	if h.db.BookExistsByGutenbergID(gutenbergID) {
		h.renderPartial(w, "import-result", map[string]any{"Error": "This book has already been imported."})
		return
	}

	// Download
	text, err := gutenberg.Download(gutenbergID)
	if err != nil {
		log.Printf("gutenberg download %d: %v", gutenbergID, err)
		h.renderPartial(w, "import-result", map[string]any{"Error": "Failed to download book."})
		return
	}

	text = gutenberg.StripBoilerplate(text)
	chapters := gutenberg.SplitChapters(text)

	bookID, err := h.db.InsertBook(title, author, "", &gutenbergID, "gutenberg")
	if err != nil {
		log.Printf("insert imported book: %v", err)
		h.renderPartial(w, "import-result", map[string]any{"Error": "Failed to save book."})
		return
	}

	var failedChapters int
	for _, ch := range chapters {
		if _, err := h.db.InsertChapter(bookID, ch.Number, ch.Title, ch.Body); err != nil {
			log.Printf("insert imported chapter %d: %v", ch.Number, err)
			failedChapters++
		}
	}

	msg := fmt.Sprintf("Imported %q with %d chapters.", title, len(chapters))
	if failedChapters > 0 {
		msg = fmt.Sprintf("Imported %q with %d chapters (%d failed).", title, len(chapters), failedChapters)
	}

	h.renderPartial(w, "import-result", map[string]any{
		"Success": msg,
		"BookID":  bookID,
	})
}

// tokenizeText tokenizes text via Voikko, falling back to plain text.
func (h *Handlers) tokenizeText(text string) ([]voikko.TokenAnalysis, string) {
	sv, err := h.voikko.ValidateSentence(text)
	if err != nil {
		log.Printf("voikko tokenize error: %v", err)
		return nil, text
	}
	return sv.Tokens, text
}
