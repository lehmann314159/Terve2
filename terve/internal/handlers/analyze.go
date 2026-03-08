package handlers

import (
	"log"
	"net/http"

	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// AnalysisData is passed to the analysis partial template.
type AnalysisData struct {
	Text              string
	Context           string
	Tokens            []voikko.TokenAnalysis
	VoikkoError       string
	LemmaTranslations map[string]string
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
			VoikkoError: "No text selected.",
		})
		return
	}

	var tokens []voikko.TokenAnalysis
	var voikkoErr string

	sv, err := h.voikko.ValidateSentence(text)
	if err != nil {
		log.Printf("Voikko error: %v", err)
		voikkoErr = "Morphological analysis unavailable."
	} else {
		tokens = sv.Tokens
	}

	// Look up English translations for lemmas from the cards table
	var lemmas []string
	for _, t := range tokens {
		if t.Type == "word" {
			for _, a := range t.Analyses {
				lemmas = append(lemmas, a.Lemma)
			}
		}
	}
	translations := h.db.LookupLemmaTranslations(lemmas)

	h.renderPartial(w, "analysis", AnalysisData{
		Text:              text,
		Context:           context,
		Tokens:            tokens,
		VoikkoError:       voikkoErr,
		LemmaTranslations: translations,
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

	// Re-run Voikko to get tokens for the prompt (instant)
	var tokens []voikko.TokenAnalysis
	sv, err := h.voikko.ValidateSentence(text)
	if err == nil {
		tokens = sv.Tokens
	}

	prompt := ollama.BuildPrompt(text, context, tokens)
	response, err := h.ollama.Generate(ollama.SystemPrompt, prompt)
	if err != nil {
		log.Printf("Ollama error: %v", err)
		h.renderPartial(w, "explanation", ExplainData{
			OllamaError: "LLM response unavailable.",
		})
		return
	}

	translation, explanation := ollama.ParseResponse(response)

	// Gather data for the save-as-flashcard button
	sess := auth.GetSession(r.Context())
	var lemma, wordClass, morphology string
	if len(tokens) > 0 {
		// Use first word token's first analysis
		for _, t := range tokens {
			if t.Type == "word" && len(t.Analyses) > 0 {
				a := t.Analyses[0]
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
				break
			}
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
