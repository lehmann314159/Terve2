package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/db"
	"github.com/lehmann314159/terve2/internal/language"
	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// Handlers holds dependencies for HTTP handlers.
type Handlers struct {
	templates   *template.Template
	drivers     map[string]language.Driver
	cookieStore *auth.CookieStore
	voikko      *voikko.Client // Finnish-only: used by quiz/flashcard/articles/books
	ollama      *ollama.Client
	db          *db.DB
}

// New creates a new Handlers instance.
func New(templates *template.Template, drivers map[string]language.Driver, cookieStore *auth.CookieStore, vc *voikko.Client, oc *ollama.Client, database *db.DB) *Handlers {
	return &Handlers{
		templates:   templates,
		drivers:     drivers,
		cookieStore: cookieStore,
		voikko:      vc,
		ollama:      oc,
		db:          database,
	}
}

// driverFor returns the language driver for the session's active language.
// Falls back to Finnish if the session is nil or the language is unrecognised.
func (h *Handlers) driverFor(r *http.Request) language.Driver {
	sess := auth.GetSession(r.Context())
	if sess != nil {
		if d, ok := h.drivers[sess.Language]; ok {
			return d
		}
	}
	if d, ok := h.drivers["fi"]; ok {
		return d
	}
	// last resort: return any driver
	for _, d := range h.drivers {
		return d
	}
	panic("no language drivers configured")
}

// render executes a full page template.
func (h *Handlers) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// renderPartial executes a partial template (for HTMX responses).
func (h *Handlers) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("partial template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
