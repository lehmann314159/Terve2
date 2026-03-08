package rss

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const feedURL = "https://feeds.yle.fi/uutiset/v1/recent.rss?publisherIds=YLE_SELKOUUTISET"

// Article represents a scraped article.
type Article struct {
	ID    string // index in the feed
	Title string
	Link  string
	Desc  string
	Text  string // scraped full text (paragraphs joined by newlines)
}

// rss XML structures
type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Channel rssChan   `xml:"channel"`
}

type rssChan struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
	Desc  string `xml:"description"`
}

var client = &http.Client{Timeout: 10 * time.Second}

// FetchFeed fetches the YLE Selkosuomi RSS feed and returns article metadata.
func FetchFeed() ([]Article, error) {
	resp, err := client.Get(feedURL)
	if err != nil {
		return nil, fmt.Errorf("rss: fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss: feed returned %d", resp.StatusCode)
	}

	var feed rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("rss: parse feed: %w", err)
	}

	articles := make([]Article, 0, len(feed.Channel.Items))
	for i, item := range feed.Channel.Items {
		articles = append(articles, Article{
			ID:    fmt.Sprintf("%d", i),
			Title: item.Title,
			Link:  item.Link,
			Desc:  item.Desc,
		})
	}
	return articles, nil
}

// ScrapeArticle fetches the HTML page for an article and extracts text content.
func ScrapeArticle(link string) (string, error) {
	resp, err := client.Get(link)
	if err != nil {
		return "", fmt.Errorf("rss: scrape article: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("rss: article returned %d", resp.StatusCode)
	}

	return extractText(resp.Body)
}

// extractText parses HTML and pulls text from <p> tags inside article content.
func extractText(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("rss: parse html: %w", err)
	}

	var paragraphs []string
	var inArticle bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Look for article or main content areas
		if n.Type == html.ElementNode {
			if n.Data == "article" || hasClass(n, "yle__article") || hasClass(n, "article") {
				inArticle = true
			}
		}

		// Collect text from <p> elements
		if n.Type == html.ElementNode && n.Data == "p" && inArticle {
			text := collectText(n)
			text = strings.TrimSpace(text)
			if text != "" {
				paragraphs = append(paragraphs, text)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == html.ElementNode && (n.Data == "article" || hasClass(n, "yle__article") || hasClass(n, "article")) {
			inArticle = false
		}
	}
	walk(doc)

	// Fallback: if no article tag found, grab all <p> tags
	if len(paragraphs) == 0 {
		var walkAll func(*html.Node)
		walkAll = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "p" {
				text := strings.TrimSpace(collectText(n))
				if text != "" && len(text) > 20 {
					paragraphs = append(paragraphs, text)
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walkAll(c)
			}
		}
		walkAll(doc)
	}

	return strings.Join(paragraphs, "\n\n"), nil
}

// collectText recursively collects all text content from a node.
func collectText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(collectText(c))
	}
	return b.String()
}

// hasClass checks if an HTML node has a given class.
func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, class) {
			return true
		}
	}
	return false
}
