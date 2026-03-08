package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lehmann314159/terve2/internal/auth"
)

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// Static files
	fs := http.FileServer(http.Dir("static"))
	s.router.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Pages
	s.router.Get("/", s.handlers.Home)
	s.router.Get("/read", s.handlers.ReadingPage)

	// Auth
	s.router.Get("/auth/login", s.authHandlers.LoginPage)
	s.router.Get("/auth/{provider}", s.authHandlers.BeginAuth)
	s.router.Get("/auth/{provider}/callback", s.authHandlers.Callback)
	s.router.Post("/auth/logout", s.authHandlers.Logout)

	// HTMX partials
	s.router.Get("/articles", s.handlers.ListArticles)
	s.router.Get("/article/{id}", s.handlers.ShowArticle)
	s.router.Post("/analyze", s.handlers.Analyze)
	s.router.Post("/explain", s.handlers.Explain)
	s.router.Post("/load-text", s.handlers.LoadCustomText)

	// Flashcards (requires auth)
	s.router.Route("/flashcards", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Get("/", s.handlers.FlashcardsPage)
		r.Get("/list", s.handlers.FlashcardList)
		r.Post("/save", s.handlers.SaveFlashcard)
		r.Post("/validate", s.handlers.ValidateFlashcard)
		r.Post("/add", s.handlers.AddFlashcard)
		r.Delete("/{cardID}", s.handlers.DeleteFlashcard)
		r.Post("/{cardID}/focus", s.handlers.ToggleFocus)
		r.Get("/review", s.handlers.ReviewSession)
		r.Post("/review/{userCardID}", s.handlers.SubmitReview)
	})
}
