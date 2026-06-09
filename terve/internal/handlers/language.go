package handlers

import (
	"net/http"
	"time"

	"github.com/lehmann314159/terve2/internal/auth"
)

// SetLanguage handles POST /language — updates the session's active language.
func (h *Handlers) SetLanguage(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("lang")
	if _, ok := h.drivers[lang]; !ok {
		http.Error(w, "unsupported language", http.StatusBadRequest)
		return
	}

	sess := auth.GetSession(r.Context())
	if sess == nil {
		sess = &auth.Session{
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		}
	}
	sess.Language = lang

	if err := h.cookieStore.Save(w, sess); err != nil {
		http.Error(w, "could not save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
