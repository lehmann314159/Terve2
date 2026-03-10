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

	err := db.SaveBookmark(userID, bookID, chID, 0)
	if err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	gotCh, gotPar := db.GetBookmark(userID, bookID)
	if gotCh != chID {
		t.Errorf("GetBookmark chapterID = %d, want %d", gotCh, chID)
	}
	if gotPar != 0 {
		t.Errorf("GetBookmark paragraph = %d, want 0", gotPar)
	}
}

func TestSaveBookmark_Upsert(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm2", "User", "", "")
	gid := 33333
	bookID, _ := db.InsertBook("Upsert Book", "Author", "", &gid, "test")
	ch1, _ := db.InsertChapter(bookID, 1, "Ch1", "Content 1")
	ch2, _ := db.InsertChapter(bookID, 2, "Ch2", "Content 2")

	db.SaveBookmark(userID, bookID, ch1, 0)
	db.SaveBookmark(userID, bookID, ch2, 0) // update

	got, _ := db.GetBookmark(userID, bookID)
	if got != ch2 {
		t.Errorf("GetBookmark after upsert = %d, want %d", got, ch2)
	}
}

func TestGetBookmark_NoBookmark(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm3", "User", "", "")
	got, gotPar := db.GetBookmark(userID, 99999)
	if got != 0 {
		t.Errorf("GetBookmark with no bookmark = %d, want 0", got)
	}
	if gotPar != 0 {
		t.Errorf("GetBookmark paragraph with no bookmark = %d, want 0", gotPar)
	}
}

func TestSaveBookmark_ParagraphRoundTrip(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "bm5", "User", "", "")
	gid := 11111
	bookID, _ := db.InsertBook("Paragraph Book", "Author", "", &gid, "test")
	chID, _ := db.InsertChapter(bookID, 1, "Ch1", "Content")

	// Save with paragraph=5
	err := db.SaveBookmark(userID, bookID, chID, 5)
	if err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	gotCh, gotPar := db.GetBookmark(userID, bookID)
	if gotCh != chID {
		t.Errorf("chapterID = %d, want %d", gotCh, chID)
	}
	if gotPar != 5 {
		t.Errorf("paragraph = %d, want 5", gotPar)
	}

	// Update to paragraph=3
	db.SaveBookmark(userID, bookID, chID, 3)
	_, gotPar = db.GetBookmark(userID, bookID)
	if gotPar != 3 {
		t.Errorf("paragraph after update = %d, want 3", gotPar)
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

	db.SaveBookmark(userID, book1, ch1, 0)
	db.SaveBookmark(userID, book2, ch2, 0)

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

// --- Quiz tests ---

func TestSaveQuizResult_AndGetRecent(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "qz1", "Quiz User", "", "")

	err := db.SaveQuizResult(userID, "case_id", 10, 7)
	if err != nil {
		t.Fatalf("SaveQuizResult: %v", err)
	}

	err = db.SaveQuizResult(userID, "form_english", 10, 9)
	if err != nil {
		t.Fatalf("SaveQuizResult: %v", err)
	}

	// Get all types
	results, err := db.GetRecentQuizResults(userID, "", 10)
	if err != nil {
		t.Fatalf("GetRecentQuizResults (all): %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Get specific type
	results, err = db.GetRecentQuizResults(userID, "case_id", 10)
	if err != nil {
		t.Fatalf("GetRecentQuizResults (case_id): %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for case_id, got %d", len(results))
	}
	if results[0].Correct != 7 || results[0].Total != 10 {
		t.Errorf("expected 7/10, got %d/%d", results[0].Correct, results[0].Total)
	}
}

func TestGetRandomUserCard(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "rnd1", "Random User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "satunnainen", Lemma: "satunnainen", WordClass: "adjective", Translation: "random"})
	db.SaveUserCard(userID, cardID)

	card, err := db.GetRandomUserCard(userID)
	if err != nil {
		t.Fatalf("GetRandomUserCard: %v", err)
	}
	if card == nil {
		t.Fatal("expected non-nil card")
	}
}

func TestGetRandomUserCardWithTranslation(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "rnd2", "Random User 2", "", "")

	// Card with translation
	c1, _ := db.CreateCard(&Card{Finnish: "hyvä", Lemma: "hyvä", WordClass: "adjective", Translation: "good"})
	db.SaveUserCard(userID, c1)

	// Card without translation
	c2, _ := db.CreateCard(&Card{Finnish: "tyhjä", Lemma: "tyhjä", WordClass: "adjective"})
	db.SaveUserCard(userID, c2)

	card, err := db.GetRandomUserCardWithTranslation(userID)
	if err != nil {
		t.Fatalf("GetRandomUserCardWithTranslation: %v", err)
	}
	if card.Translation == "" {
		t.Error("expected card with non-empty translation")
	}
}

func TestGetWeightedRandomUserCard(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "wrnd1", "Weighted User", "", "")
	cardID, _ := db.CreateCard(&Card{Finnish: "painotettu", Lemma: "painotettu", WordClass: "adjective", Translation: "weighted"})
	db.SaveUserCard(userID, cardID)

	card, err := db.GetWeightedRandomUserCard(userID, nil)
	if err != nil {
		t.Fatalf("GetWeightedRandomUserCard: %v", err)
	}
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card.Finnish != "painotettu" {
		t.Errorf("finnish = %q, want %q", card.Finnish, "painotettu")
	}
}

func TestGetWeightedRandomUserCard_ExcludeIDs(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "wrnd2", "Weighted User 2", "", "")

	c1, _ := db.CreateCard(&Card{Finnish: "ensimmäinen_w", Lemma: "ensimmäinen_w", WordClass: "adjective"})
	c2, _ := db.CreateCard(&Card{Finnish: "toinen_w", Lemma: "toinen_w", WordClass: "adjective"})
	db.SaveUserCard(userID, c1)
	db.SaveUserCard(userID, c2)

	// Exclude c1, should only get c2
	card, err := db.GetWeightedRandomUserCard(userID, []int64{c1})
	if err != nil {
		t.Fatalf("GetWeightedRandomUserCard with exclude: %v", err)
	}
	if card.ID != c2 {
		t.Errorf("expected card ID %d, got %d", c2, card.ID)
	}
}

func TestGetWeightedRandomUserCardWithTranslation(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "wrnd3", "Weighted User 3", "", "")

	// Card with translation
	c1, _ := db.CreateCard(&Card{Finnish: "hyvä_w", Lemma: "hyvä_w", WordClass: "adjective", Translation: "good"})
	db.SaveUserCard(userID, c1)

	// Card without translation
	c2, _ := db.CreateCard(&Card{Finnish: "tyhjä_w", Lemma: "tyhjä_w", WordClass: "adjective"})
	db.SaveUserCard(userID, c2)

	card, err := db.GetWeightedRandomUserCardWithTranslation(userID, nil)
	if err != nil {
		t.Fatalf("GetWeightedRandomUserCardWithTranslation: %v", err)
	}
	if card.Translation == "" {
		t.Error("expected card with non-empty translation")
	}
}

func TestGetWeightedRandomUserCardWithTranslation_ExcludeIDs(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "wrnd4", "Weighted User 4", "", "")

	c1, _ := db.CreateCard(&Card{Finnish: "eka_w", Lemma: "eka_w", WordClass: "noun", Translation: "first"})
	c2, _ := db.CreateCard(&Card{Finnish: "toka_w", Lemma: "toka_w", WordClass: "noun", Translation: "second"})
	db.SaveUserCard(userID, c1)
	db.SaveUserCard(userID, c2)

	// Exclude c1
	card, err := db.GetWeightedRandomUserCardWithTranslation(userID, []int64{c1})
	if err != nil {
		t.Fatalf("GetWeightedRandomUserCardWithTranslation with exclude: %v", err)
	}
	if card.ID != c2 {
		t.Errorf("expected card ID %d, got %d", c2, card.ID)
	}
}

func TestGetRandomUserCards_Distractors(t *testing.T) {
	db := testDB(t)

	userID, _ := db.UpsertUser("google", "rnd3", "Random User 3", "", "")

	// Create several cards with unique names and translations, track IDs
	var cardIDs []int64
	for _, w := range []struct{ fi, en string }{
		{"quiztesti_a", "quiz test a"},
		{"quiztesti_b", "quiz test b"},
		{"quiztesti_c", "quiz test c"},
		{"quiztesti_d", "quiz test d"},
	} {
		cid, _ := db.CreateCard(&Card{Finnish: w.fi, Lemma: w.fi, WordClass: "noun", Translation: w.en})
		db.SaveUserCard(userID, cid)
		cardIDs = append(cardIDs, cid)
	}

	// Get 3 distractors excluding the first card
	excludeID := cardIDs[0]
	cards, err := db.GetRandomUserCards(userID, excludeID, 3)
	if err != nil {
		t.Fatalf("GetRandomUserCards: %v", err)
	}
	if len(cards) < 1 {
		t.Errorf("expected at least 1 distractor card, got %d", len(cards))
	}
	for _, c := range cards {
		if c.ID == excludeID {
			t.Error("excluded card should not appear in results")
		}
	}
}

// --- Paradigm cache tests ---

func TestSaveParadigm_AndGetParadigm(t *testing.T) {
	db := testDB(t)

	forms := map[string]string{
		"nominative_singular": "talo",
		"genitive_singular":   "talon",
		"partitive_singular":  "taloa",
		"inessive_singular":   "talossa",
	}

	err := db.SaveParadigm("talo", "noun", "", forms)
	if err != nil {
		t.Fatalf("SaveParadigm: %v", err)
	}

	got, err := db.GetParadigm("talo", "noun", "")
	if err != nil {
		t.Fatalf("GetParadigm: %v", err)
	}

	if got["nominative_singular"] != "talo" {
		t.Errorf("nominative_singular = %q, want %q", got["nominative_singular"], "talo")
	}
	if got["inessive_singular"] != "talossa" {
		t.Errorf("inessive_singular = %q, want %q", got["inessive_singular"], "talossa")
	}
	if len(got) != 4 {
		t.Errorf("expected 4 forms, got %d", len(got))
	}
}

func TestGetParadigm_CacheMiss(t *testing.T) {
	db := testDB(t)

	_, err := db.GetParadigm("nonexistent", "noun", "")
	if err == nil {
		t.Fatal("expected error for cache miss")
	}
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// --- Sentence cache tests ---

func TestSaveSentences_AndGetByLemma(t *testing.T) {
	db := testDB(t)

	sentences := []CachedSentence{
		{Lemma: "talo", Finnish: "Menen taloon.", English: "I go to the house.", TargetForm: "taloon"},
		{Lemma: "talo", Finnish: "Talo on suuri.", English: "The house is big.", TargetForm: "Talo"},
	}

	err := db.SaveSentences(sentences)
	if err != nil {
		t.Fatalf("SaveSentences: %v", err)
	}

	got, err := db.GetSentencesByLemma("talo")
	if err != nil {
		t.Fatalf("GetSentencesByLemma: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 sentences, got %d", len(got))
	}
	if got[0].Finnish != "Menen taloon." {
		t.Errorf("finnish = %q, want %q", got[0].Finnish, "Menen taloon.")
	}
	if got[0].ID == 0 {
		t.Error("expected non-zero ID after save")
	}
}

func TestGetSentencesByLemma_Empty(t *testing.T) {
	db := testDB(t)

	got, err := db.GetSentencesByLemma("nonexistent")
	if err != nil {
		t.Fatalf("GetSentencesByLemma: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice for no results, got %d items", len(got))
	}
}

func TestGetRandomSentencesExcludingLemma(t *testing.T) {
	db := testDB(t)

	sentences := []CachedSentence{
		{Lemma: "talo", Finnish: "Talo on suuri.", English: "The house is big.", TargetForm: "Talo"},
		{Lemma: "koira", Finnish: "Koira juoksee.", English: "The dog runs.", TargetForm: "Koira"},
		{Lemma: "kissa", Finnish: "Kissa nukkuu.", English: "The cat sleeps.", TargetForm: "Kissa"},
	}
	db.SaveSentences(sentences)

	got, err := db.GetRandomSentencesExcludingLemma("talo", 10)
	if err != nil {
		t.Fatalf("GetRandomSentencesExcludingLemma: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 sentences (excluding talo), got %d", len(got))
	}
	for _, s := range got {
		if s.Lemma == "talo" {
			t.Error("excluded lemma should not appear in results")
		}
	}
}

func TestGetRandomSentencesExcludingLemma_NoOthers(t *testing.T) {
	db := testDB(t)

	sentences := []CachedSentence{
		{Lemma: "talo", Finnish: "Talo on suuri.", English: "The house is big.", TargetForm: "Talo"},
	}
	db.SaveSentences(sentences)

	got, err := db.GetRandomSentencesExcludingLemma("talo", 10)
	if err != nil {
		t.Fatalf("GetRandomSentencesExcludingLemma: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice when only excluded lemma exists, got %d items", len(got))
	}
}

func TestSaveParadigm_Upsert(t *testing.T) {
	db := testDB(t)

	original := map[string]string{"nominative_singular": "talo"}
	updated := map[string]string{"nominative_singular": "talo", "genitive_singular": "talon"}

	db.SaveParadigm("talo", "noun", "", original)
	db.SaveParadigm("talo", "noun", "", updated)

	got, err := db.GetParadigm("talo", "noun", "")
	if err != nil {
		t.Fatalf("GetParadigm after upsert: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 forms after upsert, got %d", len(got))
	}
	if got["genitive_singular"] != "talon" {
		t.Errorf("genitive_singular = %q, want %q", got["genitive_singular"], "talon")
	}
}
