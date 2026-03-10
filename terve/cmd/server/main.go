package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/lehmann314159/terve2/internal/auth"
	"github.com/lehmann314159/terve2/internal/server"
)

func main() {
	loadEnvFile(".env")

	port := envOr("PORT", "3000")
	voikkoURL := envOr("VOIKKO_URL", "http://localhost:8000")
	ollamaURL := envOr("OLLAMA_URL", "http://localhost:11434")
	ollamaModel := envOr("OLLAMA_MODEL", "qwen2.5:32b-instruct-q4_K_M")
	dbPath := envOr("DB_PATH", "./data/terve.db")

	authCfg := auth.AuthConfig{
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		BaseURL:            envOr("BASE_URL", "http://localhost:3000"),
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	sessionEncryptKey := os.Getenv("SESSION_ENCRYPT_KEY")

	srv, err := server.New(port, voikkoURL, ollamaURL, ollamaModel, dbPath, authCfg, sessionSecret, sessionEncryptKey)
	if err != nil {
		log.Fatalf("Server init failed: %v", err)
	}

	log.Printf("Starting Terve on http://localhost:%s", port)
	log.Printf("Voikko: %s  Ollama: %s  Model: %s", voikkoURL, ollamaURL, ollamaModel)
	log.Printf("Database: %s", dbPath)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadEnvFile reads a .env file and sets vars that aren't already in the environment.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // missing .env is fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
