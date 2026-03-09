package gutenberg

import (
	"testing"
)

func TestStripBoilerplate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "both markers",
			in:   "Header stuff\n*** START OF THE PROJECT GUTENBERG EBOOK ***\nActual content here.\n*** END OF THE PROJECT GUTENBERG EBOOK ***\nFooter stuff",
			want: "Actual content here.",
		},
		{
			name: "no space after stars",
			in:   "Header\n***START OF THIS PROJECT GUTENBERG EBOOK***\nBody text.\n***END OF THIS PROJECT GUTENBERG EBOOK***\nFooter",
			want: "Body text.",
		},
		{
			name: "only start marker",
			in:   "Preamble\n*** START OF THE PROJECT GUTENBERG EBOOK ***\nJust body, no footer",
			want: "Just body, no footer",
		},
		{
			name: "only end marker",
			in:   "Content before end\n*** END OF THE PROJECT GUTENBERG EBOOK ***\nFooter",
			want: "Content before end",
		},
		{
			name: "no markers",
			in:   "Plain text with no markers at all",
			want: "Plain text with no markers at all",
		},
		{
			name: "empty after stripping",
			in:   "Header\n*** START OF THE PROJECT GUTENBERG EBOOK ***\n*** END OF THE PROJECT GUTENBERG EBOOK ***\nFooter",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripBoilerplate(tt.in)
			if got != tt.want {
				t.Errorf("StripBoilerplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitChapters_FinnishOrdinals(t *testing.T) {
	text := "Preamble text here.\n\nENSIMMÄINEN LUKU\n\nFirst chapter body.\n\nTOINEN LUKU\n\nSecond chapter body.\n\nKOLMAS LUKU\n\nThird chapter body."

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}

	// First should be preamble
	if chapters[0].Title != "" {
		t.Errorf("preamble chapter should have empty title, got %q", chapters[0].Title)
	}
	if chapters[0].Body != "Preamble text here." {
		t.Errorf("preamble body = %q", chapters[0].Body)
	}

	// Verify chapter titles contain the Finnish ordinals
	if chapters[1].Title != "ENSIMMÄINEN LUKU" {
		t.Errorf("chapter 1 title = %q, want %q", chapters[1].Title, "ENSIMMÄINEN LUKU")
	}
	if chapters[2].Title != "TOINEN LUKU" {
		t.Errorf("chapter 2 title = %q, want %q", chapters[2].Title, "TOINEN LUKU")
	}
}

func TestSplitChapters_RomanNumerals(t *testing.T) {
	text := "I.\n\nFirst section content.\n\nII.\n\nSecond section content.\n\nIII.\n\nThird section content."

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}

	if chapters[0].Title != "I" {
		t.Errorf("chapter 1 title = %q, want %q", chapters[0].Title, "I")
	}
	if chapters[0].Body != "First section content." {
		t.Errorf("chapter 1 body = %q", chapters[0].Body)
	}
}

func TestSplitChapters_Numbered(t *testing.T) {
	text := "1.\n\nFirst chapter.\n\n2.\n\nSecond chapter.\n\n3.\n\nThird chapter."

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}

	if chapters[0].Body != "First chapter." {
		t.Errorf("chapter 1 body = %q", chapters[0].Body)
	}
}

func TestSplitChapters_AllCaps(t *testing.T) {
	text := "ENSIMMÄINEN OSASTO\n\nFirst section body here.\n\nTOINEN OSASTO\n\nSecond section body here.\n\nKOLMAS OSASTO\n\nThird section body here."

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}
}

func TestSplitChapters_Fallback(t *testing.T) {
	text := "Just some plain text with no chapter markers whatsoever. It goes on for a while but has no structure."

	chapters := SplitChapters(text)

	if len(chapters) != 1 {
		t.Fatalf("expected 1 fallback chapter, got %d", len(chapters))
	}
	if chapters[0].Number != 1 {
		t.Errorf("fallback chapter number = %d, want 1", chapters[0].Number)
	}
	if chapters[0].Title != "" {
		t.Errorf("fallback chapter should have empty title, got %q", chapters[0].Title)
	}
	if chapters[0].Body != text {
		t.Errorf("fallback chapter body should be the full text")
	}
}

func TestSplitChapters_EmptyChaptersSkipped(t *testing.T) {
	// Two consecutive markers with no body between them
	text := "1.\n\n2.\n\nActual content here.\n\n3.\n\nMore content."

	chapters := SplitChapters(text)

	for _, ch := range chapters {
		if ch.Body == "" {
			t.Errorf("chapter %d has empty body, should have been skipped", ch.Number)
		}
	}
}

func TestSplitChapters_PreambleBeforeFirstMarker(t *testing.T) {
	text := "This is preamble text.\n\nMore preamble.\n\nI.\n\nChapter one.\n\nII.\n\nChapter two."

	chapters := SplitChapters(text)

	if len(chapters) < 2 {
		t.Fatalf("expected at least 2 chapters, got %d", len(chapters))
	}

	// First chapter should be preamble with empty title
	if chapters[0].Title != "" {
		t.Errorf("preamble should have empty title, got %q", chapters[0].Title)
	}
	if chapters[0].Number != 1 {
		t.Errorf("preamble number = %d, want 1", chapters[0].Number)
	}
}
