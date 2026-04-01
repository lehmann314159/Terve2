package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/db"
	"github.com/lehmann314159/terve2/internal/ollama"
)

// FlashcardsPageData is passed to the flashcards page template.
type FlashcardsPageData struct {
	PageData
	Cards    []db.UserCard
	Total    int
	Due      int
	Filter   string
}

// FlashcardsPage renders the full flashcards page.
func (h *Handlers) FlashcardsPage(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	// Auto-enroll in seed cards on first visit
	if err := h.db.EnrollUserInSeedCards(sess.DBUserID); err != nil {
		log.Printf("enroll seed cards: %v", err)
	}

	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	cards, err := h.db.ListUserCards(sess.DBUserID, filter)
	if err != nil {
		log.Printf("list user cards: %v", err)
		http.Error(w, "Failed to load flashcards", http.StatusInternalServerError)
		return
	}

	total, due, _ := h.db.CountUserCards(sess.DBUserID)

	h.render(w, "base", FlashcardsPageData{
		PageData: pageData(r, "Terve — Flashcards", "flashcards"),
		Cards:    cards,
		Total:    total,
		Due:      due,
		Filter:   filter,
	})
}

// FlashcardList returns the filtered card list partial (for HTMX).
func (h *Handlers) FlashcardList(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	cards, err := h.db.ListUserCards(sess.DBUserID, filter)
	if err != nil {
		log.Printf("list user cards: %v", err)
		http.Error(w, "Failed to load flashcards", http.StatusInternalServerError)
		return
	}

	total, due, _ := h.db.CountUserCards(sess.DBUserID)

	h.renderPartial(w, "flashcard-list", FlashcardsPageData{
		PageData: pageData(r, "", "flashcards"),
		Cards:    cards,
		Total:    total,
		Due:      due,
		Filter:   filter,
	})
}

// maxFieldLen is the maximum allowed length for flashcard text fields.
const maxFieldLen = 1000

// validateCardFields checks that required fields are present and within length limits.
func validateCardFields(finnish string, fields ...string) string {
	if finnish == "" {
		return "Finnish word is required."
	}
	if len(finnish) > maxFieldLen {
		return "Finnish word is too long."
	}
	for _, f := range fields {
		if len(f) > maxFieldLen {
			return "One or more fields exceed the maximum length."
		}
	}
	return ""
}

// SaveFlashcard saves a card from the analysis panel.
func (h *Handlers) SaveFlashcard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	finnish := r.FormValue("finnish")
	lemma := r.FormValue("lemma")
	if lemma == "" {
		lemma = finnish
	}

	if msg := validateCardFields(finnish, lemma, r.FormValue("word_class"), r.FormValue("morphology"),
		r.FormValue("translation"), r.FormValue("explanation"), r.FormValue("context")); msg != "" {
		h.renderPartial(w, "save-result", map[string]string{"Error": msg})
		return
	}

	card := &db.Card{
		Finnish:     finnish,
		Lemma:       lemma,
		WordClass:   r.FormValue("word_class"),
		Morphology:  r.FormValue("morphology"),
		Translation: r.FormValue("translation"),
		Explanation: r.FormValue("explanation"),
		Context:     r.FormValue("context"),
		Source:      "user",
	}

	cardID, err := h.db.CreateCard(card)
	if err != nil {
		log.Printf("create card: %v", err)
		h.renderPartial(w, "save-result", map[string]string{"Error": "Failed to save card."})
		return
	}

	if _, err := h.db.SaveUserCard(sess.DBUserID, cardID); err != nil {
		log.Printf("save user card: %v", err)
		h.renderPartial(w, "save-result", map[string]string{"Error": "Failed to save card."})
		return
	}

	h.renderPartial(w, "save-result", map[string]string{"Success": "Card saved!"})
}

// WordCardBtnData is passed to the word-card-btn partial template.
type WordCardBtnData struct {
	Saved       bool
	UserCardID  int64
	Finnish     string
	Lemma       string
	WordClass   string
	Morphology  string
	Translation string
}

// SaveWordCard saves a single word from the morph table as a flashcard.
func (h *Handlers) SaveWordCard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	finnish := r.FormValue("finnish")
	lemma := r.FormValue("lemma")
	if lemma == "" {
		lemma = finnish
	}
	wordClass := r.FormValue("word_class")
	morphology := r.FormValue("morphology")
	translation := r.FormValue("translation")

	card := &db.Card{
		Finnish:     finnish,
		Lemma:       lemma,
		WordClass:   wordClass,
		Morphology:  morphology,
		Translation: translation,
		Source:      "user",
	}

	cardID, err := h.db.CreateCard(card)
	if err != nil {
		log.Printf("save word card: %v", err)
		h.renderPartial(w, "word-card-btn", WordCardBtnData{Finnish: finnish, Lemma: lemma, WordClass: wordClass, Morphology: morphology, Translation: translation})
		return
	}

	ucID, err := h.db.SaveUserCard(sess.DBUserID, cardID)
	if err != nil {
		log.Printf("save user card (word): %v", err)
		h.renderPartial(w, "word-card-btn", WordCardBtnData{Finnish: finnish, Lemma: lemma, WordClass: wordClass, Morphology: morphology, Translation: translation})
		return
	}

	h.renderPartial(w, "word-card-btn", WordCardBtnData{
		Saved:       true,
		UserCardID:  ucID,
		Finnish:     finnish,
		Lemma:       lemma,
		WordClass:   wordClass,
		Morphology:  morphology,
		Translation: translation,
	})
}

// RemoveWordCard removes a word flashcard from the morph table.
func (h *Handlers) RemoveWordCard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	ucID, err := strconv.ParseInt(r.FormValue("uc_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteUserCard(ucID, sess.DBUserID); err != nil {
		log.Printf("remove word card: %v", err)
	}

	h.renderPartial(w, "word-card-btn", WordCardBtnData{
		Finnish:     r.FormValue("finnish"),
		Lemma:       r.FormValue("lemma"),
		WordClass:   r.FormValue("word_class"),
		Morphology:  r.FormValue("morphology"),
		Translation: r.FormValue("translation"),
	})
}

// ValidateFlashcardData is the preview data for manual add.
type ValidateFlashcardData struct {
	Finnish     string
	Lemma       string
	WordClass   string
	Morphology  string
	Translation string
	Error       string
	AlreadySaved bool
}

// ValidateFlashcard previews a manually entered word via Voikko + Ollama.
func (h *Handlers) ValidateFlashcard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	finnish := r.FormValue("finnish")

	if finnish == "" {
		h.renderPartial(w, "validate-result", ValidateFlashcardData{Error: "Please enter a Finnish word."})
		return
	}

	// Check if already saved
	alreadySaved := h.db.UserHasCard(sess.DBUserID, finnish, finnish)

	// Get morphology from Voikko
	var lemma, wordClass, morphology string
	analyses, err := h.voikko.AnalyzeWord(finnish)
	if err != nil {
		log.Printf("voikko analyze for flashcard: %v", err)
	} else if len(analyses) > 0 {
		a := analyses[0]
		lemma = a.Lemma
		wordClass = a.WordClass
		if a.Case != "" {
			morphology = a.Case
		}
		if a.Number != "" {
			if morphology != "" {
				morphology += ", "
			}
			morphology += a.Number
		}
		// Check with the lemma too
		if !alreadySaved {
			alreadySaved = h.db.UserHasCard(sess.DBUserID, lemma, finnish)
		}
	}
	if lemma == "" {
		lemma = finnish
	}

	// Get translation from Ollama
	var translation string
	prompt := ollama.BuildPrompt(finnish, "", nil)
	resp, err := h.ollama.Generate(ollama.SystemPrompt, prompt)
	if err != nil {
		log.Printf("ollama for flashcard validate: %v", err)
	} else {
		translation, _ = ollama.ParseResponse(resp)
	}

	h.renderPartial(w, "validate-result", ValidateFlashcardData{
		Finnish:      finnish,
		Lemma:        lemma,
		WordClass:    wordClass,
		Morphology:   morphology,
		Translation:  translation,
		AlreadySaved: alreadySaved,
	})
}

// AddFlashcard adds a manually validated card.
func (h *Handlers) AddFlashcard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	finnish := r.FormValue("finnish")
	if msg := validateCardFields(finnish, r.FormValue("lemma"), r.FormValue("word_class"),
		r.FormValue("morphology"), r.FormValue("translation")); msg != "" {
		h.renderPartial(w, "save-result", map[string]string{"Error": msg})
		return
	}

	card := &db.Card{
		Finnish:     finnish,
		Lemma:       r.FormValue("lemma"),
		WordClass:   r.FormValue("word_class"),
		Morphology:  r.FormValue("morphology"),
		Translation: r.FormValue("translation"),
		Source:      "user",
	}

	cardID, err := h.db.CreateCard(card)
	if err != nil {
		log.Printf("create card (manual): %v", err)
		h.renderPartial(w, "save-result", map[string]string{"Error": "Failed to add card."})
		return
	}

	if _, err := h.db.SaveUserCard(sess.DBUserID, cardID); err != nil {
		log.Printf("save user card (manual): %v", err)
		h.renderPartial(w, "save-result", map[string]string{"Error": "Failed to add card."})
		return
	}

	// Return updated list
	h.FlashcardList(w, r)
}

// DeleteFlashcard removes a user's card.
func (h *Handlers) DeleteFlashcard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	ucID, err := strconv.ParseInt(chi.URLParam(r, "cardID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteUserCard(ucID, sess.DBUserID); err != nil {
		log.Printf("delete user card: %v", err)
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	h.FlashcardList(w, r)
}

// ToggleFocus toggles the focus flag on a user's card.
func (h *Handlers) ToggleFocus(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	ucID, err := strconv.ParseInt(chi.URLParam(r, "cardID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

	if err := h.db.ToggleFocus(ucID, sess.DBUserID); err != nil {
		log.Printf("toggle focus: %v", err)
		http.Error(w, "Failed to toggle focus", http.StatusInternalServerError)
		return
	}

	h.FlashcardList(w, r)
}

// ToggleFocusReview toggles focus and re-renders the review card in place.
func (h *Handlers) ToggleFocusReview(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	ucID, err := strconv.ParseInt(chi.URLParam(r, "cardID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

	if err := h.db.ToggleFocus(ucID, sess.DBUserID); err != nil {
		log.Printf("toggle focus (review): %v", err)
		http.Error(w, "Failed to toggle focus", http.StatusInternalServerError)
		return
	}

	uc, err := h.db.GetUserCard(ucID, sess.DBUserID)
	if err != nil {
		log.Printf("get user card after focus toggle: %v", err)
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	h.renderPartial(w, "review-card", ReviewCardData{UserCard: uc})
}

// ReviewCardData is passed to the review card template.
type ReviewCardData struct {
	UserCard *db.UserCard
	Done     bool
}

// ReviewSession returns the next due card for review.
func (h *Handlers) ReviewSession(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	cards, err := h.db.GetDueCards(sess.DBUserID, 1)
	if err != nil {
		log.Printf("get due cards: %v", err)
		h.renderPartial(w, "review-card", ReviewCardData{Done: true})
		return
	}

	if len(cards) == 0 {
		h.renderPartial(w, "review-card", ReviewCardData{Done: true})
		return
	}

	h.renderPartial(w, "review-card", ReviewCardData{UserCard: &cards[0]})
}

// SubmitReview processes a review rating and returns the next card.
func (h *Handlers) SubmitReview(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	ucID, err := strconv.ParseInt(chi.URLParam(r, "userCardID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

	quality, err := strconv.Atoi(r.FormValue("quality"))
	if err != nil || (quality != 1 && quality != 3 && quality != 4 && quality != 5) {
		http.Error(w, "Invalid quality rating", http.StatusBadRequest)
		return
	}

	// Get current state
	uc, err := h.db.GetUserCard(ucID, sess.DBUserID)
	if err != nil {
		log.Printf("get user card for review: %v", err)
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	// Compute SM-2
	result := SM2(uc.EaseFactor, uc.IntervalDays, uc.Repetitions, quality)

	// Update DB
	if err := h.db.UpdateReview(ucID, result.EaseFactor, result.IntervalDays, result.Repetitions, result.NextReview); err != nil {
		log.Printf("update review: %v", err)
		http.Error(w, "Failed to save review", http.StatusInternalServerError)
		return
	}

	// Return next card
	h.ReviewSession(w, r)
}
