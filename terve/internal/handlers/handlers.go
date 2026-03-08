package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// Handlers holds dependencies for HTTP handlers.
type Handlers struct {
	templates *template.Template
	voikko    *voikko.Client
	ollama    *ollama.Client
}

// New creates a new Handlers instance.
func New(templates *template.Template, vc *voikko.Client, oc *ollama.Client) *Handlers {
	return &Handlers{
		templates: templates,
		voikko:    vc,
		ollama:    oc,
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
