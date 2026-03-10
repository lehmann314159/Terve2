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

	// Books (public reading, auth-gated bookmarks/import)
	s.router.Route("/books", func(r chi.Router) {
		r.Get("/", s.handlers.BooksPage)
		r.Get("/{bookID}", s.handlers.BookReader)
		r.Get("/{bookID}/chapter/{num}", s.handlers.BookChapter)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth)
			r.Post("/{bookID}/bookmark", s.handlers.SaveBookmark)
			r.Get("/search", s.handlers.SearchGutenberg)
			r.Post("/import", s.handlers.ImportBook)
		})
	})

	// Quiz (requires auth)
	s.router.Route("/quiz", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Get("/", s.handlers.QuizHub)
		r.Get("/case-id", s.handlers.CaseIDPage)
		r.Get("/case-id/question", s.handlers.CaseIDQuestion)
		r.Post("/case-id/answer", s.handlers.CaseIDAnswer)
		r.Get("/form-english", s.handlers.FormEnglishPage)
		r.Get("/form-english/question", s.handlers.FormEnglishQuestion)
		r.Post("/form-english/answer", s.handlers.FormEnglishAnswer)
		r.Get("/declension", s.handlers.DeclensionPage)
		r.Get("/declension/question", s.handlers.DeclensionQuestion)
		r.Post("/declension/answer", s.handlers.DeclensionAnswer)
		r.Get("/conjugation", s.handlers.ConjugationPage)
		r.Get("/conjugation/question", s.handlers.ConjugationQuestion)
		r.Post("/conjugation/answer", s.handlers.ConjugationAnswer)
		r.Post("/results", s.handlers.QuizResults)
	})

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
		r.Post("/{cardID}/focus-review", s.handlers.ToggleFocusReview)
		r.Get("/review", s.handlers.ReviewSession)
		r.Post("/review/{userCardID}", s.handlers.SubmitReview)
	})
}
