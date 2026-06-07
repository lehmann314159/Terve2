package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/lehmann314159/terve2/internal/db"
	"github.com/lehmann314159/terve2/internal/language"
	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// Handlers holds dependencies for HTTP handlers.
type Handlers struct {
	templates *template.Template
	driver    language.Driver
	voikko    *voikko.Client  // Finnish-only: used by quiz/flashcard/articles/books
	ollama    *ollama.Client
	db        *db.DB
}

// New creates a new Handlers instance.
func New(templates *template.Template, driver language.Driver, vc *voikko.Client, oc *ollama.Client, database *db.DB) *Handlers {
	return &Handlers{
		templates: templates,
		driver:    driver,
		voikko:    vc,
		ollama:    oc,
		db:        database,
	}
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
