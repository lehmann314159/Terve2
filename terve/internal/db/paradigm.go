package db

import (
	"encoding/json"
	"fmt"
)

// GetParadigm returns cached paradigm forms for a lemma+wordClass+tense.
// Returns sql.ErrNoRows if not cached.
func (db *DB) GetParadigm(lemma, wordClass, tense string) (map[string]string, error) {
	var formsJSON string
	err := db.QueryRow(`
		SELECT forms_json FROM paradigms
		WHERE lemma = ? AND word_class = ? AND tense = ?
	`, lemma, wordClass, tense).Scan(&formsJSON)
	if err != nil {
		return nil, err
	}

	var forms map[string]string
	if err := json.Unmarshal([]byte(formsJSON), &forms); err != nil {
		return nil, fmt.Errorf("decode paradigm forms: %w", err)
	}
	return forms, nil
}

// SaveParadigm upserts a paradigm into the cache.
func (db *DB) SaveParadigm(lemma, wordClass, tense string, forms map[string]string) error {
	data, err := json.Marshal(forms)
	if err != nil {
		return fmt.Errorf("encode paradigm forms: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO paradigms (lemma, word_class, tense, forms_json)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(lemma, word_class, tense) DO UPDATE SET forms_json = excluded.forms_json
	`, lemma, wordClass, tense, string(data))
	return err
}

// DeleteParadigm removes a cached paradigm (used if regeneration is needed).
func (db *DB) DeleteParadigm(lemma, wordClass, tense string) error {
	_, err := db.Exec(`
		DELETE FROM paradigms WHERE lemma = ? AND word_class = ? AND tense = ?
	`, lemma, wordClass, tense)
	return err
}
