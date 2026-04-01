package ollama

import (
	"fmt"
	"strings"

	"github.com/lehmann314159/terve2/internal/voikko"
)

// SystemPrompt is the instruction set for the LLM.
const SystemPrompt = `You are a Finnish language tutor helping an English-speaking learner understand Finnish text. You receive:
1. A Finnish word or phrase the learner selected
2. Morphological analysis from Voikko (a Finnish NLP tool)
3. The sentence context where the word appeared

Respond in exactly this format:

TRANSLATION: <English translation of the selected text>

EXPLANATION: <grammatical explanation>

Rules for TRANSLATION:
- Translate just the selected Finnish text into natural English
- One line, no quotes

Rules for EXPLANATION:
- Explain the grammatical form using the Voikko analysis as ground truth
- If it's a declined/conjugated form, explain how it relates to the base form (lemma)
- Keep it concise — 2-4 sentences
- Use simple grammatical terminology
- If multiple analyses exist, explain the most likely one given the context
- Do NOT repeat the raw Voikko data — synthesize it into a natural explanation`

// BuildPrompt constructs the user prompt from selected text, Voikko analysis, and context.
func BuildPrompt(text, context string, tokens []voikko.TokenAnalysis) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Selected text: %q\n", text)
	fmt.Fprintf(&b, "Sentence context: %q\n\n", context)

	b.WriteString("Voikko morphological analysis:\n")
	for _, tok := range tokens {
		if tok.Type != "word" {
			continue
		}
		for _, a := range tok.Analyses {
			fmt.Fprintf(&b, "- %s: lemma=%s, class=%s", tok.Token, a.Lemma, a.WordClass)
			if a.Case != "" {
				fmt.Fprintf(&b, ", case=%s", a.Case)
			}
			if a.Number != "" {
				fmt.Fprintf(&b, ", number=%s", a.Number)
			}
			if a.Person != "" {
				fmt.Fprintf(&b, ", person=%s", a.Person)
			}
			if a.Tense != "" {
				fmt.Fprintf(&b, ", tense=%s", a.Tense)
			}
			if a.Mood != "" {
				fmt.Fprintf(&b, ", mood=%s", a.Mood)
			}
			if a.Possessive != "" {
				fmt.Fprintf(&b, ", possessive=%s", a.Possessive)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\nRespond with TRANSLATION and EXPLANATION as specified.")
	return b.String()
}

// DifficultySystemPrompt instructs the LLM to assess CEFR level.
const DifficultySystemPrompt = `You are a Finnish language difficulty assessor. When given a Finnish text sample, respond with only the CEFR level that best describes the reading difficulty for a Finnish language learner. Use exactly one of: A1, A2, B1, B2, C1, C2. No other text.`

// BuildDifficultyPrompt builds the prompt for CEFR difficulty estimation from a text sample.
func BuildDifficultyPrompt(sample string) string {
	return fmt.Sprintf("Rate the CEFR difficulty of this Finnish text:\n\n%s", sample)
}

// ParseDifficultyResponse extracts a CEFR level from the LLM response.
func ParseDifficultyResponse(response string) string {
	for _, level := range []string{"A1", "A2", "B1", "B2", "C1", "C2"} {
		if strings.Contains(strings.ToUpper(response), level) {
			return level
		}
	}
	return ""
}

// ParseResponse splits the LLM response into translation and explanation parts.
func ParseResponse(response string) (translation, explanation string) {
	lines := strings.Split(response, "\n")
	var transLines, explLines []string
	section := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "TRANSLATION:") {
			section = "translation"
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "TRANSLATION:"))
			if rest != "" {
				transLines = append(transLines, rest)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "EXPLANATION:") {
			section = "explanation"
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "EXPLANATION:"))
			if rest != "" {
				explLines = append(explLines, rest)
			}
			continue
		}
		switch section {
		case "translation":
			if trimmed != "" {
				transLines = append(transLines, trimmed)
			}
		case "explanation":
			explLines = append(explLines, line)
		}
	}

	translation = strings.TrimSpace(strings.Join(transLines, " "))
	explanation = strings.TrimSpace(strings.Join(explLines, "\n"))

	// Fallback: if parsing failed, put everything in explanation
	if translation == "" && explanation == "" {
		explanation = strings.TrimSpace(response)
	}
	return
}
