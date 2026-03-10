package ollama

import (
	"encoding/json"
	"fmt"
	"strings"
)

const paradigmSystem = "You are a Finnish morphology expert. Return ONLY valid JSON with no markdown fences or extra text."

// GenerateNounParadigm asks Ollama to generate all 28 case forms (14 cases x 2 numbers)
// for a Finnish noun or adjective. Returns a map like {"nominative_singular": "talo", ...}.
func GenerateNounParadigm(c *Client, lemma string) (map[string]string, error) {
	prompt := fmt.Sprintf(`Generate all 14 case forms of the Finnish noun/adjective "%s" in both singular and plural.

Return a JSON object with keys in the format "case_number", for example:
{
  "nominative_singular": "...",
  "nominative_plural": "...",
  "genitive_singular": "...",
  "genitive_plural": "...",
  "partitive_singular": "...",
  "partitive_plural": "...",
  "inessive_singular": "...",
  "inessive_plural": "...",
  "elative_singular": "...",
  "elative_plural": "...",
  "illative_singular": "...",
  "illative_plural": "...",
  "adessive_singular": "...",
  "adessive_plural": "...",
  "ablative_singular": "...",
  "ablative_plural": "...",
  "allative_singular": "...",
  "allative_plural": "...",
  "essive_singular": "...",
  "essive_plural": "...",
  "translative_singular": "...",
  "translative_plural": "...",
  "abessive_singular": "...",
  "abessive_plural": "...",
  "instructive_singular": "...",
  "instructive_plural": "...",
  "comitative_singular": "...",
  "comitative_plural": "..."
}

Return ONLY the JSON object, no explanation.`, lemma)

	resp, err := c.Generate(paradigmSystem, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate noun paradigm: %w", err)
	}

	return parseParadigmJSON(resp)
}

// GenerateVerbParadigm asks Ollama to generate present and past indicative forms
// for a Finnish verb. Returns a map like {"1st_present": "puhun", ...}.
func GenerateVerbParadigm(c *Client, lemma string) (map[string]string, error) {
	prompt := fmt.Sprintf(`Generate person forms of the Finnish verb "%s" in present and past indicative (imperfekti).

Include 1st, 2nd, 3rd person singular and plural, plus the passive form for each tense.

Return a JSON object with keys in the format "person_tense":
{
  "1st_singular_present": "...",
  "2nd_singular_present": "...",
  "3rd_singular_present": "...",
  "1st_plural_present": "...",
  "2nd_plural_present": "...",
  "3rd_plural_present": "...",
  "passive_present": "...",
  "1st_singular_past": "...",
  "2nd_singular_past": "...",
  "3rd_singular_past": "...",
  "1st_plural_past": "...",
  "2nd_plural_past": "...",
  "3rd_plural_past": "...",
  "passive_past": "..."
}

Return ONLY the JSON object, no explanation.`, lemma)

	resp, err := c.Generate(paradigmSystem, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate verb paradigm: %w", err)
	}

	return parseParadigmJSON(resp)
}

// parseParadigmJSON extracts a JSON object from an LLM response,
// stripping markdown fences and other formatting quirks.
func parseParadigmJSON(resp string) (map[string]string, error) {
	resp = strings.TrimSpace(resp)

	// Strip markdown code fences
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		// Remove first line (```json or ```)
		if len(lines) > 1 {
			lines = lines[1:]
		}
		// Remove last line if it's ```
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		resp = strings.Join(lines, "\n")
	}

	resp = strings.TrimSpace(resp)

	var forms map[string]string
	if err := json.Unmarshal([]byte(resp), &forms); err != nil {
		return nil, fmt.Errorf("parse paradigm JSON: %w (response: %.200s)", err, resp)
	}
	return forms, nil
}
