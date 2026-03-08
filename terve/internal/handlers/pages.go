package handlers

import (
	"net/http"
)

// PageData is passed to full page templates.
type PageData struct {
	Title      string
	ActivePage string
}

// Home renders the home page.
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", PageData{
		Title:      "Terve — Finnish Reading",
		ActivePage: "home",
	})
}

// ReadingPage renders the two-panel reading + analysis page.
func (h *Handlers) ReadingPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", PageData{
		Title:      "Terve — Read",
		ActivePage: "read",
	})
}
