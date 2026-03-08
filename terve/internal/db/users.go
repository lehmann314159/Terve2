package db

import "time"

// User represents a row in the users table.
type User struct {
	ID         int64
	Provider   string
	ProviderID string
	Name       string
	Email      string
	AvatarURL  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpsertUser creates or updates a user by provider+provider_id.
// On conflict, name/email/avatar_url/updated_at are refreshed.
// Returns the database user ID.
func (db *DB) UpsertUser(provider, providerID, name, email, avatarURL string) (int64, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO users (provider, provider_id, name, email, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(provider, provider_id) DO UPDATE SET
			name       = excluded.name,
			email      = excluded.email,
			avatar_url = excluded.avatar_url,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`, provider, providerID, name, email, avatarURL).Scan(&id)
	return id, err
}

// GetUser returns a user by database ID, or nil if not found.
func (db *DB) GetUser(id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT id, provider, provider_id, name, email, avatar_url, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.Name, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
