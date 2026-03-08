package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/securecookie"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

const (
	stateCookieName = "oauth_state"
	stateCookieAge  = 5 * 60 // 5 minutes
)

// AuthConfig holds OAuth credentials from env vars.
type AuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	BaseURL            string
}

// AuthHandlers manages OAuth login/callback/logout routes.
type AuthHandlers struct {
	config    AuthConfig
	store     *CookieStore
	stateSC   *securecookie.SecureCookie
	templates *template.Template
	providers map[string]*oauth2.Config
}

// NewAuthHandlers creates the auth handler set.
func NewAuthHandlers(cfg AuthConfig, store *CookieStore, stateSC *securecookie.SecureCookie, templates *template.Template) *AuthHandlers {
	h := &AuthHandlers{
		config:    cfg,
		store:     store,
		stateSC:   stateSC,
		templates: templates,
		providers: make(map[string]*oauth2.Config),
	}

	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		h.providers["google"] = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.BaseURL + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     endpoints.Google,
		}
	}

	if cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		h.providers["github"] = &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.BaseURL + "/auth/github/callback",
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     endpoints.GitHub,
		}
	}

	return h
}

// LoginPage renders the login page with available providers.
func (ah *AuthHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title         string
		ActivePage    string
		User          *Session
		GoogleEnabled bool
		GitHubEnabled bool
	}{
		Title:         "Terve — Sign In",
		ActivePage:    "login",
		User:          GetSession(r.Context()),
		GoogleEnabled: ah.providers["google"] != nil,
		GitHubEnabled: ah.providers["github"] != nil,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ah.templates.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("login template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// BeginAuth redirects the user to the OAuth provider.
func (ah *AuthHandlers) BeginAuth(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	cfg, ok := ah.providers[provider]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusBadRequest)
		return
	}

	state, err := ah.generateState()
	if err != nil {
		log.Printf("generate state: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	encoded, err := ah.stateSC.Encode(stateCookieName, state)
	if err != nil {
		log.Printf("encode state: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    encoded,
		Path:     "/auth/",
		MaxAge:   stateCookieAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusFound)
}

// Callback handles the OAuth provider callback.
func (ah *AuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	cfg, ok := ah.providers[provider]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusBadRequest)
		return
	}

	// Validate state
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil {
		http.Error(w, "Missing state cookie", http.StatusBadRequest)
		return
	}
	var expectedState string
	if err := ah.stateSC.Decode(stateCookieName, stateCookie.Value, &expectedState); err != nil {
		http.Error(w, "Invalid state cookie", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != expectedState {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   stateCookieName,
		Value:  "",
		Path:   "/auth/",
		MaxAge: -1,
	})

	// Exchange code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code", http.StatusBadRequest)
		return
	}

	token, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("oauth exchange (%s): %v", provider, err)
		http.Error(w, "OAuth exchange failed", http.StatusInternalServerError)
		return
	}

	// Fetch user info
	var user *UserInfo
	switch provider {
	case "google":
		user, err = FetchGoogleUser(r.Context(), token)
	case "github":
		user, err = FetchGitHubUser(r.Context(), token)
	}
	if err != nil {
		log.Printf("fetch user (%s): %v", provider, err)
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}

	// Create session
	sess := &Session{
		UserID:    fmt.Sprintf("%s:%s", provider, user.ID),
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		Provider:  provider,
		ExpiresAt: time.Now().Add(sessionMaxAge),
	}

	if err := ah.store.Save(w, sess); err != nil {
		log.Printf("save session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout clears the session cookie.
func (ah *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	ah.store.Clear(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (ah *AuthHandlers) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
