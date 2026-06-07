package spanish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lehmann314159/terve2/internal/language"
)

// Driver implements language.Driver for Spanish using the Spanish sidecar.
type Driver struct {
	baseURL string
	http    *http.Client
}

// New creates a Spanish Driver that calls the sidecar at sidecarURL.
func New(sidecarURL string) *Driver {
	return &Driver{
		baseURL: sidecarURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *Driver) Language() string { return "Spanish" }

// sidecarToken is the JSON shape returned by the Spanish sidecar.
type sidecarToken struct {
	Surface  string   `json:"surface"`
	Lemma    string   `json:"lemma"`
	POS      string   `json:"pos"`
	Features []string `json:"features"`
}

func (d *Driver) Analyze(text string) (*language.AnalysisResult, error) {
	payload, _ := json.Marshal(map[string]string{"text": text})
	resp, err := d.http.Post(d.baseURL+"/analyze", "application/json",
		bytes.NewReader(payload))
	if err != nil {
		return &language.AnalysisResult{Error: "Morphological analysis unavailable."}, nil
	}
	defer resp.Body.Close()

	var raw []sidecarToken
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return &language.AnalysisResult{Error: "Analysis decode error."}, nil
	}

	tokens := make([]language.Token, 0, len(raw))
	for _, t := range raw {
		tokens = append(tokens, language.Token{
			Surface:  t.Surface,
			Lemma:    t.Lemma,
			POS:      t.POS,
			Features: t.Features,
		})
	}
	return &language.AnalysisResult{Tokens: tokens}, nil
}

func (d *Driver) SystemPrompt() string { return systemPrompt }

const systemPrompt = `You are the explanation layer in a Spanish reading comprehension app for beginners (A1-B1 CEFR).

Your role: the tool layer has already performed morphological analysis. You receive the tool output and produce a concise panel for the learner. You do NOT generate morphological claims from your own knowledge — you contextualize, select among alternatives, and explain what the tool found.

Rules:
1. If the tool provides multiple conjugation matches or ambiguous readings, select the one that fits the sentence context and explain why. State all plausible readings for ambiguous forms.
2. You MAY override the tool's mood or lemma tag if context makes a different reading clearly correct. If you do, state explicitly that you are overriding and why.
3. SER vs. ESTAR — this is critical. NEVER say "temporary" or "permanent." These words are wrong and misleading.
   - Use SER for intrinsic identity, origin, material, or classification: what something IS.
     Example: "La puerta es de madera" → EXPLANATION: Use ser to describe what something is made of — a defining characteristic.
   - Use ESTAR for resultant states or conditions — how something has come to be at this moment.
     Example: "La puerta está abierta" → EXPLANATION: Use estar for a resultant condition — the door has been opened and is now in that state.
   If you write "temporary" or "permanent" in any EXPLANATION, your response is wrong.
4. For enclitic decomposer output, explain the base verb and each clitic component separately.

Output format — three fields, no headers, no preamble:
TRANSLATION: [English translation of the input]
FORM: [morphological label: what form is this and what verb/word is it from]
EXPLANATION: [one sentence a beginner can apply to a new sentence]

Keep the total response under 80 words.`

// BuildPrompt constructs the Spanish vetting prompt.
func (d *Driver) BuildPrompt(text, context string, result *language.AnalysisResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Selected text: %q\n", text)
	fmt.Fprintf(&b, "Sentence context: %q\n\n", context)
	b.WriteString("Tool analysis:\n")
	for _, tok := range result.Tokens {
		fmt.Fprintf(&b, "- %s: lemma=%s, pos=%s", tok.Surface, tok.Lemma, tok.POS)
		for _, f := range tok.Features {
			k, v, _ := strings.Cut(f, "=")
			fmt.Fprintf(&b, ", %s=%s", strings.ToLower(k), v)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nRespond with TRANSLATION, FORM, and EXPLANATION as specified.")
	return b.String()
}

// ParseResponse handles the Spanish three-field format:
// TRANSLATION: ...
// FORM: ...
// EXPLANATION: ...
// FORM is prepended to explanation (blank line separator) so ExplainData needs no new fields.
func (d *Driver) ParseResponse(response string) (translation, explanation string) {
	lines := strings.Split(response, "\n")
	var transLines, formLines, explLines []string
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
		if strings.HasPrefix(trimmed, "FORM:") {
			section = "form"
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "FORM:"))
			if rest != "" {
				formLines = append(formLines, rest)
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
		case "form":
			if trimmed != "" {
				formLines = append(formLines, trimmed)
			}
		case "explanation":
			explLines = append(explLines, line)
		}
	}

	translation = strings.TrimSpace(strings.Join(transLines, " "))
	form := strings.TrimSpace(strings.Join(formLines, " "))
	expl := strings.TrimSpace(strings.Join(explLines, "\n"))

	// Merge FORM and EXPLANATION: FORM first, blank line, then EXPLANATION.
	switch {
	case form != "" && expl != "":
		explanation = form + "\n\n" + expl
	case form != "":
		explanation = form
	default:
		explanation = expl
	}

	if translation == "" && explanation == "" {
		explanation = strings.TrimSpace(response)
	}
	return
}
