package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/securecookie"
	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/db"
	"github.com/lehmann314159/terve2/internal/handlers"
	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// Server represents the HTTP server.
type Server struct {
	port         string
	router       *chi.Mux
	templates    *template.Template
	handlers     *handlers.Handlers
	authHandlers *auth.AuthHandlers
}

// New creates a new server instance.
func New(port, voikkoURL, ollamaURL, ollamaModel, dbPath string, authCfg auth.AuthConfig, sessionSecret, sessionEncryptKey string) (*Server, error) {
	s := &Server{
		port:   port,
		router: chi.NewRouter(),
	}

	s.parseTemplates()

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Session cookie store
	hashKey := deriveKey(sessionSecret)
	encKey := deriveKey(sessionEncryptKey)
	cookieStore := auth.NewCookieStore(hashKey, encKey)

	// State cookie signer (uses same hash key, no encryption needed)
	stateSC := securecookie.New(hashKey, nil)

	vc := voikko.NewClient(voikkoURL)
	oc := ollama.NewClient(ollamaURL, ollamaModel)
	s.handlers = handlers.New(s.templates, vc, oc)
	s.authHandlers = auth.NewAuthHandlers(authCfg, cookieStore, stateSC, s.templates, database)

	// Middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Compress(5))
	s.router.Use(auth.Middleware(cookieStore))

	s.setupRoutes()

	return s, nil
}

// parseTemplates loads all HTML templates.
func (s *Server) parseTemplates() {
	funcMap := template.FuncMap{
		"safe": func(str string) template.HTML {
			return template.HTML(str)
		},
		"jsonString": func(s string) template.JS {
			b, _ := json.Marshal(s)
			return template.JS(b)
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

// deriveKey decodes a hex-encoded key or generates a random 32-byte key.
func deriveKey(hexKey string) []byte {
	if hexKey != "" {
		b, err := hex.DecodeString(hexKey)
		if err == nil && len(b) == 32 {
			return b
		}
		log.Printf("Warning: invalid key (expected 64 hex chars), generating random key")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random key: " + err.Error())
	}
	return b
}
