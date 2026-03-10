package ollama

import (
	"encoding/json"
	"fmt"
	"strings"
)

const sentenceSystem = "You are a Finnish language teacher. Return ONLY valid JSON with no markdown fences or extra text."

// SentenceEntry represents a generated example sentence.
type SentenceEntry struct {
	Finnish    string `json:"finnish"`
	English    string `json:"english"`
	TargetForm string `json:"target_form"`
}

// GenerateSentences asks Ollama to generate 5 example sentences using a lemma.
func GenerateSentences(c *Client, lemma, wordClass string) ([]SentenceEntry, error) {
	prompt := fmt.Sprintf(`Generate 5 short Finnish sentences (A2-B1 level) using the %s "%s" in various forms.

Return a JSON array where each element has:
- "finnish": the Finnish sentence
- "english": the English translation
- "target_form": the exact form of "%s" as it appears in the sentence

Example format:
[
  {"finnish": "Menen taloon aamulla.", "english": "I go to the house in the morning.", "target_form": "taloon"},
  ...
]

Return ONLY the JSON array, no explanation.`, wordClass, lemma, lemma)

	resp, err := c.Generate(sentenceSystem, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate sentences: %w", err)
	}

	return parseSentenceJSON(resp)
}

// parseSentenceJSON extracts a JSON array of SentenceEntry from an LLM response,
// stripping markdown fences. Entries where target_form doesn't appear in finnish
// are filtered out.
func parseSentenceJSON(resp string) ([]SentenceEntry, error) {
	resp = strings.TrimSpace(resp)

	// Strip markdown code fences
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		if len(lines) > 1 {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		resp = strings.Join(lines, "\n")
	}

	resp = strings.TrimSpace(resp)

	var entries []SentenceEntry
	if err := json.Unmarshal([]byte(resp), &entries); err != nil {
		return nil, fmt.Errorf("parse sentence JSON: %w (response: %.200s)", err, resp)
	}

	// Filter: target_form must appear in finnish (case-insensitive)
	var valid []SentenceEntry
	for _, e := range entries {
		if e.Finnish == "" || e.English == "" || e.TargetForm == "" {
			continue
		}
		if strings.Contains(strings.ToLower(e.Finnish), strings.ToLower(e.TargetForm)) {
			valid = append(valid, e)
		}
	}

	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid sentences after filtering (had %d entries)", len(entries))
	}

	return valid, nil
}
