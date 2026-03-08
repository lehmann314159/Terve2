package handlers

import (
	"log"
	"net/http"

	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// AnalysisData is passed to the analysis partial template.
type AnalysisData struct {
	Text        string
	Context     string
	Tokens      []voikko.TokenAnalysis
	VoikkoError string
}

// ExplainData is passed to the explanation partial template.
type ExplainData struct {
	Translation string
	Explanation string
	OllamaError string
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

	h.renderPartial(w, "analysis", AnalysisData{
		Text:        text,
		Context:     context,
		Tokens:      tokens,
		VoikkoError: voikkoErr,
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
	h.renderPartial(w, "explanation", ExplainData{
		Translation: translation,
		Explanation: explanation,
	})
}
