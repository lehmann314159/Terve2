package server

import (
	"net/http"
)

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// Static files
	fs := http.FileServer(http.Dir("static"))
	s.router.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Pages
	s.router.Get("/", s.handlers.Home)
	s.router.Get("/read", s.handlers.ReadingPage)

	// HTMX partials
	s.router.Get("/articles", s.handlers.ListArticles)
	s.router.Get("/article/{id}", s.handlers.ShowArticle)
	s.router.Post("/analyze", s.handlers.Analyze)
	s.router.Post("/load-text", s.handlers.LoadCustomText)
}
