package db

import "time"

// CachedSentence represents a cached example sentence for a lemma.
type CachedSentence struct {
	ID         int64
	Lemma      string
	Finnish    string
	English    string
	TargetForm string
	CreatedAt  time.Time
}

// GetSentencesByLemma returns cached sentences for a lemma.
// Returns an empty slice (not error) if none exist.
func (db *DB) GetSentencesByLemma(lemma string) ([]CachedSentence, error) {
	rows, err := db.Query(`
		SELECT id, lemma, finnish, english, target_form, created_at
		FROM sentence_cache
		WHERE lemma = ?
	`, lemma)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sentences []CachedSentence
	for rows.Next() {
		var s CachedSentence
		if err := rows.Scan(&s.ID, &s.Lemma, &s.Finnish, &s.English, &s.TargetForm, &s.CreatedAt); err != nil {
			return nil, err
		}
		sentences = append(sentences, s)
	}
	return sentences, rows.Err()
}

// SaveSentences batch-inserts cached sentences.
func (db *DB) SaveSentences(sentences []CachedSentence) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO sentence_cache (lemma, finnish, english, target_form)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range sentences {
		if _, err := stmt.Exec(s.Lemma, s.Finnish, s.English, s.TargetForm); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetRandomSentencesExcludingLemma returns random cached sentences from
// lemmas other than the excluded one. Results are grouped by lemma for diversity.
func (db *DB) GetRandomSentencesExcludingLemma(excludeLemma string, limit int) ([]CachedSentence, error) {
	rows, err := db.Query(`
		SELECT id, lemma, finnish, english, target_form, created_at
		FROM sentence_cache
		WHERE lemma != ?
		GROUP BY lemma
		ORDER BY RANDOM()
		LIMIT ?
	`, excludeLemma, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sentences []CachedSentence
	for rows.Next() {
		var s CachedSentence
		if err := rows.Scan(&s.ID, &s.Lemma, &s.Finnish, &s.English, &s.TargetForm, &s.CreatedAt); err != nil {
			return nil, err
		}
		sentences = append(sentences, s)
	}
	return sentences, rows.Err()
}
