package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/language"
)

// AnalysisData is passed to the analysis partial template.
type AnalysisData struct {
	Text              string
	Context           string
	Tokens            []language.Token
	AnalysisError     string
	LemmaTranslations map[string]string
	LoggedIn          bool
	SavedLemmas       map[string]int64 // lemma → user_card_id
}

// ExplainData is passed to the explanation partial template.
type ExplainData struct {
	Translation string
	Explanation string
	OllamaError string
	// Fields for the save-as-flashcard button
	LoggedIn    bool
	Text        string
	Context     string
	Lemma       string
	WordClass   string
	Morphology  string
}

// Analyze handles POST /analyze — returns morphology immediately.
func (h *Handlers) Analyze(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	context := r.FormValue("context")

	if text == "" {
		h.renderPartial(w, "analysis", AnalysisData{
			AnalysisError: "No text selected.",
		})
		return
	}

	result, err := h.driverFor(r).Analyze(text)
	if err != nil {
		log.Printf("Driver analyze error: %v", err)
		h.renderPartial(w, "analysis", AnalysisData{
			AnalysisError: "Morphological analysis unavailable.",
		})
		return
	}

	// Look up English translations for lemmas from the cards table
	var lemmas []string
	for _, tok := range result.Tokens {
		lemmas = append(lemmas, tok.Lemma)
	}
	translations := h.db.LookupLemmaTranslations(lemmas)

	sess := auth.GetSession(r.Context())
	var savedLemmas map[string]int64
	loggedIn := sess != nil
	if loggedIn {
		savedLemmas = h.db.LookupUserCardIDsByLemmas(sess.DBUserID, lemmas)
	}

	h.renderPartial(w, "analysis", AnalysisData{
		Text:              text,
		Context:           context,
		Tokens:            result.Tokens,
		AnalysisError:     result.Error,
		LemmaTranslations: translations,
		LoggedIn:          loggedIn,
		SavedLemmas:       savedLemmas,
	})
}

// Explain handles POST /explain — returns translation + explanation from Ollama.
func (h *Handlers) Explain(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	context := r.FormValue("context")

	if text == "" {
		h.renderPartial(w, "explanation", ExplainData{
			OllamaError: "No text provided.",
		})
		return
	}

	// Re-run driver to get tokens for the prompt (instant)
	drv := h.driverFor(r)
	result, _ := drv.Analyze(text)
	if result == nil {
		result = &language.AnalysisResult{}
	}

	prompt := drv.BuildPrompt(text, context, result)
	response, err := h.ollama.Generate(drv.SystemPrompt(), prompt)
	if err != nil {
		log.Printf("Ollama error: %v", err)
		h.renderPartial(w, "explanation", ExplainData{
			OllamaError: "LLM response unavailable.",
		})
		return
	}

	translation, explanation := drv.ParseResponse(response)

	// Gather data for the save-as-flashcard button
	sess := auth.GetSession(r.Context())
	var lemma, wordClass, morphology string
	if len(result.Tokens) > 0 {
		tok := result.Tokens[0]
		lemma = tok.Lemma
		wordClass = tok.POS
		if len(tok.Features) > 0 {
			morphology = strings.Join(tok.Features, ", ")
		}
	}

	h.renderPartial(w, "explanation", ExplainData{
		Translation: translation,
		Explanation: explanation,
		LoggedIn:    sess != nil,
		Text:        text,
		Context:     context,
		Lemma:       lemma,
		WordClass:   wordClass,
		Morphology:  morphology,
	})
}
