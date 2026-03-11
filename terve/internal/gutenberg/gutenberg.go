package gutenberg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Chapter represents a single chapter extracted from a Gutenberg text.
type Chapter struct {
	Number int
	Title  string
	Body   string
}

// SearchResult represents a single book from the Gutendex API.
type SearchResult struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Authors []Author `json:"authors"`
}

// Author is a Gutendex author entry.
type Author struct {
	Name string `json:"name"`
}

type gutendexResponse struct {
	Results []SearchResult `json:"results"`
}

// Download fetches the plain-text UTF-8 version of a Gutenberg book.
func Download(id int) (string, error) {
	url := fmt.Sprintf("https://www.gutenberg.org/ebooks/%d.txt.utf-8", id)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("gutenberg download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gutenberg download: HTTP %d for ID %d", resp.StatusCode, id)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gutenberg download: read body: %w", err)
	}
	// Normalize Windows line endings (Gutenberg texts use \r\n).
	return strings.ReplaceAll(string(body), "\r", ""), nil
}

// StripBoilerplate removes the Project Gutenberg header and footer.
func StripBoilerplate(text string) string {
	// Find start marker
	startIdx := strings.Index(text, "*** START OF")
	if startIdx == -1 {
		startIdx = strings.Index(text, "***START OF")
	}
	if startIdx != -1 {
		// Skip to the end of the line containing the marker
		nlIdx := strings.Index(text[startIdx:], "\n")
		if nlIdx != -1 {
			text = text[startIdx+nlIdx+1:]
		}
	}

	// Find end marker
	endIdx := strings.Index(text, "*** END OF")
	if endIdx == -1 {
		endIdx = strings.Index(text, "***END OF")
	}
	if endIdx != -1 {
		text = text[:endIdx]
	}

	return strings.TrimSpace(text)
}

// Finnish ordinal chapter words mapped to numbers.
var finnishOrdinals = map[string]int{
	"ENSIMMÄINEN": 1, "TOINEN": 2, "KOLMAS": 3, "NELJÄS": 4,
	"VIIDES": 5, "KUUDES": 6, "SEITSEMÄS": 7, "KAHDEKSAS": 8,
	"YHDEKSÄS": 9, "KYMMENES": 10, "YHDESTOISTA": 11, "KAHDESTOISTA": 12,
	"KOLMASTOISTA": 13, "NELJÄSTOISTA": 14, "VIIDESTOISTA": 15,
	"KUUDESTOISTA": 16, "SEITSEMÄSTOISTA": 17, "KAHDEKSASTOISTA": 18,
	"YHDEKSÄSTOISTA": 19, "KAHDESKYMMENES": 20,
}

// Chapter-matching regexps (compiled once).
var (
	// "ENSIMMÄINEN LUKU" etc.
	reFinnishChapter = regexp.MustCompile(`(?m)^[ \t]*(` + buildOrdinalPattern() + `)\s+LUKU\.?\s*$`)
	// Roman numerals as section headers: "I.", "II.", "III." (alone on line)
	reRoman = regexp.MustCompile(`(?m)^[ \t]*((?:X{0,3})(?:IX|IV|V?I{0,3}))\.?\s*$`)
	// Numbered chapters: "1.", "2." (alone on line, at least 2 blank lines before)
	reNumbered = regexp.MustCompile(`(?m)^[ \t]*(\d{1,3})\.[ \t]*$`)
	// Standalone ALL-CAPS lines (min 3 chars, no lowercase)
	reAllCaps = regexp.MustCompile(`(?m)^[ \t]*([A-ZÄÖÅÜ][A-ZÄÖÅÜ\s,.\-:!?]{2,})[ \t]*$`)
)

func buildOrdinalPattern() string {
	words := make([]string, 0, len(finnishOrdinals))
	for w := range finnishOrdinals {
		words = append(words, w)
	}
	return strings.Join(words, "|")
}

// SplitChapters divides stripped text into chapters.
// It tries multiple heuristics in order of specificity.
func SplitChapters(text string) []Chapter {
	// Try Finnish ordinal chapters first
	if chapters := splitByRegex(text, reFinnishChapter); len(chapters) > 1 {
		return mergeShortChapters(chapters, 200)
	}

	// Try roman numeral sections
	if chapters := splitByRoman(text); len(chapters) > 1 {
		return mergeShortChapters(chapters, 200)
	}

	// Try numbered chapters
	if chapters := splitByRegex(text, reNumbered); len(chapters) > 1 {
		return mergeShortChapters(chapters, 200)
	}

	// Try ALL-CAPS section headers (but be conservative — need at least 3 sections)
	if chapters := splitByAllCaps(text); len(chapters) >= 3 {
		return mergeShortChapters(chapters, 200)
	}

	// Fallback: single chapter
	return []Chapter{{Number: 1, Title: "", Body: text}}
}

// mergeShortChapters collapses chapters whose body is shorter than minBody
// bytes into their neighbours. Short chapters at the start or middle are
// merged forward (prepended to the next chapter); a short final chapter is
// merged backward. After merging, chapters are renumbered sequentially.
func mergeShortChapters(chapters []Chapter, minBody int) []Chapter {
	if len(chapters) == 0 {
		return chapters
	}

	merged := make([]Chapter, 0, len(chapters))
	var carry string // accumulated tiny-chapter text to prepend to the next real chapter

	for i, ch := range chapters {
		if len(ch.Body) < minBody {
			// Accumulate this chapter's body into carry
			if carry != "" {
				carry += "\n\n" + ch.Body
			} else {
				carry = ch.Body
			}
			// If this is the last chapter and we have a carry, append to previous
			if i == len(chapters)-1 && len(merged) > 0 {
				merged[len(merged)-1].Body += "\n\n" + carry
				carry = ""
			}
			continue
		}

		// Real chapter — prepend any accumulated carry
		if carry != "" {
			ch.Body = carry + "\n\n" + ch.Body
			carry = ""
		}
		merged = append(merged, ch)
	}

	// If everything was tiny (carry still set, merged empty), return as single chapter
	if len(merged) == 0 && carry != "" {
		return []Chapter{{Number: 1, Title: "", Body: carry}}
	}

	// Renumber sequentially
	for i := range merged {
		merged[i].Number = i + 1
	}

	return merged
}

func splitByRegex(text string, re *regexp.Regexp) []Chapter {
	locs := re.FindAllStringIndex(text, -1)
	if len(locs) < 2 {
		return nil
	}

	matches := re.FindAllStringSubmatch(text, -1)
	var chapters []Chapter

	// Content before first chapter marker
	if preamble := strings.TrimSpace(text[:locs[0][0]]); preamble != "" {
		chapters = append(chapters, Chapter{Number: 1, Title: "", Body: preamble})
	}

	for i, loc := range locs {
		title := strings.TrimSpace(matches[i][0])
		var body string
		if i+1 < len(locs) {
			body = text[loc[1]:locs[i+1][0]]
		} else {
			body = text[loc[1]:]
		}
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		num := len(chapters) + 1
		chapters = append(chapters, Chapter{Number: num, Title: title, Body: body})
	}

	return chapters
}

var romanValues = map[string]int{
	"I": 1, "II": 2, "III": 3, "IV": 4, "V": 5,
	"VI": 6, "VII": 7, "VIII": 8, "IX": 9, "X": 10,
	"XI": 11, "XII": 12, "XIII": 13, "XIV": 14, "XV": 15,
	"XVI": 16, "XVII": 17, "XVIII": 18, "XIX": 19, "XX": 20,
	"XXI": 21, "XXII": 22, "XXIII": 23, "XXIV": 24, "XXV": 25,
	"XXVI": 26, "XXVII": 27, "XXVIII": 28, "XXIX": 29, "XXX": 30,
}

func splitByRoman(text string) []Chapter {
	locs := reRoman.FindAllStringIndex(text, -1)
	matches := reRoman.FindAllStringSubmatch(text, -1)

	// Filter to only valid roman numerals
	var validLocs [][]int
	var validMatches [][]string
	for i, m := range matches {
		numeral := strings.TrimRight(m[1], ".")
		if _, ok := romanValues[numeral]; ok && numeral != "" {
			validLocs = append(validLocs, locs[i])
			validMatches = append(validMatches, m)
		}
	}

	if len(validLocs) < 2 {
		return nil
	}

	var chapters []Chapter
	if preamble := strings.TrimSpace(text[:validLocs[0][0]]); preamble != "" {
		chapters = append(chapters, Chapter{Number: 1, Title: "", Body: preamble})
	}

	for i, loc := range validLocs {
		title := strings.TrimSpace(validMatches[i][1])
		var body string
		if i+1 < len(validLocs) {
			body = text[loc[1]:validLocs[i+1][0]]
		} else {
			body = text[loc[1]:]
		}
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		num := len(chapters) + 1
		chapters = append(chapters, Chapter{Number: num, Title: title, Body: body})
	}

	return chapters
}

func splitByAllCaps(text string) []Chapter {
	locs := reAllCaps.FindAllStringIndex(text, -1)
	matches := reAllCaps.FindAllStringSubmatch(text, -1)

	// Filter out very short matches and lines that are just punctuation
	var validLocs [][]int
	var validTitles []string
	for i, m := range matches {
		title := strings.TrimSpace(m[1])
		// Must have at least 2 alphabetic characters
		alphaCount := 0
		for _, r := range title {
			if unicode.IsLetter(r) {
				alphaCount++
			}
		}
		if alphaCount >= 3 {
			validLocs = append(validLocs, locs[i])
			validTitles = append(validTitles, title)
		}
	}

	if len(validLocs) < 3 {
		return nil
	}

	var chapters []Chapter
	if preamble := strings.TrimSpace(text[:validLocs[0][0]]); preamble != "" {
		chapters = append(chapters, Chapter{Number: 1, Title: "", Body: preamble})
	}

	for i, loc := range validLocs {
		title := validTitles[i]
		var body string
		if i+1 < len(validLocs) {
			body = text[loc[1]:validLocs[i+1][0]]
		} else {
			body = text[loc[1]:]
		}
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		num := len(chapters) + 1
		chapters = append(chapters, Chapter{Number: num, Title: title, Body: body})
	}

	return chapters
}

// Search queries the Gutendex API for Finnish books matching the query string.
func Search(query string) ([]SearchResult, error) {
	url := fmt.Sprintf("https://gutendex.com/books/?languages=fi&search=%s", query)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("gutendex search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gutendex search: HTTP %d", resp.StatusCode)
	}

	var result gutendexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gutendex search: decode: %w", err)
	}

	return result.Results, nil
}
