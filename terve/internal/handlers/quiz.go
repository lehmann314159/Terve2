package handlers

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/db"
)

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

type QuizResultsData struct {
	QuizType string
	QuizSlug string
	Total    int
	Correct  int
	Percent  int
	Recent   []db.QuizResult
}

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
	for attempt := 0; attempt < 10; attempt++ {
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
