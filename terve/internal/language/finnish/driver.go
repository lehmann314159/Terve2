package finnish

import (
	"fmt"
	"strings"

	"github.com/lehmann314159/terve2/internal/language"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// Driver implements language.Driver for Finnish using the Voikko sidecar.
type Driver struct {
	voikko *voikko.Client
}

// New creates a Finnish Driver that calls the Voikko sidecar at voikkoURL.
func New(voikkoURL string) *Driver {
	return &Driver{voikko: voikko.NewClient(voikkoURL)}
}

func (d *Driver) Language() string { return "Finnish" }

func (d *Driver) Analyze(text string) (*language.AnalysisResult, error) {
	sv, err := d.voikko.ValidateSentence(text)
	if err != nil {
		return &language.AnalysisResult{Error: "Morphological analysis unavailable."}, nil
	}
	var tokens []language.Token
	for _, t := range sv.Tokens {
		if t.Type != "word" {
			continue
		}
		for _, a := range t.Analyses {
			tok := language.Token{
				Surface: t.Token,
				Lemma:   a.Lemma,
				POS:     normalizeWordClass(a.WordClass, a.WordClassEnglish),
			}
			if a.Case != ""       { tok.Features = append(tok.Features, "Case="+a.Case) }
			if a.Number != ""     { tok.Features = append(tok.Features, "Number="+a.Number) }
			if a.Person != ""     { tok.Features = append(tok.Features, "Person="+a.Person) }
			if a.Tense != ""      { tok.Features = append(tok.Features, "Tense="+a.Tense) }
			if a.Mood != ""       { tok.Features = append(tok.Features, "Mood="+a.Mood) }
			if a.Possessive != "" { tok.Features = append(tok.Features, "Possessive="+a.Possessive) }
			tokens = append(tokens, tok)
		}
	}
	return &language.AnalysisResult{Tokens: tokens}, nil
}

// normalizeWordClass returns a clean English label. Voikko returns Finnish
// word class names; this converts the common ones to English equivalents.
func normalizeWordClass(fi, en string) string {
	if en != "" {
		return en
	}
	switch fi {
	case "nimisana":   return "noun"
	case "teonsana":   return "verb"
	case "laatusana":  return "adjective"
	case "asemosana":  return "pronoun"
	case "seikkasana": return "adverb"
	case "sidesana":   return "conjunction"
	case "suhdesana":  return "adposition"
	default:           return fi
	}
}

func (d *Driver) SystemPrompt() string { return systemPrompt }

const systemPrompt = `You are a Finnish language tutor helping an English-speaking learner understand Finnish text. You receive:
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
- Do NOT repeat the raw Voikko data — synthesize it into a natural explanation

Critical rules — never violate these:
- Adessive possession (minulla on): NEVER say the adessive means "at me" or frame it as spatial. It is grammaticalized possession. "Minulla on auto" means "I have a car" — Finnish uses adessive + olla for possession, not a verb meaning "to have."
- Locative cases: NEVER use "interior vs. exterior" or "inside vs. surface" as the primary framing for -ssa vs. -lla. Case choice is lexically determined for institutions, cities, and abstracts ("koulussa", "töissä", "Helsingissä"). Explain the spatial framing only when genuinely applicable and note immediately that many nouns don't follow it.
- Finnish passive: NEVER say it means "it is done" or implies no agent. The Finnish passive (-taan/-tään) always implies an unspecified human agent — closer to French "on" or English "one." "Syödään" means "people eat" or "one eats," not "it is eaten."
- Partitive vs. accusative: NEVER use "incomplete vs. complete action" as the primary explanation. Partitive = ongoing, unbounded, or partial relationship; accusative = bounded, total, resultative. With negation, partitive is always required — this is a grammatical rule, not a semantic claim.
- You MAY override Voikko's analysis if sentence context makes a different reading clearly correct. State explicitly that you are doing so and why.`

func (d *Driver) BuildPrompt(text, context string, result *language.AnalysisResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Selected text: %q\n", text)
	fmt.Fprintf(&b, "Sentence context: %q\n\n", context)
	b.WriteString("Morphological analysis:\n")
	for _, tok := range result.Tokens {
		fmt.Fprintf(&b, "- %s: lemma=%s, pos=%s", tok.Surface, tok.Lemma, tok.POS)
		for _, f := range tok.Features {
			k, v, _ := strings.Cut(f, "=")
			fmt.Fprintf(&b, ", %s=%s", strings.ToLower(k), v)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nRespond with TRANSLATION and EXPLANATION as specified. /no_think")
	return b.String()
}

func (d *Driver) ParseResponse(response string) (translation, explanation string) {
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

	if translation == "" && explanation == "" {
		explanation = strings.TrimSpace(response)
	}
	return
}
