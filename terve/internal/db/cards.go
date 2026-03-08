package db

import (
	"database/sql"
	"time"
)

// Card represents a row in the cards table (shared vocabulary entry).
type Card struct {
	ID          int64
	Finnish     string
	Lemma       string
	WordClass   string
	Morphology  string
	Translation string
	Explanation string
	Context     string
	Source      string
	CreatedAt   time.Time
}

// UserCard represents a row in the user_cards table (per-user review state).
type UserCard struct {
	ID           int64
	UserID       int64
	CardID       int64
	Focused      bool
	EaseFactor   float64
	IntervalDays int
	Repetitions  int
	NextReview   time.Time
	LastReview   *time.Time
	CreatedAt    time.Time
	Card         Card // joined card data
}

// CreateCard inserts a card, returning its ID. If the lemma+finnish pair
// already exists, returns the existing card's ID.
func (db *DB) CreateCard(c *Card) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO cards (finnish, lemma, word_class, morphology, translation, explanation, context, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(lemma, finnish) DO UPDATE SET
			translation = CASE WHEN excluded.translation != '' THEN excluded.translation ELSE cards.translation END,
			explanation = CASE WHEN excluded.explanation != '' THEN excluded.explanation ELSE cards.explanation END
	`, c.Finnish, c.Lemma, c.WordClass, c.Morphology, c.Translation, c.Explanation, c.Context, c.Source)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	// If ON CONFLICT fired, LastInsertId may be 0 — look up by unique key.
	if id == 0 {
		err = db.QueryRow(`SELECT id FROM cards WHERE lemma = ? AND finnish = ?`, c.Lemma, c.Finnish).Scan(&id)
	}
	return id, err
}

// SaveUserCard links a user to a card. Returns the user_card ID.
func (db *DB) SaveUserCard(userID, cardID int64) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO user_cards (user_id, card_id)
		VALUES (?, ?)
		ON CONFLICT(user_id, card_id) DO NOTHING
	`, userID, cardID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		err = db.QueryRow(`SELECT id FROM user_cards WHERE user_id = ? AND card_id = ?`, userID, cardID).Scan(&id)
	}
	return id, err
}

// GetDueCards returns user_cards due for review, ordered by next_review.
// Focused cards come first, then by due date. Limit to n cards.
func (db *DB) GetDueCards(userID int64, limit int) ([]UserCard, error) {
	rows, err := db.Query(`
		SELECT uc.id, uc.user_id, uc.card_id, uc.focused, uc.ease_factor,
		       uc.interval_days, uc.repetitions, uc.next_review, uc.last_review, uc.created_at,
		       c.id, c.finnish, c.lemma, c.word_class, c.morphology,
		       c.translation, c.explanation, c.context, c.source, c.created_at
		FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.user_id = ? AND uc.next_review <= CURRENT_TIMESTAMP
		ORDER BY uc.focused DESC, uc.next_review ASC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUserCards(rows)
}

// UpdateReview updates the SM-2 fields on a user_card after a review.
func (db *DB) UpdateReview(ucID int64, easeFactor float64, intervalDays, repetitions int, nextReview time.Time) error {
	_, err := db.Exec(`
		UPDATE user_cards
		SET ease_factor = ?, interval_days = ?, repetitions = ?, next_review = ?, last_review = CURRENT_TIMESTAMP
		WHERE id = ?
	`, easeFactor, intervalDays, repetitions, nextReview, ucID)
	return err
}

// ToggleFocus flips the focused flag on a user_card.
func (db *DB) ToggleFocus(ucID, userID int64) error {
	_, err := db.Exec(`
		UPDATE user_cards SET focused = NOT focused
		WHERE id = ? AND user_id = ?
	`, ucID, userID)
	return err
}

// EnrollUserInSeedCards creates user_card rows for all seed cards the user
// doesn't already have.
func (db *DB) EnrollUserInSeedCards(userID int64) error {
	_, err := db.Exec(`
		INSERT INTO user_cards (user_id, card_id)
		SELECT ?, c.id FROM cards c
		WHERE c.source = 'seed'
		AND c.id NOT IN (SELECT card_id FROM user_cards WHERE user_id = ?)
	`, userID, userID)
	return err
}

// ListUserCards returns all user_cards for a user, with joined card data.
// filter: "all", "focused", "due"
func (db *DB) ListUserCards(userID int64, filter string) ([]UserCard, error) {
	q := `
		SELECT uc.id, uc.user_id, uc.card_id, uc.focused, uc.ease_factor,
		       uc.interval_days, uc.repetitions, uc.next_review, uc.last_review, uc.created_at,
		       c.id, c.finnish, c.lemma, c.word_class, c.morphology,
		       c.translation, c.explanation, c.context, c.source, c.created_at
		FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.user_id = ?
	`
	switch filter {
	case "focused":
		q += ` AND uc.focused = 1`
	case "due":
		q += ` AND uc.next_review <= CURRENT_TIMESTAMP`
	}
	q += ` ORDER BY uc.focused DESC, c.finnish ASC`

	rows, err := db.Query(q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUserCards(rows)
}

// DeleteUserCard removes a user_card (not the underlying card).
func (db *DB) DeleteUserCard(ucID, userID int64) error {
	_, err := db.Exec(`DELETE FROM user_cards WHERE id = ? AND user_id = ?`, ucID, userID)
	return err
}

// CountUserCards returns total and due counts for a user.
func (db *DB) CountUserCards(userID int64) (total, due int, err error) {
	err = db.QueryRow(`
		SELECT
			COUNT(*),
			COUNT(CASE WHEN next_review <= CURRENT_TIMESTAMP THEN 1 END)
		FROM user_cards WHERE user_id = ?
	`, userID).Scan(&total, &due)
	return
}

// GetUserCard returns a single user_card with joined card data.
func (db *DB) GetUserCard(ucID, userID int64) (*UserCard, error) {
	row := db.QueryRow(`
		SELECT uc.id, uc.user_id, uc.card_id, uc.focused, uc.ease_factor,
		       uc.interval_days, uc.repetitions, uc.next_review, uc.last_review, uc.created_at,
		       c.id, c.finnish, c.lemma, c.word_class, c.morphology,
		       c.translation, c.explanation, c.context, c.source, c.created_at
		FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.id = ? AND uc.user_id = ?
	`, ucID, userID)

	uc := &UserCard{}
	var lastReview sql.NullTime
	err := row.Scan(
		&uc.ID, &uc.UserID, &uc.CardID, &uc.Focused, &uc.EaseFactor,
		&uc.IntervalDays, &uc.Repetitions, &uc.NextReview, &lastReview, &uc.CreatedAt,
		&uc.Card.ID, &uc.Card.Finnish, &uc.Card.Lemma, &uc.Card.WordClass, &uc.Card.Morphology,
		&uc.Card.Translation, &uc.Card.Explanation, &uc.Card.Context, &uc.Card.Source, &uc.Card.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastReview.Valid {
		uc.LastReview = &lastReview.Time
	}
	return uc, nil
}

// UserHasCard checks whether a user already has a card with the given lemma+finnish.
func (db *DB) UserHasCard(userID int64, lemma, finnish string) bool {
	var exists int
	db.QueryRow(`
		SELECT 1 FROM user_cards uc
		JOIN cards c ON c.id = uc.card_id
		WHERE uc.user_id = ? AND c.lemma = ? AND c.finnish = ?
	`, userID, lemma, finnish).Scan(&exists)
	return exists == 1
}

func scanUserCards(rows *sql.Rows) ([]UserCard, error) {
	var ucs []UserCard
	for rows.Next() {
		var uc UserCard
		var lastReview sql.NullTime
		err := rows.Scan(
			&uc.ID, &uc.UserID, &uc.CardID, &uc.Focused, &uc.EaseFactor,
			&uc.IntervalDays, &uc.Repetitions, &uc.NextReview, &lastReview, &uc.CreatedAt,
			&uc.Card.ID, &uc.Card.Finnish, &uc.Card.Lemma, &uc.Card.WordClass, &uc.Card.Morphology,
			&uc.Card.Translation, &uc.Card.Explanation, &uc.Card.Context, &uc.Card.Source, &uc.Card.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if lastReview.Valid {
			uc.LastReview = &lastReview.Time
		}
		ucs = append(ucs, uc)
	}
	return ucs, rows.Err()
}
