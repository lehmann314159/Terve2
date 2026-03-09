package rss

import (
	"strings"
	"testing"
)

func TestExtractText_ArticleTag(t *testing.T) {
	html := `<html><body>
		<article>
			<p>First paragraph inside article.</p>
			<p>Second paragraph inside article.</p>
		</article>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if !strings.Contains(got, "First paragraph inside article.") {
		t.Errorf("expected first paragraph, got %q", got)
	}
	if !strings.Contains(got, "Second paragraph inside article.") {
		t.Errorf("expected second paragraph, got %q", got)
	}
}

func TestExtractText_YleArticleClass(t *testing.T) {
	html := `<html><body>
		<div class="yle__article">
			<p>YLE article content here.</p>
		</div>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if !strings.Contains(got, "YLE article content here.") {
		t.Errorf("expected YLE content, got %q", got)
	}
}

func TestExtractText_ArticleClass(t *testing.T) {
	html := `<html><body>
		<div class="article">
			<p>Article class content.</p>
		</div>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if !strings.Contains(got, "Article class content.") {
		t.Errorf("expected article class content, got %q", got)
	}
}

func TestExtractText_FallbackLongParagraphs(t *testing.T) {
	html := `<html><body>
		<p>Short</p>
		<p>This is a longer paragraph that should be captured by the fallback logic.</p>
		<p>Another sufficiently long paragraph for fallback extraction.</p>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	// "Short" should be excluded (len <= 20)
	if strings.Contains(got, "Short") {
		t.Errorf("short paragraph should be excluded in fallback, got %q", got)
	}
	if !strings.Contains(got, "longer paragraph") {
		t.Errorf("expected longer paragraph in fallback, got %q", got)
	}
}

func TestExtractText_EmptyParagraphsExcluded(t *testing.T) {
	html := `<html><body>
		<article>
			<p>  </p>
			<p>Real content here.</p>
			<p></p>
		</article>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if got != "Real content here." {
		t.Errorf("got %q, want only real content", got)
	}
}

func TestExtractText_MultipleParagraphsJoined(t *testing.T) {
	html := `<html><body>
		<article>
			<p>Paragraph one.</p>
			<p>Paragraph two.</p>
			<p>Paragraph three.</p>
		</article>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}

	parts := strings.Split(got, "\n\n")
	if len(parts) != 3 {
		t.Errorf("expected 3 paragraphs joined by \\n\\n, got %d parts: %q", len(parts), got)
	}
}

func TestExtractText_MinimalHTML(t *testing.T) {
	html := `<html><body></body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for minimal HTML, got %q", got)
	}
}

func TestExtractText_NestedElements(t *testing.T) {
	html := `<html><body>
		<article>
			<p>Text with <strong>bold</strong> and <a href="#">links</a> inside.</p>
		</article>
	</body></html>`

	got, err := extractText(strings.NewReader(html))
	if err != nil {
		t.Fatalf("extractText: %v", err)
	}
	if got != "Text with bold and links inside." {
		t.Errorf("got %q, want collected text from nested elements", got)
	}
}
