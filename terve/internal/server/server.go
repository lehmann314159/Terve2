package server

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lehmann314159/terve2/internal/handlers"
	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// Server represents the HTTP server.
type Server struct {
	port      string
	router    *chi.Mux
	templates *template.Template
	handlers  *handlers.Handlers
}

// New creates a new server instance.
func New(port, voikkoURL, ollamaURL, ollamaModel string) *Server {
	s := &Server{
		port:   port,
		router: chi.NewRouter(),
	}

	s.parseTemplates()

	vc := voikko.NewClient(voikkoURL)
	oc := ollama.NewClient(ollamaURL, ollamaModel)
	s.handlers = handlers.New(s.templates, vc, oc)

	// Middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Compress(5))

	s.setupRoutes()

	return s
}

// parseTemplates loads all HTML templates.
func (s *Server) parseTemplates() {
	funcMap := template.FuncMap{
		"safe": func(str string) template.HTML {
			return template.HTML(str)
		},
	}

	tmpl := template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseGlob(filepath.Join("templates", "layouts", "*.html")))
	tmpl = template.Must(tmpl.ParseGlob(filepath.Join("templates", "pages", "*.html")))
	tmpl = template.Must(tmpl.ParseGlob(filepath.Join("templates", "partials", "*.html")))

	s.templates = tmpl
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	return http.ListenAndServe(":"+s.port, s.router)
}
