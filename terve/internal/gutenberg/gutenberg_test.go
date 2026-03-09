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
	body := "Tämä on riittävän pitkä kappale suomalaista tekstiä, joka ylittää kaksisataa tavua. " +
		"Tarvitsemme tämän varmistamaan, että yhdistämislogiikka ei tiivistä näitä lukuja yhteen. " +
		"Lisää täytetekstiä tähän kohtaan varmuuden vuoksi."

	text := "Preamble text here.\n\nENSIMMÄINEN LUKU\n\n" + body + "\n\nTOINEN LUKU\n\n" + body + "\n\nKOLMAS LUKU\n\n" + body

	chapters := SplitChapters(text)

	// Preamble is short and gets merged into the first real chapter
	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}

	// Verify chapter titles contain the Finnish ordinals
	if chapters[0].Title != "ENSIMMÄINEN LUKU" {
		t.Errorf("chapter 1 title = %q, want %q", chapters[0].Title, "ENSIMMÄINEN LUKU")
	}
	if chapters[1].Title != "TOINEN LUKU" {
		t.Errorf("chapter 2 title = %q, want %q", chapters[1].Title, "TOINEN LUKU")
	}
}

func TestSplitChapters_RomanNumerals(t *testing.T) {
	body := "Tämä on riittävän pitkä kappale suomalaista tekstiä, joka ylittää kaksisataa tavua. " +
		"Tarvitsemme tämän varmistamaan, että yhdistämislogiikka ei tiivistä näitä lukuja yhteen. " +
		"Lisää täytetekstiä tähän kohtaan varmuuden vuoksi."

	text := "I.\n\n" + body + "\n\nII.\n\n" + body + "\n\nIII.\n\n" + body

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}

	if chapters[0].Title != "I" {
		t.Errorf("chapter 1 title = %q, want %q", chapters[0].Title, "I")
	}
}

func TestSplitChapters_Numbered(t *testing.T) {
	body := "Tämä on riittävän pitkä kappale suomalaista tekstiä, joka ylittää kaksisataa tavua. " +
		"Tarvitsemme tämän varmistamaan, että yhdistämislogiikka ei tiivistä näitä lukuja yhteen. " +
		"Lisää täytetekstiä tähän kohtaan varmuuden vuoksi."

	text := "1.\n\n" + body + "\n\n2.\n\n" + body + "\n\n3.\n\n" + body

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}
}

func TestSplitChapters_AllCaps(t *testing.T) {
	// Build bodies that exceed the 200-byte merge threshold
	longBody := "This is a sufficiently long body that exceeds the two-hundred byte minimum threshold for chapter merging. " +
		"We need to ensure it is long enough so that the merge logic does not collapse these chapters together. Extra padding here."

	text := "ENSIMMÄINEN OSASTO\n\n" + longBody + "\n\nTOINEN OSASTO\n\n" + longBody + "\n\nKOLMAS OSASTO\n\n" + longBody

	chapters := SplitChapters(text)

	if len(chapters) < 3 {
		t.Fatalf("expected at least 3 chapters, got %d", len(chapters))
	}

	// Verify tiny preamble chapters are merged away
	for _, ch := range chapters {
		if len(ch.Body) < 200 {
			t.Errorf("chapter %d body too short (%d bytes), should have been merged", ch.Number, len(ch.Body))
		}
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
	body := "Tämä on riittävän pitkä kappale suomalaista tekstiä, joka ylittää kaksisataa tavua. " +
		"Tarvitsemme tämän varmistamaan, että yhdistämislogiikka ei tiivistä näitä lukuja yhteen. " +
		"Lisää täytetekstiä tähän kohtaan varmuuden vuoksi."

	// Two consecutive markers with no body between them
	text := "1.\n\n2.\n\n" + body + "\n\n3.\n\n" + body

	chapters := SplitChapters(text)

	for _, ch := range chapters {
		if ch.Body == "" {
			t.Errorf("chapter %d has empty body, should have been skipped", ch.Number)
		}
	}
}

func TestSplitChapters_PreambleBeforeFirstMarker(t *testing.T) {
	body := "Tämä on riittävän pitkä kappale suomalaista tekstiä, joka ylittää kaksisataa tavua. " +
		"Tarvitsemme tämän varmistamaan, että yhdistämislogiikka ei tiivistä näitä lukuja yhteen. " +
		"Lisää täytetekstiä tähän kohtaan varmuuden vuoksi."

	text := "This is preamble text.\n\nMore preamble.\n\nI.\n\n" + body + "\n\nII.\n\n" + body

	chapters := SplitChapters(text)

	if len(chapters) < 2 {
		t.Fatalf("expected at least 2 chapters, got %d", len(chapters))
	}

	// Short preamble gets merged into first real chapter, so first chapter has title "I"
	if chapters[0].Title != "I" {
		t.Errorf("first chapter title = %q, want %q (preamble merged)", chapters[0].Title, "I")
	}
	if chapters[0].Number != 1 {
		t.Errorf("first chapter number = %d, want 1", chapters[0].Number)
	}
}

func TestMergeShortChapters(t *testing.T) {
	long := "This text is long enough to exceed the two-hundred byte minimum threshold we use for merging. " +
		"It represents a real chapter with actual content that should be preserved as its own standalone chapter entry."

	t.Run("short chapters at beginning merge into next", func(t *testing.T) {
		chapters := []Chapter{
			{Number: 1, Title: "Title Page", Body: "short"},
			{Number: 2, Title: "Author", Body: "also short"},
			{Number: 3, Title: "Chapter 1", Body: long},
		}
		got := mergeShortChapters(chapters, 200)
		if len(got) != 1 {
			t.Fatalf("expected 1 chapter, got %d", len(got))
		}
		if got[0].Number != 1 {
			t.Errorf("number = %d, want 1", got[0].Number)
		}
		if got[0].Title != "Chapter 1" {
			t.Errorf("title = %q, want %q", got[0].Title, "Chapter 1")
		}
		if len(got[0].Body) <= len(long) {
			t.Error("merged chapter should contain prepended short content")
		}
	})

	t.Run("short chapter at end merges into previous", func(t *testing.T) {
		chapters := []Chapter{
			{Number: 1, Title: "Chapter 1", Body: long},
			{Number: 2, Title: "Epilogue", Body: "tiny"},
		}
		got := mergeShortChapters(chapters, 200)
		if len(got) != 1 {
			t.Fatalf("expected 1 chapter, got %d", len(got))
		}
		if got[0].Title != "Chapter 1" {
			t.Errorf("title = %q, want %q", got[0].Title, "Chapter 1")
		}
		if len(got[0].Body) <= len(long) {
			t.Error("merged chapter should contain appended short content")
		}
	})

	t.Run("all short chapters collapse to single chapter", func(t *testing.T) {
		chapters := []Chapter{
			{Number: 1, Title: "A", Body: "one"},
			{Number: 2, Title: "B", Body: "two"},
			{Number: 3, Title: "C", Body: "three"},
		}
		got := mergeShortChapters(chapters, 200)
		if len(got) != 1 {
			t.Fatalf("expected 1 chapter, got %d", len(got))
		}
		if got[0].Body != "one\n\ntwo\n\nthree" {
			t.Errorf("body = %q", got[0].Body)
		}
	})

	t.Run("chapters above threshold are untouched", func(t *testing.T) {
		long2 := long + " Second chapter content that is also sufficiently long."
		chapters := []Chapter{
			{Number: 1, Title: "Ch1", Body: long},
			{Number: 2, Title: "Ch2", Body: long2},
		}
		got := mergeShortChapters(chapters, 200)
		if len(got) != 2 {
			t.Fatalf("expected 2 chapters, got %d", len(got))
		}
		if got[0].Body != long {
			t.Error("chapter 1 body should be unchanged")
		}
		if got[1].Body != long2 {
			t.Error("chapter 2 body should be unchanged")
		}
	})

	t.Run("renumbering is sequential after merge", func(t *testing.T) {
		long2 := long + " More unique content for chapter two."
		chapters := []Chapter{
			{Number: 1, Title: "Preamble", Body: "tiny"},
			{Number: 2, Title: "Ch1", Body: long},
			{Number: 3, Title: "Interlude", Body: "small"},
			{Number: 4, Title: "Ch2", Body: long2},
		}
		got := mergeShortChapters(chapters, 200)
		if len(got) != 2 {
			t.Fatalf("expected 2 chapters, got %d", len(got))
		}
		for i, ch := range got {
			if ch.Number != i+1 {
				t.Errorf("chapter[%d].Number = %d, want %d", i, ch.Number, i+1)
			}
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		got := mergeShortChapters(nil, 200)
		if len(got) != 0 {
			t.Fatalf("expected 0 chapters, got %d", len(got))
		}
	})
}
