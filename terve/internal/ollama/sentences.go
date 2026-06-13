package ollama

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SentenceEntry represents a generated example sentence.
type SentenceEntry struct {
	Sentence   string `json:"sentence"`
	English    string `json:"english"`
	TargetForm string `json:"target_form"`
}

// GenerateSentences asks Ollama to generate 5 example sentences using a lemma
// in the given language (e.g. "Finnish", "Spanish").
func GenerateSentences(c *Client, language, lemma, wordClass string) ([]SentenceEntry, error) {
	system := fmt.Sprintf("You are a %s language teacher. Return ONLY valid JSON with no markdown fences or extra text.", language)

	prompt := fmt.Sprintf(`Generate 5 short %s sentences (A2-B1 level) using the %s "%s" in various forms.

Return a JSON array where each element has:
- "sentence": the %s sentence
- "english": the English translation
- "target_form": the exact form of "%s" as it appears in the sentence

Example format:
[
  {"sentence": "Example sentence here.", "english": "English translation.", "target_form": "word_form"},
  ...
]

Return ONLY the JSON array, no explanation.`, language, wordClass, lemma, language, lemma)

	resp, err := c.Generate(system, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate sentences: %w", err)
	}

	return parseSentenceJSON(resp)
}

// parseSentenceJSON extracts a JSON array of SentenceEntry from an LLM response,
// stripping markdown fences. Entries where target_form doesn't appear in sentence
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

	// Filter: target_form must appear in sentence (case-insensitive)
	var valid []SentenceEntry
	for _, e := range entries {
		if e.Sentence == "" || e.English == "" || e.TargetForm == "" {
			continue
		}
		if strings.Contains(strings.ToLower(e.Sentence), strings.ToLower(e.TargetForm)) {
			valid = append(valid, e)
		}
	}

	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid sentences after filtering (had %d entries)", len(entries))
	}

	return valid, nil
}
