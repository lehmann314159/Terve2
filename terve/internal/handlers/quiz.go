package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/db"
	"github.com/lehmann314159/terve2/internal/ollama"
)

// --- Cloze & Sentence Translation data types ---

type ClozeQuestionData struct {
	Sentence                     string
	Lemma                        string
	Options                      []QuizOption
	CorrectValue                 string
	QuestionNum, Total, Score    int
	UsedIDs                      string
}

type SentenceTranslationQuestionData struct {
	Finnish                      string
	Lemma                        string
	Options                      []QuizOption
	CorrectValue                 string
	QuestionNum, Total, Score    int
	UsedIDs                      string
}

// --- Data types ---

type QuizHubData struct {
	PageData
	RecentResults []db.QuizResult
}

type QuizSessionData struct {
	PageData
	QuizType  string
	QuizSlug  string
	QuizTitle string
	Level     string // "A1-A2", "B1", "B2+" — only for case-id
}

type QuizOption struct {
	Value   string
	Display string
}

type CaseIDQuestionData struct {
	Word, Lemma, WordClass string
	Options                []QuizOption
	CorrectValue           string
	QuestionNum            int
	Total                  int
	Score                  int
	Level                  string
	UsedIDs                string
}

type FormEnglishQuestionData struct {
	Word, Lemma, WordClass, Morphology string
	Options                            []QuizOption
	CorrectValue                       string
	QuestionNum                        int
	Total                              int
	Score                              int
	UsedIDs                            string
}

type QuizAnswerData struct {
	Correct       bool
	SelectedValue string
	CorrectValue  string
	Word          string
	Explanation   string
	QuestionNum   int
	Total         int
	Score         int
	QuizSlug      string
	Level         string
	UsedIDs       string
}

type DeclensionQuestionData struct {
	Lemma, WordClass, TargetForm string
	Options                      []QuizOption
	CorrectValue                 string
	QuestionNum, Total, Score    int
	UsedIDs                      string
}

type ConjugationQuestionData struct {
	Lemma, TargetForm         string
	Options                   []QuizOption
	CorrectValue              string
	QuestionNum, Total, Score int
	UsedIDs                   string
}

type QuizResultsData struct {
	QuizType string
	QuizSlug string
	Total    int
	Correct  int
	Percent  int
	Recent   []db.QuizResult
}

// quizMaxAttempts is the maximum number of cards to try before giving up on
// generating a quiz question.
const quizMaxAttempts = 10

// --- Finnish cases for distractor generation ---

var finnishCases = []string{
	"nominative", "genitive", "partitive",
	"inessive", "elative", "illative",
	"adessive", "ablative", "allative",
	"essive", "translative", "abessive",
	"instructive", "comitative",
}

var finnishNumbers = []string{"singular", "plural"}

// cefrCaseTiers defines which cases appear at each CEFR difficulty level.
// Each tier is cumulative — B1 includes all A1-A2 cases, B2+ is all 14.
var cefrCaseTiers = map[string][]string{
	"A1-A2": {"nominative", "genitive", "partitive", "inessive", "elative", "illative"},
	"B1": {"nominative", "genitive", "partitive", "inessive", "elative", "illative",
		"adessive", "ablative", "allative", "essive", "translative"},
	"B2+": {"nominative", "genitive", "partitive", "inessive", "elative", "illative",
		"adessive", "ablative", "allative", "essive", "translative",
		"abessive", "instructive", "comitative"},
}

// parseUsedIDs parses a comma-separated string of card IDs into a slice.
func parseUsedIDs(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var ids []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if id, err := strconv.ParseInt(p, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// appendUsedID adds a card ID to the comma-separated used list.
func appendUsedID(used string, id int64) string {
	idStr := strconv.FormatInt(id, 10)
	if used == "" {
		return idStr
	}
	return used + "," + idStr
}

// caseInTier checks if a case name is in the allowed set.
func caseInTier(caseName string, allowed []string) bool {
	for _, c := range allowed {
		if strings.EqualFold(c, caseName) {
			return true
		}
	}
	return false
}

// --- Handlers ---

// QuizHub renders the quiz type selection page.
func (h *Handlers) QuizHub(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())

	recent, err := h.db.GetRecentQuizResults(sess.DBUserID, "", 10)
	if err != nil {
		log.Printf("get recent quiz results: %v", err)
	}

	h.render(w, "base", QuizHubData{
		PageData:      pageData(r, "Terve \u2014 Quiz", "quiz"),
		RecentResults: recent,
	})
}

// CaseIDPage renders the case identification quiz session page.
func (h *Handlers) CaseIDPage(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	if _, ok := cefrCaseTiers[level]; !ok {
		level = "B2+"
	}
	h.render(w, "base", QuizSessionData{
		PageData:  pageData(r, "Terve \u2014 Case Identification", "quiz-session"),
		QuizType:  "case_id",
		QuizSlug:  "case-id",
		QuizTitle: "Case Identification (" + level + ")",
		Level:     level,
	})
}

// CaseIDQuestion generates a single case identification question (HTMX partial).
func (h *Handlers) CaseIDQuestion(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	qNum, _ := strconv.Atoi(r.URL.Query().Get("q"))
	score, _ := strconv.Atoi(r.URL.Query().Get("s"))
	level := r.URL.Query().Get("level")
	used := r.URL.Query().Get("used")
	if qNum < 1 {
		qNum = 1
	}

	allowedCases, ok := cefrCaseTiers[level]
	if !ok {
		level = "B2+"
		allowedCases = cefrCaseTiers["B2+"]
	}

	excludeIDs := parseUsedIDs(used)

	// Try up to 10 cards to find one with a case in the allowed tier
	for attempt := 0; attempt < quizMaxAttempts; attempt++ {
		card, err := h.db.GetWeightedRandomUserCard(sess.DBUserID, excludeIDs)
		if err != nil {
			log.Printf("quiz: get weighted random card: %v", err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "You need flashcards in your deck to take quizzes. Add some cards first!",
			})
			return
		}

		analyses, err := h.voikko.AnalyzeWord(card.Finnish)
		if err != nil || len(analyses) == 0 {
			continue
		}

		a := analyses[0]
		if a.Case == "" {
			continue
		}

		// Check if the case is in the allowed CEFR tier
		if !caseInTier(a.Case, allowedCases) {
			continue
		}

		correctCase := a.Case
		correctNumber := a.Number
		correctValue := correctCase
		correctDisplay := correctCase
		if correctNumber != "" {
			correctValue = correctCase + " " + correctNumber
			correctDisplay = correctCase + " " + correctNumber
		}

		// Generate 3 distractors from allowed cases only
		distractors := generateCaseDistractors(correctCase, correctNumber, 3, allowedCases)

		// Build options: correct + distractors, then shuffle
		options := []QuizOption{{Value: correctValue, Display: correctDisplay}}
		for _, d := range distractors {
			options = append(options, QuizOption{Value: d, Display: d})
		}
		rand.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})

		h.renderPartial(w, "case-id-question", CaseIDQuestionData{
			Word:         card.Finnish,
			Lemma:        card.Lemma,
			WordClass:    a.WordClassEnglish,
			Options:      options,
			CorrectValue: correctValue,
			QuestionNum:  qNum,
			Total:        10,
			Score:        score,
			Level:        level,
			UsedIDs:      appendUsedID(used, card.ID),
		})
		return
	}

	// All attempts failed
	h.renderPartial(w, "quiz-error", map[string]string{
		"Error": "Could not find a suitable word for this question. Try adding more nouns or adjectives to your deck.",
	})
}

// CaseIDAnswer checks a case identification answer (HTMX partial).
func (h *Handlers) CaseIDAnswer(w http.ResponseWriter, r *http.Request) {
	selected := r.FormValue("selected")
	correct := r.FormValue("correct")
	word := r.FormValue("word")
	qNum, _ := strconv.Atoi(r.FormValue("q"))
	score, _ := strconv.Atoi(r.FormValue("s"))
	level := r.FormValue("level")
	used := r.FormValue("used")

	isCorrect := selected == correct
	if isCorrect {
		score++
	}

	explanation := fmt.Sprintf("The word \"%s\" is in the %s.", word, correct)

	h.renderPartial(w, "quiz-answer", QuizAnswerData{
		Correct:       isCorrect,
		SelectedValue: selected,
		CorrectValue:  correct,
		Word:          word,
		Explanation:   explanation,
		QuestionNum:   qNum,
		Total:         10,
		Score:         score,
		QuizSlug:      "case-id",
		Level:         level,
		UsedIDs:       used,
	})
}

// FormEnglishPage renders the form-to-English quiz session page.
func (h *Handlers) FormEnglishPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", QuizSessionData{
		PageData:  pageData(r, "Terve \u2014 Form to English", "quiz-session"),
		QuizType:  "form_english",
		QuizSlug:  "form-english",
		QuizTitle: "Form to English",
	})
}

// FormEnglishQuestion generates a form-to-English question (HTMX partial).
func (h *Handlers) FormEnglishQuestion(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	qNum, _ := strconv.Atoi(r.URL.Query().Get("q"))
	score, _ := strconv.Atoi(r.URL.Query().Get("s"))
	used := r.URL.Query().Get("used")
	if qNum < 1 {
		qNum = 1
	}

	excludeIDs := parseUsedIDs(used)

	card, err := h.db.GetWeightedRandomUserCardWithTranslation(sess.DBUserID, excludeIDs)
	if err != nil {
		log.Printf("quiz: get weighted random card with translation: %v", err)
		h.renderPartial(w, "quiz-error", map[string]string{
			"Error": "You need flashcards with translations in your deck. Add some cards first!",
		})
		return
	}

	// Get morphology string
	morphology := card.Morphology
	if morphology == "" {
		analyses, err := h.voikko.AnalyzeWord(card.Finnish)
		if err == nil && len(analyses) > 0 {
			a := analyses[0]
			var parts []string
			if a.WordClassEnglish != "" {
				parts = append(parts, a.WordClassEnglish)
			}
			if a.Case != "" {
				parts = append(parts, a.Case)
			}
			if a.Number != "" {
				parts = append(parts, a.Number)
			}
			morphology = strings.Join(parts, ", ")
		}
	}

	// Get 3 distractor cards with different translations
	distractors, err := h.db.GetRandomUserCards(sess.DBUserID, card.ID, 3)
	if err != nil {
		log.Printf("quiz: get distractor cards: %v", err)
	}

	// Build options
	options := []QuizOption{{Value: card.Translation, Display: card.Translation}}
	seen := map[string]bool{strings.ToLower(card.Translation): true}
	for _, d := range distractors {
		lower := strings.ToLower(d.Translation)
		if !seen[lower] {
			options = append(options, QuizOption{Value: d.Translation, Display: d.Translation})
			seen[lower] = true
		}
	}

	// If we don't have 4 options, that's OK — proceed with what we have
	rand.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})

	h.renderPartial(w, "form-english-question", FormEnglishQuestionData{
		Word:         card.Finnish,
		Lemma:        card.Lemma,
		WordClass:    card.WordClass,
		Morphology:   morphology,
		Options:      options,
		CorrectValue: card.Translation,
		QuestionNum:  qNum,
		Total:        10,
		Score:        score,
		UsedIDs:      appendUsedID(used, card.ID),
	})
}

// FormEnglishAnswer checks a form-to-English answer (HTMX partial).
func (h *Handlers) FormEnglishAnswer(w http.ResponseWriter, r *http.Request) {
	selected := r.FormValue("selected")
	correct := r.FormValue("correct")
	word := r.FormValue("word")
	qNum, _ := strconv.Atoi(r.FormValue("q"))
	score, _ := strconv.Atoi(r.FormValue("s"))
	used := r.FormValue("used")

	isCorrect := strings.EqualFold(selected, correct)
	if isCorrect {
		score++
	}

	explanation := fmt.Sprintf("\"%s\" means: %s", word, correct)

	h.renderPartial(w, "quiz-answer", QuizAnswerData{
		Correct:       isCorrect,
		SelectedValue: selected,
		CorrectValue:  correct,
		Word:          word,
		Explanation:   explanation,
		QuestionNum:   qNum,
		Total:         10,
		Score:         score,
		QuizSlug:      "form-english",
		UsedIDs:       used,
	})
}

// QuizResults saves the quiz score and shows results (HTMX partial).
func (h *Handlers) QuizResults(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	quizType := r.FormValue("quiz_type")
	quizSlug := r.FormValue("quiz_slug")
	total, _ := strconv.Atoi(r.FormValue("total"))
	correct, _ := strconv.Atoi(r.FormValue("correct"))

	if total == 0 {
		total = 10
	}

	// Save result
	if err := h.db.SaveQuizResult(sess.DBUserID, quizType, total, correct); err != nil {
		log.Printf("save quiz result: %v", err)
	}

	// Get recent results for this type
	recent, err := h.db.GetRecentQuizResults(sess.DBUserID, quizType, 5)
	if err != nil {
		log.Printf("get recent quiz results: %v", err)
	}

	percent := 0
	if total > 0 {
		percent = correct * 100 / total
	}

	h.renderPartial(w, "quiz-results", QuizResultsData{
		QuizType: quizType,
		QuizSlug: quizSlug,
		Total:    total,
		Correct:  correct,
		Percent:  percent,
		Recent:   recent,
	})
}

// --- Helpers ---

// generateCaseDistractors returns n random case+number strings different from the correct one.
// allowedCases restricts distractors to the given CEFR tier. If nil, all cases are used.
func generateCaseDistractors(correctCase, correctNumber string, n int, allowedCases []string) []string {
	type caseNum struct{ c, n string }
	correct := caseNum{correctCase, correctNumber}

	cases := finnishCases
	if len(allowedCases) > 0 {
		cases = allowedCases
	}

	var all []caseNum
	for _, c := range cases {
		for _, num := range finnishNumbers {
			cn := caseNum{c, num}
			if cn != correct {
				all = append(all, cn)
			}
		}
	}

	rand.Shuffle(len(all), func(i, j int) {
		all[i], all[j] = all[j], all[i]
	})

	var result []string
	for i := 0; i < n && i < len(all); i++ {
		if all[i].n != "" {
			result = append(result, all[i].c+" "+all[i].n)
		} else {
			result = append(result, all[i].c)
		}
	}
	return result
}

// --- Declension quiz ---

// nounCaseKeys lists all 28 paradigm keys for noun declension.
var nounCaseKeys = func() []string {
	var keys []string
	for _, c := range finnishCases {
		for _, n := range finnishNumbers {
			keys = append(keys, c+"_"+n)
		}
	}
	return keys
}()

// formatCaseKey converts "inessive_singular" to "inessive singular" for display.
func formatCaseKey(key string) string {
	return strings.ReplaceAll(key, "_", " ")
}

// getOrGenerateNounParadigm returns cached paradigm or generates one via Ollama,
// verifying forms with Voikko spell-check.
func (h *Handlers) getOrGenerateNounParadigm(lemma, wordClass string) (map[string]string, error) {
	forms, err := h.db.GetParadigm(lemma, wordClass, "")
	if err == nil {
		return forms, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Generate via Ollama
	forms, err = ollama.GenerateNounParadigm(h.ollama, lemma)
	if err != nil {
		return nil, err
	}

	// Verify each form with Voikko spell-checker, drop invalid ones
	verified := make(map[string]string)
	for key, form := range forms {
		valid, verr := h.voikko.ValidateWord(form)
		if verr != nil {
			log.Printf("voikko validate %q: %v", form, verr)
			// Keep the form if we can't reach Voikko (graceful degradation)
			verified[key] = form
			continue
		}
		if valid {
			verified[key] = form
		} else {
			log.Printf("paradigm: dropping invalid form %s=%q for lemma %q", key, form, lemma)
		}
	}

	if len(verified) > 0 {
		if err := h.db.SaveParadigm(lemma, wordClass, "", verified); err != nil {
			log.Printf("save paradigm cache: %v", err)
		}
	}

	return verified, nil
}

// DeclensionPage renders the declension quiz session page.
func (h *Handlers) DeclensionPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", QuizSessionData{
		PageData:  pageData(r, "Terve — Declension", "quiz-session"),
		QuizType:  "declension",
		QuizSlug:  "declension",
		QuizTitle: "Declension",
	})
}

// DeclensionQuestion generates a single declension question (HTMX partial).
func (h *Handlers) DeclensionQuestion(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	qNum, _ := strconv.Atoi(r.URL.Query().Get("q"))
	score, _ := strconv.Atoi(r.URL.Query().Get("s"))
	used := r.URL.Query().Get("used")
	if qNum < 1 {
		qNum = 1
	}

	excludeIDs := parseUsedIDs(used)

	for attempt := 0; attempt < quizMaxAttempts; attempt++ {
		card, err := h.db.GetWeightedRandomUserCard(sess.DBUserID, excludeIDs)
		if err != nil {
			log.Printf("declension quiz: get card: %v", err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "You need flashcards in your deck to take quizzes. Add some cards first!",
			})
			return
		}

		// Analyze to get lemma and word class
		analyses, err := h.voikko.AnalyzeWord(card.Finnish)
		if err != nil || len(analyses) == 0 {
			continue
		}
		a := analyses[0]

		// Only nouns and adjectives have case declension
		wc := strings.ToLower(a.WordClassEnglish)
		if wc != "noun" && wc != "adjective" {
			continue
		}

		lemma := a.Lemma
		if lemma == "" {
			lemma = card.Lemma
		}

		paradigm, err := h.getOrGenerateNounParadigm(lemma, wc)
		if err != nil {
			log.Printf("declension quiz: get paradigm for %q: %v", lemma, err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "Could not generate word forms. Make sure Ollama is running.",
			})
			return
		}

		// Need at least 4 forms for a good quiz
		if len(paradigm) < 4 {
			continue
		}

		// Collect available keys
		var availableKeys []string
		for _, key := range nounCaseKeys {
			if _, ok := paradigm[key]; ok {
				availableKeys = append(availableKeys, key)
			}
		}
		if len(availableKeys) < 4 {
			continue
		}

		// Pick a random target
		rand.Shuffle(len(availableKeys), func(i, j int) {
			availableKeys[i], availableKeys[j] = availableKeys[j], availableKeys[i]
		})
		targetKey := availableKeys[0]
		correctForm := paradigm[targetKey]

		// Build distractors: 3 other forms from the same paradigm
		var distractorForms []string
		seen := map[string]bool{strings.ToLower(correctForm): true}
		for _, key := range availableKeys[1:] {
			form := paradigm[key]
			lower := strings.ToLower(form)
			if !seen[lower] {
				distractorForms = append(distractorForms, form)
				seen[lower] = true
			}
			if len(distractorForms) >= 3 {
				break
			}
		}

		options := []QuizOption{{Value: correctForm, Display: correctForm}}
		for _, d := range distractorForms {
			options = append(options, QuizOption{Value: d, Display: d})
		}
		rand.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})

		h.renderPartial(w, "declension-question", DeclensionQuestionData{
			Lemma:        lemma,
			WordClass:    wc,
			TargetForm:   formatCaseKey(targetKey),
			Options:      options,
			CorrectValue: correctForm,
			QuestionNum:  qNum,
			Total:        10,
			Score:        score,
			UsedIDs:      appendUsedID(used, card.ID),
		})
		return
	}

	h.renderPartial(w, "quiz-error", map[string]string{
		"Error": "Could not find a suitable word. Try adding more nouns or adjectives to your deck.",
	})
}

// DeclensionAnswer checks a declension answer (HTMX partial).
func (h *Handlers) DeclensionAnswer(w http.ResponseWriter, r *http.Request) {
	selected := r.FormValue("selected")
	correct := r.FormValue("correct")
	word := r.FormValue("word")
	target := r.FormValue("target")
	qNum, _ := strconv.Atoi(r.FormValue("q"))
	score, _ := strconv.Atoi(r.FormValue("s"))
	used := r.FormValue("used")

	isCorrect := strings.EqualFold(selected, correct)
	if isCorrect {
		score++
	}

	explanation := fmt.Sprintf("'%s' is the %s of '%s'", correct, target, word)

	h.renderPartial(w, "quiz-answer", QuizAnswerData{
		Correct:       isCorrect,
		SelectedValue: selected,
		CorrectValue:  correct,
		Word:          word,
		Explanation:   explanation,
		QuestionNum:   qNum,
		Total:         10,
		Score:         score,
		QuizSlug:      "declension",
		UsedIDs:       used,
	})
}

// --- Conjugation quiz ---

// verbFormKeys lists all paradigm keys for verb conjugation.
var verbFormKeys = []string{
	"1st_singular_present", "2nd_singular_present", "3rd_singular_present",
	"1st_plural_present", "2nd_plural_present", "3rd_plural_present",
	"passive_present",
	"1st_singular_past", "2nd_singular_past", "3rd_singular_past",
	"1st_plural_past", "2nd_plural_past", "3rd_plural_past",
	"passive_past",
}

// formatVerbKey converts "3rd_singular_present" to "3rd person singular present" for display.
func formatVerbKey(key string) string {
	parts := strings.Split(key, "_")
	if len(parts) == 2 {
		// passive_present -> "passive present"
		return parts[0] + " " + parts[1]
	}
	if len(parts) == 3 {
		// 1st_singular_present -> "1st person singular present"
		return parts[0] + " person " + parts[1] + " " + parts[2]
	}
	return strings.ReplaceAll(key, "_", " ")
}

// getOrGenerateVerbParadigm returns cached paradigm or generates one via Ollama.
func (h *Handlers) getOrGenerateVerbParadigm(lemma string) (map[string]string, error) {
	forms, err := h.db.GetParadigm(lemma, "verb", "")
	if err == nil {
		return forms, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	forms, err = ollama.GenerateVerbParadigm(h.ollama, lemma)
	if err != nil {
		return nil, err
	}

	verified := make(map[string]string)
	for key, form := range forms {
		valid, verr := h.voikko.ValidateWord(form)
		if verr != nil {
			log.Printf("voikko validate %q: %v", form, verr)
			verified[key] = form
			continue
		}
		if valid {
			verified[key] = form
		} else {
			log.Printf("paradigm: dropping invalid form %s=%q for lemma %q", key, form, lemma)
		}
	}

	if len(verified) > 0 {
		if err := h.db.SaveParadigm(lemma, "verb", "", verified); err != nil {
			log.Printf("save verb paradigm cache: %v", err)
		}
	}

	return verified, nil
}

// ConjugationPage renders the conjugation quiz session page.
func (h *Handlers) ConjugationPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", QuizSessionData{
		PageData:  pageData(r, "Terve — Conjugation", "quiz-session"),
		QuizType:  "conjugation",
		QuizSlug:  "conjugation",
		QuizTitle: "Conjugation",
	})
}

// ConjugationQuestion generates a single conjugation question (HTMX partial).
func (h *Handlers) ConjugationQuestion(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	qNum, _ := strconv.Atoi(r.URL.Query().Get("q"))
	score, _ := strconv.Atoi(r.URL.Query().Get("s"))
	used := r.URL.Query().Get("used")
	if qNum < 1 {
		qNum = 1
	}

	excludeIDs := parseUsedIDs(used)

	for attempt := 0; attempt < quizMaxAttempts; attempt++ {
		card, err := h.db.GetWeightedRandomUserCard(sess.DBUserID, excludeIDs)
		if err != nil {
			log.Printf("conjugation quiz: get card: %v", err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "You need flashcards in your deck to take quizzes. Add some cards first!",
			})
			return
		}

		analyses, err := h.voikko.AnalyzeWord(card.Finnish)
		if err != nil || len(analyses) == 0 {
			continue
		}
		a := analyses[0]

		wc := strings.ToLower(a.WordClassEnglish)
		if wc != "verb" {
			continue
		}

		lemma := a.Lemma
		if lemma == "" {
			lemma = card.Lemma
		}

		paradigm, err := h.getOrGenerateVerbParadigm(lemma)
		if err != nil {
			log.Printf("conjugation quiz: get paradigm for %q: %v", lemma, err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "Could not generate verb forms. Make sure Ollama is running.",
			})
			return
		}

		if len(paradigm) < 4 {
			continue
		}

		var availableKeys []string
		for _, key := range verbFormKeys {
			if _, ok := paradigm[key]; ok {
				availableKeys = append(availableKeys, key)
			}
		}
		if len(availableKeys) < 4 {
			continue
		}

		rand.Shuffle(len(availableKeys), func(i, j int) {
			availableKeys[i], availableKeys[j] = availableKeys[j], availableKeys[i]
		})
		targetKey := availableKeys[0]
		correctForm := paradigm[targetKey]

		var distractorForms []string
		seen := map[string]bool{strings.ToLower(correctForm): true}
		for _, key := range availableKeys[1:] {
			form := paradigm[key]
			lower := strings.ToLower(form)
			if !seen[lower] {
				distractorForms = append(distractorForms, form)
				seen[lower] = true
			}
			if len(distractorForms) >= 3 {
				break
			}
		}

		options := []QuizOption{{Value: correctForm, Display: correctForm}}
		for _, d := range distractorForms {
			options = append(options, QuizOption{Value: d, Display: d})
		}
		rand.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})

		h.renderPartial(w, "conjugation-question", ConjugationQuestionData{
			Lemma:        lemma,
			TargetForm:   formatVerbKey(targetKey),
			Options:      options,
			CorrectValue: correctForm,
			QuestionNum:  qNum,
			Total:        10,
			Score:        score,
			UsedIDs:      appendUsedID(used, card.ID),
		})
		return
	}

	h.renderPartial(w, "quiz-error", map[string]string{
		"Error": "Could not find a suitable verb. Try adding more verbs to your deck.",
	})
}

// ConjugationAnswer checks a conjugation answer (HTMX partial).
func (h *Handlers) ConjugationAnswer(w http.ResponseWriter, r *http.Request) {
	selected := r.FormValue("selected")
	correct := r.FormValue("correct")
	word := r.FormValue("word")
	target := r.FormValue("target")
	qNum, _ := strconv.Atoi(r.FormValue("q"))
	score, _ := strconv.Atoi(r.FormValue("s"))
	used := r.FormValue("used")

	isCorrect := strings.EqualFold(selected, correct)
	if isCorrect {
		score++
	}

	explanation := fmt.Sprintf("'%s' is the %s form of '%s'", correct, target, word)

	h.renderPartial(w, "quiz-answer", QuizAnswerData{
		Correct:       isCorrect,
		SelectedValue: selected,
		CorrectValue:  correct,
		Word:          word,
		Explanation:   explanation,
		QuestionNum:   qNum,
		Total:         10,
		Score:         score,
		QuizSlug:      "conjugation",
		UsedIDs:       used,
	})
}

// --- Cloze quiz ---

// getOrGenerateSentences returns cached sentences for a lemma, or generates
// them via Ollama on a cache miss and saves them to the DB.
func (h *Handlers) getOrGenerateSentences(lemma, wordClass string) ([]db.CachedSentence, error) {
	cached, err := h.db.GetSentencesByLemma(lemma)
	if err != nil {
		return nil, err
	}
	if len(cached) > 0 {
		return cached, nil
	}

	entries, err := ollama.GenerateSentences(h.ollama, lemma, wordClass)
	if err != nil {
		return nil, err
	}

	var sentences []db.CachedSentence
	for _, e := range entries {
		sentences = append(sentences, db.CachedSentence{
			Lemma:      lemma,
			Finnish:    e.Finnish,
			English:    e.English,
			TargetForm: e.TargetForm,
		})
	}

	if err := h.db.SaveSentences(sentences); err != nil {
		log.Printf("save sentence cache: %v", err)
	}

	// Re-fetch to get IDs
	return h.db.GetSentencesByLemma(lemma)
}

// blankTargetForm replaces the first case-insensitive occurrence of targetForm
// in sentence with "___". Returns the blanked sentence and true, or the
// original sentence and false if not found.
func blankTargetForm(sentence, targetForm string) (string, bool) {
	lower := strings.ToLower(sentence)
	target := strings.ToLower(targetForm)
	idx := strings.Index(lower, target)
	if idx < 0 {
		return sentence, false
	}
	return sentence[:idx] + "___" + sentence[idx+len(targetForm):], true
}

// ClozePage renders the cloze quiz session page.
func (h *Handlers) ClozePage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", QuizSessionData{
		PageData:  pageData(r, "Terve — Cloze", "quiz-session"),
		QuizType:  "cloze",
		QuizSlug:  "cloze",
		QuizTitle: "Cloze",
	})
}

// ClozeQuestion generates a single cloze question (HTMX partial).
func (h *Handlers) ClozeQuestion(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	qNum, _ := strconv.Atoi(r.URL.Query().Get("q"))
	score, _ := strconv.Atoi(r.URL.Query().Get("s"))
	used := r.URL.Query().Get("used")
	if qNum < 1 {
		qNum = 1
	}

	excludeIDs := parseUsedIDs(used)

	for attempt := 0; attempt < quizMaxAttempts; attempt++ {
		card, err := h.db.GetWeightedRandomUserCard(sess.DBUserID, excludeIDs)
		if err != nil {
			log.Printf("cloze quiz: get card: %v", err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "You need flashcards in your deck to take quizzes. Add some cards first!",
			})
			return
		}

		wordClass := card.WordClass
		if wordClass == "" {
			wordClass = "word"
		}
		lemma := card.Lemma

		sentences, err := h.getOrGenerateSentences(lemma, wordClass)
		if err != nil {
			log.Printf("cloze quiz: get sentences for %q: %v", lemma, err)
			excludeIDs = append(excludeIDs, card.ID)
			continue
		}

		if len(sentences) == 0 {
			excludeIDs = append(excludeIDs, card.ID)
			continue
		}

		// Pick a random sentence
		s := sentences[rand.IntN(len(sentences))]
		blanked, ok := blankTargetForm(s.Finnish, s.TargetForm)
		if !ok {
			excludeIDs = append(excludeIDs, card.ID)
			continue
		}

		// Build distractors: prefer same word class from user's deck
		distractorCards, _ := h.db.GetRandomUserCards(sess.DBUserID, card.ID, 6)
		options := []QuizOption{{Value: s.TargetForm, Display: s.TargetForm}}
		seen := map[string]bool{strings.ToLower(s.TargetForm): true}

		// Prefer same word class
		for _, d := range distractorCards {
			if len(options) >= 4 {
				break
			}
			lower := strings.ToLower(d.Finnish)
			if !seen[lower] && strings.EqualFold(d.WordClass, wordClass) {
				options = append(options, QuizOption{Value: d.Finnish, Display: d.Finnish})
				seen[lower] = true
			}
		}
		// Fill remaining with any word class
		for _, d := range distractorCards {
			if len(options) >= 4 {
				break
			}
			lower := strings.ToLower(d.Finnish)
			if !seen[lower] {
				options = append(options, QuizOption{Value: d.Finnish, Display: d.Finnish})
				seen[lower] = true
			}
		}

		rand.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})

		h.renderPartial(w, "cloze-question", ClozeQuestionData{
			Sentence:     blanked,
			Lemma:        lemma,
			Options:      options,
			CorrectValue: s.TargetForm,
			QuestionNum:  qNum,
			Total:        10,
			Score:        score,
			UsedIDs:      appendUsedID(used, card.ID),
		})
		return
	}

	h.renderPartial(w, "quiz-error", map[string]string{
		"Error": "Could not generate a cloze question. Make sure Ollama is running and you have cards in your deck.",
	})
}

// ClozeAnswer checks a cloze answer (HTMX partial).
func (h *Handlers) ClozeAnswer(w http.ResponseWriter, r *http.Request) {
	selected := r.FormValue("selected")
	correct := r.FormValue("correct")
	word := r.FormValue("word")
	qNum, _ := strconv.Atoi(r.FormValue("q"))
	score, _ := strconv.Atoi(r.FormValue("s"))
	used := r.FormValue("used")

	isCorrect := strings.EqualFold(selected, correct)
	if isCorrect {
		score++
	}

	explanation := fmt.Sprintf("The missing word is \"%s\" (lemma: %s)", correct, word)

	h.renderPartial(w, "quiz-answer", QuizAnswerData{
		Correct:       isCorrect,
		SelectedValue: selected,
		CorrectValue:  correct,
		Word:          word,
		Explanation:   explanation,
		QuestionNum:   qNum,
		Total:         10,
		Score:         score,
		QuizSlug:      "cloze",
		UsedIDs:       used,
	})
}

// --- Sentence Translation quiz ---

// SentenceTranslationPage renders the sentence translation quiz session page.
func (h *Handlers) SentenceTranslationPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", QuizSessionData{
		PageData:  pageData(r, "Terve — Sentence Translation", "quiz-session"),
		QuizType:  "sentence_translation",
		QuizSlug:  "sentence-translation",
		QuizTitle: "Sentence Translation",
	})
}

// SentenceTranslationQuestion generates a sentence translation question (HTMX partial).
func (h *Handlers) SentenceTranslationQuestion(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r.Context())
	qNum, _ := strconv.Atoi(r.URL.Query().Get("q"))
	score, _ := strconv.Atoi(r.URL.Query().Get("s"))
	used := r.URL.Query().Get("used")
	if qNum < 1 {
		qNum = 1
	}

	excludeIDs := parseUsedIDs(used)

	for attempt := 0; attempt < quizMaxAttempts; attempt++ {
		card, err := h.db.GetWeightedRandomUserCard(sess.DBUserID, excludeIDs)
		if err != nil {
			log.Printf("sentence translation quiz: get card: %v", err)
			h.renderPartial(w, "quiz-error", map[string]string{
				"Error": "You need flashcards in your deck to take quizzes. Add some cards first!",
			})
			return
		}

		wordClass := card.WordClass
		if wordClass == "" {
			wordClass = "word"
		}
		lemma := card.Lemma

		sentences, err := h.getOrGenerateSentences(lemma, wordClass)
		if err != nil {
			log.Printf("sentence translation quiz: get sentences for %q: %v", lemma, err)
			excludeIDs = append(excludeIDs, card.ID)
			continue
		}

		if len(sentences) == 0 {
			excludeIDs = append(excludeIDs, card.ID)
			continue
		}

		// Pick a random sentence
		s := sentences[rand.IntN(len(sentences))]

		// Build distractors: cached translations from other lemmas
		options := []QuizOption{{Value: s.English, Display: s.English}}
		seen := map[string]bool{strings.ToLower(s.English): true}

		otherSentences, _ := h.db.GetRandomSentencesExcludingLemma(lemma, 6)
		for _, os := range otherSentences {
			if len(options) >= 4 {
				break
			}
			lower := strings.ToLower(os.English)
			if !seen[lower] {
				options = append(options, QuizOption{Value: os.English, Display: os.English})
				seen[lower] = true
			}
		}

		// Fall back to card translations if we don't have enough distractors
		if len(options) < 4 {
			distractorCards, _ := h.db.GetRandomUserCards(sess.DBUserID, card.ID, 6)
			for _, d := range distractorCards {
				if len(options) >= 4 {
					break
				}
				lower := strings.ToLower(d.Translation)
				if d.Translation != "" && !seen[lower] {
					options = append(options, QuizOption{Value: d.Translation, Display: d.Translation})
					seen[lower] = true
				}
			}
		}

		rand.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})

		h.renderPartial(w, "sentence-translation-question", SentenceTranslationQuestionData{
			Finnish:      s.Finnish,
			Lemma:        lemma,
			Options:      options,
			CorrectValue: s.English,
			QuestionNum:  qNum,
			Total:        10,
			Score:        score,
			UsedIDs:      appendUsedID(used, card.ID),
		})
		return
	}

	h.renderPartial(w, "quiz-error", map[string]string{
		"Error": "Could not generate a translation question. Make sure Ollama is running and you have cards in your deck.",
	})
}

// SentenceTranslationAnswer checks a sentence translation answer (HTMX partial).
func (h *Handlers) SentenceTranslationAnswer(w http.ResponseWriter, r *http.Request) {
	selected := r.FormValue("selected")
	correct := r.FormValue("correct")
	word := r.FormValue("word")
	qNum, _ := strconv.Atoi(r.FormValue("q"))
	score, _ := strconv.Atoi(r.FormValue("s"))
	used := r.FormValue("used")

	isCorrect := strings.EqualFold(selected, correct)
	if isCorrect {
		score++
	}

	explanation := fmt.Sprintf("The sentence means: \"%s\"", correct)

	h.renderPartial(w, "quiz-answer", QuizAnswerData{
		Correct:       isCorrect,
		SelectedValue: selected,
		CorrectValue:  correct,
		Word:          word,
		Explanation:   explanation,
		QuestionNum:   qNum,
		Total:         10,
		Score:         score,
		QuizSlug:      "sentence-translation",
		UsedIDs:       used,
	})
}
