package db

import (
	"database/sql"
	"testing"
	"time"
)

// testDB opens an in-memory SQLite database with full migration + seeding.
func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- User tests ---

func TestUpsertUser_CreatesNew(t *testing.T) {
	db := testDB(t)

	id, err := db.UpsertUser("google", "123", "Test User", "test@example.com", "https://avatar.url")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero user ID")
	}
}

func TestUpsertUser_UpdatesExisting(t *testing.T) {
	db := testDB(t)

	id1, err := db.UpsertUser("github", "456", "Original Name", "old@example.com", "")
	if err != nil {
		t.Fatalf("first UpsertUser: %v", err)
	}

	id2, err := db.UpsertUser("github", "456", "Updated Name", "new@example.com", "https://new-avatar.url")
	if err != nil {
		t.Fatalf("second UpsertUser: %v", err)
	}

	if id1 != id2 {
		t.Errorf("expected same ID on upsert, got %d and %d", id1, id2)
	}

	user, err := db.GetUser(id2)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Name != "Updated Name" {
		t.Errorf("name = %q, want %q", user.Name, "Updated Name")
	}
	if user.Email != "new@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "new@example.com")
	}
}

func TestGetUser_Found(t *testing.T) {
	db := testDB(t)

	id, _ := db.UpsertUser("google", "789", "Found User", "found@example.com", "")
	user, err := db.GetUser(id)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Provider != "google" || user.ProviderID != "789" {
		t.Errorf("provider info mismatch: %s/%s", user.Provider, user.ProviderID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.GetUser(99999)
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// --- Book tests ---

func TestInsertBook_AndGetBook(t *testing.T) {
	db := testDB(t)

	gid := 99999
	bookID, err := db.InsertBook("Test Book", "Test Author", "A test book", &gid, "test")
	if err != nil {
		t.Fatalf("InsertBook: %v", err)
	}
	if bookID == 0 {
		t.Fatal("expected non-zero book ID")
	}

	book, err := db.GetBook(bookID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if book.Title != "Test Book" {
		t.Errorf("title = %q, want %q", book.Title, "Test Book")
	}
	if book.Author != "Test Author" {
		t.Errorf("author = %q, want %q", book.Author, "Test Author")
	}
	if book.GutenbergID == nil || *book.GutenbergID != 99999 {
		t.Errorf("gutenberg_id = %v, want 99999", book.GutenbergID)
	}
}

func TestInsertBook_ConflictReturnsExisting(t *testing.T) {
	db := testDB(t)

	gid := 88888
	id1, err := db.InsertBook("Book One", "Author", "", &gid, "test")
	if err != nil {
		t.Fatalf("first InsertBook: %v", err)
	}

	id2, err := db.InsertBook("Book Two", "Author", "", &gid, "test")
	if err != nil {
		t.Fatalf("second InsertBook: %v", err)
	}

	if id1 != id2 {
		t.Errorf("expected same ID on conflict, got %d and %d", id1, id2)
	}
}

func TestListBooks(t *testing.T) {
	db := testDB(t)

	books, err := db.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}

	// Seed books should be present
	if len(books) == 0 {
		t.Fatal("expected seed books, got 0")
	}

	// Each book should have a chapter count > 0 (seed books have chapters)
	for _, b := range books {
		if b.ChapterCount == 0 {
			t.Errorf("book %q has 0 chapters", b.Title)
		}
	}
}

func TestSeedChapterContentSizes(t *testing.T) {
	db := testDB(t)

	books, err := db.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}

	for _, b := range books {
		chapters, err := db.GetChapters(b.ID)
		if err != nil {
			t.Errorf("GetChapters(%d): %v", b.ID, err)
			continue
		}
		for _, ch := range chapters {
			t.Logf("%-30s Ch %2d: %6d bytes  title=%q", b.Title, ch.ChapterNumber, len(ch.Content), ch.Title)
			if len(ch.Content) < 200 {
				t.Errorf("book %q ch %d content too short: %d bytes (min 200)", b.Title, ch.ChapterNumber, len(ch.Content))
			}
		}
	}
}

func TestInsertChapter_AndGetChapter(t *testing.T) {
	db := testDB(t)

	gid := 77777
	bookID, _ := db.InsertBook("Chapter Test", "Author", "", &gid, "test")

	chID, err := db.InsertChapter(bookID, 1, "Chapter One", "Content of chapter one.")
	if err != nil {
		t.Fatalf("InsertChapter: %v", err)
	}
	if chID == 0 {
		t.Fatal("expected non-zero chapter ID")
	}

	ch, err := db.GetChapter(bookID, 1)
	if err != nil {
		t.Fatalf("GetChapter: %v", err)
	}
	if ch.Title != "Chapter One" {
		t.Errorf("title = %q, want %q", ch.Title, "Chapter One")
	}
	if ch.Content != "Content of chapter one." {
		t.Errorf("content = %q", ch.Content)
	}
}

func TestGetChapters_Ordered(t *testing.T) {
	db := testDB(t)

	gid := 66666
	bookID, _ := db.InsertBook("Ordered Chapters", "Author", "", &gid, "test")

	// Insert out of order
	db.InsertChapter(bookID, 3, "Three", "Third")
	db.InsertChapter(bookID, 1, "One", "First")
	db.InsertChapter(bookID, 2, "Two", "Second")

	chapters, err := db.GetChapters(bookID)
	if err != nil {
		t.Fatalf("GetChapters: %v", err)
	}
	if len(chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(chapters))
	}

	for i, ch := range chapters {
		if ch.ChapterNumber != i+1 {
			t.Errorf("chapter[%d].ChapterNumber = %d, want %d", i, ch.ChapterNumber, i+1)
		}
	}
}

func TestBookExistsByGutenbergID(t *testing.T) {
	db := testDB(t)

	gid := 55555
	db.InsertBook("Exists Test", "Author", "", &gid, "test")

	if !db.BookExistsByGutenbergID(55555) {
		t.Error("expected book to exist")
	}
	if db.BookExistsByGutenbergID(11111) {
		t.Error("expected book NOT to exist")
	}
}

// --- Bookmark tests ---

func TestSaveBookmark_AndGetBookmark(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm1", "Bookmark User", "", "")
	gid := 44444
	bookID, _ := db.InsertBook("Bookmark Book", "Author", "", &gid, "test")
	chID, _ := db.InsertChapter(bookID, 1, "Ch1", "Content")

	err := db.SaveBookmark(userID, bookID, chID)
	if err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	got := db.GetBookmark(userID, bookID)
	if got != chID {
		t.Errorf("GetBookmark = %d, want %d", got, chID)
	}
}

func TestSaveBookmark_Upsert(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm2", "User", "", "")
	gid := 33333
	bookID, _ := db.InsertBook("Upsert Book", "Author", "", &gid, "test")
	ch1, _ := db.InsertChapter(bookID, 1, "Ch1", "Content 1")
	ch2, _ := db.InsertChapter(bookID, 2, "Ch2", "Content 2")

	db.SaveBookmark(userID, bookID, ch1)
	db.SaveBookmark(userID, bookID, ch2) // update

	got := db.GetBookmark(userID, bookID)
	if got != ch2 {
		t.Errorf("GetBookmark after upsert = %d, want %d", got, ch2)
	}
}

func TestGetBookmark_NoBookmark(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm3", "User", "", "")
	got := db.GetBookmark(userID, 99999)
	if got != 0 {
		t.Errorf("GetBookmark with no bookmark = %d, want 0", got)
	}
}

func TestGetUserBookmarks(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm4", "User", "", "")
	gid1, gid2 := 22222, 22223
	book1, _ := db.InsertBook("Book1", "A", "", &gid1, "test")
	book2, _ := db.InsertBook("Book2", "A", "", &gid2, "test")
	ch1, _ := db.InsertChapter(book1, 1, "Ch1", "C")
	ch2, _ := db.InsertChapter(book2, 1, "Ch1", "C")

	db.SaveBookmark(userID, book1, ch1)
	db.SaveBookmark(userID, book2, ch2)

	bm := db.GetUserBookmarks(userID)
	if len(bm) != 2 {
		t.Fatalf("expected 2 bookmarks, got %d", len(bm))
	}
	if bm[book1] != ch1 {
		t.Errorf("bookmark for book1: got %d, want %d", bm[book1], ch1)
	}
	if bm[book2] != ch2 {
		t.Errorf("bookmark for book2: got %d, want %d", bm[book2], ch2)
	}
}

// --- Card tests ---

func TestCreateCard_AndGetUserCard(t *testing.T) {
	db := testDB(t)

	cardID, err := db.CreateCard(&Card{
		Finnish:     "testi",
		Lemma:       "testi",
		WordClass:   "noun",
		Translation: "test",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if cardID == 0 {
		t.Fatal("expected non-zero card ID")
	}

	userID, _ := db.UpsertUser("google", "c1", "Card User", "", "")
	ucID, err := db.SaveUserCard(userID, cardID)
	if err != nil {
		t.Fatalf("SaveUserCard: %v", err)
	}

	uc, err := db.GetUserCard(ucID, userID)
	if err != nil {
		t.Fatalf("GetUserCard: %v", err)
	}
	if uc.Card.Finnish != "testi" {
		t.Errorf("card finnish = %q, want %q", uc.Card.Finnish, "testi")
	}
	if uc.Card.Translation != "test" {
		t.Errorf("card translation = %q, want %q", uc.Card.Translation, "test")
	}
}

func TestSaveUserCard_LinkUserToCard(t *testing.T) {
	db := testDB(t)

	cardID, _ := db.CreateCard(&Card{Finnish: "linkki", Lemma: "linkki", WordClass: "noun"})
	userID, _ := db.UpsertUser("github", "c2", "Link User", "", "")

	ucID, err := db.SaveUserCard(userID, cardID)
	if err != nil {
		t.Fatalf("SaveUserCard: %v", err)
	}
	if ucID == 0 {
		t.Fatal("expected non-zero user_card ID")
	}

	// Saving again should return same ID (ON CONFLICT DO NOTHING)
	ucID2, err := db.SaveUserCard(userID, cardID)
	if err != nil {
		t.Fatalf("SaveUserCard duplicate: %v", err)
	}
	if ucID != ucID2 {
		t.Errorf("duplicate SaveUserCard: got %d, want %d", ucID2, ucID)
	}
}

func TestGetDueCards(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "due1", "Due User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "erääntyvä", Lemma: "erääntyvä", WordClass: "adjective"})
	db.SaveUserCard(userID, cardID)

	// Default next_review is CURRENT_TIMESTAMP, so it should be due immediately
	due, err := db.GetDueCards(userID, 10)
	if err != nil {
		t.Fatalf("GetDueCards: %v", err)
	}

	found := false
	for _, uc := range due {
		if uc.Card.Finnish == "erääntyvä" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected card to be in due list")
	}
}

func TestUpdateReview(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "rev1", "Review User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "arvostelu", Lemma: "arvostelu", WordClass: "noun"})
	ucID, _ := db.SaveUserCard(userID, cardID)

	nextReview := time.Now().Add(24 * time.Hour)
	err := db.UpdateReview(ucID, 2.8, 3, 2, nextReview)
	if err != nil {
		t.Fatalf("UpdateReview: %v", err)
	}

	uc, err := db.GetUserCard(ucID, userID)
	if err != nil {
		t.Fatalf("GetUserCard after review: %v", err)
	}
	if uc.EaseFactor != 2.8 {
		t.Errorf("ease_factor = %f, want 2.8", uc.EaseFactor)
	}
	if uc.IntervalDays != 3 {
		t.Errorf("interval_days = %d, want 3", uc.IntervalDays)
	}
	if uc.Repetitions != 2 {
		t.Errorf("repetitions = %d, want 2", uc.Repetitions)
	}
	if uc.LastReview == nil {
		t.Error("last_review should be set after UpdateReview")
	}
}

func TestToggleFocus(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "foc1", "Focus User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "fokus", Lemma: "fokus", WordClass: "noun"})
	ucID, _ := db.SaveUserCard(userID, cardID)

	// Should start unfocused
	uc, _ := db.GetUserCard(ucID, userID)
	if uc.Focused {
		t.Error("card should start unfocused")
	}

	// Toggle on
	if err := db.ToggleFocus(ucID, userID); err != nil {
		t.Fatalf("ToggleFocus: %v", err)
	}
	uc, _ = db.GetUserCard(ucID, userID)
	if !uc.Focused {
		t.Error("card should be focused after toggle")
	}

	// Toggle off
	db.ToggleFocus(ucID, userID)
	uc, _ = db.GetUserCard(ucID, userID)
	if uc.Focused {
		t.Error("card should be unfocused after second toggle")
	}
}

func TestCountUserCards(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "cnt1", "Count User", "", "")

	// Create and link two cards
	c1, _ := db.CreateCard(&Card{Finnish: "yksi_testi", Lemma: "yksi_testi", WordClass: "noun"})
	c2, _ := db.CreateCard(&Card{Finnish: "kaksi_testi", Lemma: "kaksi_testi", WordClass: "noun"})
	db.SaveUserCard(userID, c1)
	db.SaveUserCard(userID, c2)

	total, due, err := db.CountUserCards(userID)
	if err != nil {
		t.Fatalf("CountUserCards: %v", err)
	}
	if total < 2 {
		t.Errorf("total = %d, want at least 2", total)
	}
	// Both cards default to CURRENT_TIMESTAMP for next_review, so should be due
	if due < 2 {
		t.Errorf("due = %d, want at least 2", due)
	}
}

func TestUserHasCard(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "has1", "Has User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "löytyy", Lemma: "löytyy", WordClass: "verb"})
	db.SaveUserCard(userID, cardID)

	if !db.UserHasCard(userID, "löytyy", "löytyy") {
		t.Error("expected UserHasCard to return true")
	}
	if db.UserHasCard(userID, "nonexistent", "nonexistent") {
		t.Error("expected UserHasCard to return false for unknown card")
	}
}

func TestDeleteUserCard(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "del1", "Delete User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "poistettava", Lemma: "poistettava", WordClass: "adjective"})
	ucID, _ := db.SaveUserCard(userID, cardID)

	err := db.DeleteUserCard(ucID, userID)
	if err != nil {
		t.Fatalf("DeleteUserCard: %v", err)
	}

	_, err = db.GetUserCard(ucID, userID)
	if err == nil {
		t.Error("expected error after deleting user_card")
	}
}
