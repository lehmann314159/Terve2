package handlers

import (
	"log"
	"net/http"

	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// AnalysisData is passed to the analysis partial template.
type AnalysisData struct {
	Text           string
	Context        string
	Translation    string
	Tokens         []voikko.TokenAnalysis
	Explanation    string
	VoikkoError    string
	OllamaError    string
}

// Analyze handles POST /analyze — the core interaction.
func (h *Handlers) Analyze(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	context := r.FormValue("context")

	if text == "" {
		h.renderPartial(w, "analysis", AnalysisData{
			VoikkoError: "No text selected.",
		})
		return
	}

	// Step 1: Voikko analysis
	var tokens []voikko.TokenAnalysis
	var voikkoErr string

	sv, err := h.voikko.ValidateSentence(text)
	if err != nil {
		log.Printf("Voikko error: %v", err)
		voikkoErr = "Morphological analysis unavailable."
	} else {
		tokens = sv.Tokens
	}

	// Step 2: Single Ollama call for translation + explanation
	var translation, explanation string
	var ollamaErr string

	prompt := ollama.BuildPrompt(text, context, tokens)
	response, err := h.ollama.Generate(ollama.SystemPrompt, prompt)
	if err != nil {
		log.Printf("Ollama error: %v", err)
		ollamaErr = "LLM response unavailable."
	} else {
		translation, explanation = ollama.ParseResponse(response)
	}

	h.renderPartial(w, "analysis", AnalysisData{
		Text:        text,
		Context:     context,
		Translation: translation,
		Tokens:      tokens,
		Explanation: explanation,
		VoikkoError: voikkoErr,
		OllamaError: ollamaErr,
	})
}
