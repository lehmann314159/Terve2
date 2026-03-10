package voikko

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is an HTTP client for the Voikko sidecar service.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new Voikko client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// AnalyzeWord returns full morphological analysis of a word.
func (c *Client) AnalyzeWord(word string) ([]MorphAnalysis, error) {
	body, err := c.post("/analyze", map[string]string{"word": word})
	if err != nil {
		return nil, err
	}
	var result []MorphAnalysis
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("voikko: decode analyze response: %w", err)
	}
	return result, nil
}

// ValidateSentence tokenizes and analyzes every word in a sentence.
func (c *Client) ValidateSentence(sentence string) (*SentenceValidation, error) {
	body, err := c.post("/validate-sentence", map[string]string{"sentence": sentence})
	if err != nil {
		return nil, err
	}
	var result SentenceValidation
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("voikko: decode validate-sentence response: %w", err)
	}
	return &result, nil
}

// GetVowelHarmony determines front/back vowel harmony for a word.
func (c *Client) GetVowelHarmony(word string) (*VowelHarmony, error) {
	body, err := c.post("/vowel-harmony", map[string]string{"word": word})
	if err != nil {
		return nil, err
	}
	var result VowelHarmony
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("voikko: decode vowel-harmony response: %w", err)
	}
	return &result, nil
}

// GetSuggestions returns spelling suggestions for a word.
func (c *Client) GetSuggestions(word string) (*Suggestions, error) {
	body, err := c.post("/suggestions", map[string]string{"word": word})
	if err != nil {
		return nil, err
	}
	var result Suggestions
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("voikko: decode suggestions response: %w", err)
	}
	return &result, nil
}

// ValidateWord checks if a word is a valid Finnish word via Voikko's spell-checker.
func (c *Client) ValidateWord(word string) (bool, error) {
	body, err := c.post("/validate", map[string]string{"word": word})
	if err != nil {
		return false, err
	}
	var result WordValidation
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("voikko: decode validate response: %w", err)
	}
	return result.Valid, nil
}

// post sends a POST request with a JSON body and returns the response body.
func (c *Client) post(path string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("voikko: marshal request: %w", err)
	}

	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("voikko: request to %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voikko: %s returned %d", path, resp.StatusCode)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("voikko: read response: %w", err)
	}
	return buf.Bytes(), nil
}
