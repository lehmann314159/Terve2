package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lehmann314159/terve2/internal/rss"
)

// cachedArticles holds the most recent feed fetch (stateless, no persistence).
var cachedArticles []rss.Article

// ListArticles fetches the RSS feed and renders the article list partial.
func (h *Handlers) ListArticles(w http.ResponseWriter, r *http.Request) {
	articles, err := rss.FetchFeed()
	if err != nil {
		log.Printf("RSS fetch error: %v", err)
		h.renderPartial(w, "article-list", map[string]any{
			"Error": "Could not load articles. Try pasting your own text below.",
		})
		return
	}
	cachedArticles = articles
	h.renderPartial(w, "article-list", map[string]any{
		"Articles": articles,
	})
}

// ShowArticle scrapes an article and renders tokenized text.
func (h *Handlers) ShowArticle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	idx, err := strconv.Atoi(idStr)
	if err != nil || idx < 0 || idx >= len(cachedArticles) {
		h.renderPartial(w, "article-text", map[string]any{
			"Error": "Article not found.",
		})
		return
	}

	article := cachedArticles[idx]

	// Scrape article text
	text, err := rss.ScrapeArticle(article.Link)
	if err != nil {
		log.Printf("Scrape error: %v", err)
		// Fall back to description
		text = article.Desc
	}
	if text == "" {
		text = article.Desc
	}

	// Tokenize via Voikko
	sv, err := h.voikko.ValidateSentence(text)
	if err != nil {
		log.Printf("Voikko tokenize error: %v", err)
		// Render plain text as fallback
		h.renderPartial(w, "article-text", map[string]any{
			"Title":     article.Title,
			"PlainText": text,
		})
		return
	}

	h.renderPartial(w, "article-text", map[string]any{
		"Title":  article.Title,
		"Tokens": sv.Tokens,
	})
}

// LoadCustomText tokenizes user-pasted text.
func (h *Handlers) LoadCustomText(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	if text == "" {
		h.renderPartial(w, "article-text", map[string]any{
			"Error": "No text provided.",
		})
		return
	}

	sv, err := h.voikko.ValidateSentence(text)
	if err != nil {
		log.Printf("Voikko tokenize error: %v", err)
		h.renderPartial(w, "article-text", map[string]any{
			"PlainText": text,
		})
		return
	}

	h.renderPartial(w, "article-text", map[string]any{
		"Title":  "Custom Text",
		"Tokens": sv.Tokens,
	})
}
