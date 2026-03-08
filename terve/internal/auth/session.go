package auth

import (
	"encoding/gob"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const (
	sessionCookieName = "terve_session"
	sessionMaxAge     = 30 * 24 * time.Hour // 30 days
)

// Session holds the authenticated user's identity.
type Session struct {
	UserID    string
	DBUserID  int64  // primary key in the users table
	Name      string
	Email     string
	AvatarURL string
	Provider  string // "google" or "github"
	ExpiresAt time.Time
}

func init() {
	gob.Register(&Session{})
}

// CookieStore wraps securecookie for session encoding/decoding.
type CookieStore struct {
	sc *securecookie.SecureCookie
}

// NewCookieStore creates a cookie store with the given hash and encryption keys.
func NewCookieStore(hashKey, encryptKey []byte) *CookieStore {
	return &CookieStore{
		sc: securecookie.New(hashKey, encryptKey),
	}
}

// Save encodes the session and sets it as a cookie.
func (cs *CookieStore) Save(w http.ResponseWriter, sess *Session) error {
	encoded, err := cs.sc.Encode(sessionCookieName, sess)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // set true behind TLS in production
	})
	return nil
}

// Load decodes the session from the request cookie.
// Returns nil (no error) if the cookie is missing or expired.
func (cs *CookieStore) Load(r *http.Request) *Session {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	var sess Session
	if err := cs.sc.Decode(sessionCookieName, cookie.Value, &sess); err != nil {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return &sess
}

// Clear removes the session cookie.
func (cs *CookieStore) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
