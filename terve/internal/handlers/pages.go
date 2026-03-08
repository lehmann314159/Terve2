package handlers

import (
	"net/http"

	"github.com/lehmann314159/terve2/internal/auth"
)

// PageData is passed to full page templates.
type PageData struct {
	Title         string
	ActivePage    string
	User          *auth.Session
	GoogleEnabled bool
	GitHubEnabled bool
}

// pageData creates a PageData with the session from the request context.
func pageData(r *http.Request, title, activePage string) PageData {
	return PageData{
		Title:      title,
		ActivePage: activePage,
		User:       auth.GetSession(r.Context()),
	}
}

// Home renders the home page.
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", pageData(r, "Terve — Finnish Reading", "home"))
}

// ReadingPage renders the two-panel reading + analysis page.
func (h *Handlers) ReadingPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "base", pageData(r, "Terve — Read", "read"))
}
