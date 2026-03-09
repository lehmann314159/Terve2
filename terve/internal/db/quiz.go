package db

import "time"

// QuizResult represents a completed quiz session.
type QuizResult struct {
	ID        int64
	UserID    int64
	QuizType  string
	Total     int
	Correct   int
	CreatedAt time.Time
}

// SaveQuizResult records the outcome of a quiz session.
func (db *DB) SaveQuizResult(userID int64, quizType string, total, correct int) error {
	_, err := db.Exec(`
		INSERT INTO quiz_results (user_id, quiz_type, total, correct)
		VALUES (?, ?, ?, ?)
	`, userID, quizType, total, correct)
	return err
}

// GetRecentQuizResults returns recent quiz results for a user.
// If quizType is empty, returns all types.
func (db *DB) GetRecentQuizResults(userID int64, quizType string, limit int) ([]QuizResult, error) {
	var q string
	var args []any
	if quizType == "" {
		q = `SELECT id, user_id, quiz_type, total, correct, created_at
			FROM quiz_results WHERE user_id = ?
			ORDER BY created_at DESC LIMIT ?`
		args = []any{userID, limit}
	} else {
		q = `SELECT id, user_id, quiz_type, total, correct, created_at
			FROM quiz_results WHERE user_id = ? AND quiz_type = ?
			ORDER BY created_at DESC LIMIT ?`
		args = []any{userID, quizType, limit}
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QuizResult
	for rows.Next() {
		var r QuizResult
		if err := rows.Scan(&r.ID, &r.UserID, &r.QuizType, &r.Total, &r.Correct, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetRandomUserCards returns random cards from a user's deck with non-empty translations.
// excludeCardID can be set to skip a specific card (0 to skip none).
func (db *DB) GetRandomUserCards(userID, excludeCardID int64, limit int) ([]Card, error) {
	rows, err := db.Query(`
		SELECT c.id, c.finnish, c.lemma, c.word_class, c.morphology,
		       c.translation, c.explanation, c.context, c.source, c.created_at
		FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.user_id = ? AND c.translation != '' AND c.id != ?
		ORDER BY RANDOM()
		LIMIT ?
	`, userID, excludeCardID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.Finnish, &c.Lemma, &c.WordClass, &c.Morphology,
			&c.Translation, &c.Explanation, &c.Context, &c.Source, &c.CreatedAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// GetRandomUserCard returns a single random card from a user's deck.
func (db *DB) GetRandomUserCard(userID int64) (*Card, error) {
	row := db.QueryRow(`
		SELECT c.id, c.finnish, c.lemma, c.word_class, c.morphology,
		       c.translation, c.explanation, c.context, c.source, c.created_at
		FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.user_id = ?
		ORDER BY RANDOM()
		LIMIT 1
	`, userID)

	var c Card
	err := row.Scan(&c.ID, &c.Finnish, &c.Lemma, &c.WordClass, &c.Morphology,
		&c.Translation, &c.Explanation, &c.Context, &c.Source, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetRandomUserCardWithTranslation returns a single random card with a non-empty translation.
func (db *DB) GetRandomUserCardWithTranslation(userID int64) (*Card, error) {
	row := db.QueryRow(`
		SELECT c.id, c.finnish, c.lemma, c.word_class, c.morphology,
		       c.translation, c.explanation, c.context, c.source, c.created_at
		FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.user_id = ? AND c.translation != ''
		ORDER BY RANDOM()
		LIMIT 1
	`, userID)

	var c Card
	err := row.Scan(&c.ID, &c.Finnish, &c.Lemma, &c.WordClass, &c.Morphology,
		&c.Translation, &c.Explanation, &c.Context, &c.Source, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
