package db

import (
	"embed"
	"log"

	"github.com/lehmann314159/terve2/internal/gutenberg"
)

//go:embed bookdata/*.txt
var bookFiles embed.FS

type seedBook struct {
	Filename    string
	Title       string
	Author      string
	Description string
	GutenbergID int
	Difficulty  string
	Lang        string // "fi" or "es"; defaults to "fi"
}

var seedBookList = []seedBook{
	// --- Easy (A1–A2): Children's literature ---
	{
		Filename:    "bookdata/16223.txt",
		Title:       "Lukemisia lapsille 1",
		Author:      "Zacharias Topelius",
		Description: "Children's stories — among the easiest Finnish literature to read.",
		GutenbergID: 16223,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/16314.txt",
		Title:       "Lukemisia lapsille 2",
		Author:      "Zacharias Topelius",
		Description: "Second volume of Topelius's classic Finnish children's stories.",
		GutenbergID: 16314,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/51853.txt",
		Title:       "Lukemisia lapsille 3",
		Author:      "Zacharias Topelius",
		Description: "Third volume of Topelius's classic Finnish children's stories.",
		GutenbergID: 51853,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/51934.txt",
		Title:       "Lukemisia lapsille 4",
		Author:      "Zacharias Topelius",
		Description: "Fourth volume of Topelius's classic Finnish children's stories.",
		GutenbergID: 51934,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/52419.txt",
		Title:       "Lukemisia lapsille 6",
		Author:      "Zacharias Topelius",
		Description: "Sixth volume of Topelius's classic Finnish children's stories.",
		GutenbergID: 52419,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/52508.txt",
		Title:       "Lukemisia lapsille 7",
		Author:      "Zacharias Topelius",
		Description: "Seventh volume of Topelius's classic Finnish children's stories.",
		GutenbergID: 52508,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/17486.txt",
		Title:       "Lukemisia lapsille 8",
		Author:      "Zacharias Topelius",
		Description: "Eighth volume of Topelius's classic Finnish children's stories.",
		GutenbergID: 17486,
		Difficulty:  "A1",
	},
	{
		Filename:    "bookdata/59954.txt",
		Title:       "Suomen kansan peikkosatuja",
		Author:      "Iivo Härkönen",
		Description: "Finnish folk fairy tales featuring trolls and spirits.",
		GutenbergID: 59954,
		Difficulty:  "A2",
	},
	{
		Filename:    "bookdata/59917.txt",
		Title:       "Suomen kansan eläinsatuja",
		Author:      "Iivo Härkönen",
		Description: "Finnish animal folk tales.",
		GutenbergID: 59917,
		Difficulty:  "A2",
	},
	// --- Intermediate (A2–B1): Translated classics ---
	{
		Filename:    "bookdata/45046.txt",
		Title:       "Koti-satuja lapsille ja nuorisolle",
		Author:      "Jacob & Wilhelm Grimm",
		Description: "Grimm fairy tales translated into Finnish.",
		GutenbergID: 45046,
		Difficulty:  "A2",
	},
	{
		Filename:    "bookdata/53484.txt",
		Title:       "Satuja ja tarinoita I",
		Author:      "H. C. Andersen",
		Description: "Andersen's fairy tales in Finnish translation.",
		GutenbergID: 53484,
		Difficulty:  "A2",
	},
	{
		Filename:    "bookdata/46569.txt",
		Title:       "Liisan seikkailut ihmemaassa",
		Author:      "Lewis Carroll",
		Description: "Alice's Adventures in Wonderland in Finnish.",
		GutenbergID: 46569,
		Difficulty:  "B1",
	},
	{
		Filename:    "bookdata/48434.txt",
		Title:       "Pekka Poikanen",
		Author:      "J. M. Barrie",
		Description: "Peter Pan in Finnish translation.",
		GutenbergID: 48434,
		Difficulty:  "B1",
	},
	// --- Advanced (B2–C1): Finnish literary classics ---
	{
		Filename:    "bookdata/77618.txt",
		Title:       "Joulu-yön tarina",
		Author:      "Larin-Kyösti",
		Description: "A short Christmas story.",
		GutenbergID: 77618,
		Difficulty:  "B2",
	},
	{
		Filename:    "bookdata/77649.txt",
		Title:       "Aamusta iltaan",
		Author:      "Reino Rauanheimo",
		Description: "A novel following daily life from morning to evening.",
		GutenbergID: 77649,
		Difficulty:  "B2",
	},
	{
		Filename:    "bookdata/13593.txt",
		Title:       "Yksin",
		Author:      "Juhani Aho",
		Description: "A short, accessible prose work by one of Finland's most celebrated authors.",
		GutenbergID: 13593,
		Difficulty:  "B2",
	},
	{
		Filename:    "bookdata/13173.txt",
		Title:       "Anna Liisa",
		Author:      "Minna Canth",
		Description: "A short play dealing with guilt, morality, and redemption.",
		GutenbergID: 13173,
		Difficulty:  "C1",
	},
	{
		Filename:    "bookdata/11296.txt",
		Title:       "Työmiehen vaimo",
		Author:      "Minna Canth",
		Description: "A social realist drama exploring the life of a working man's wife.",
		GutenbergID: 11296,
		Difficulty:  "C1",
	},
	{
		Filename:    "bookdata/12688.txt",
		Title:       "Vänrikki Stoolin tarinat",
		Author:      "J.L. Runeberg",
		Description: "Classic Finnish poetry collection, tales of Ensign Stål.",
		GutenbergID: 12688,
		Difficulty:  "C1",
	},
	{
		Filename:    "bookdata/11940.txt",
		Title:       "Seitsemän veljestä",
		Author:      "Aleksis Kivi",
		Description: "The Finnish national novel about seven brothers growing up in rural Finland.",
		GutenbergID: 11940,
		Difficulty:  "C1",
	},
	// --- Spanish: Easy (A1–A2) ---
	{
		Filename:    "bookdata/55206.txt",
		Title:       "Fábulas",
		Author:      "Félix María de Samaniego",
		Description: "Classic Spanish verse fables in the tradition of Aesop — short, clear, and ideal for beginners.",
		GutenbergID: 55206,
		Difficulty:  "A1",
		Lang:        "es",
	},
	{
		Filename:    "bookdata/39209.txt",
		Title:       "Platero y yo",
		Author:      "Juan Ramón Jiménez",
		Description: "Lyrical prose vignettes about a poet and his donkey in Andalusia. Short chapters, vivid imagery.",
		GutenbergID: 39209,
		Difficulty:  "A2",
		Lang:        "es",
	},
	// --- Spanish: Intermediate (B1–B2) ---
	{
		Filename:    "bookdata/320.txt",
		Title:       "Lazarillo de Tormes",
		Author:      "Anonymous",
		Description: "The original Spanish picaresque novel — the life of a poor boy serving various masters. Short and fast-paced.",
		GutenbergID: 320,
		Difficulty:  "B1",
		Lang:        "es",
	},
	{
		Filename:    "bookdata/29506.txt",
		Title:       "El sombrero de tres picos",
		Author:      "Pedro Antonio de Alarcón",
		Description: "A comic novella of love, jealousy, and disguise in an Andalusian village. Lively 19th-century Spanish.",
		GutenbergID: 29506,
		Difficulty:  "B1",
		Lang:        "es",
	},
	{
		Filename:    "bookdata/17223.txt",
		Title:       "Pepita Jiménez",
		Author:      "Juan Valera",
		Description: "An epistolary novel about a young seminarian who falls in love. Elegant 19th-century prose.",
		GutenbergID: 17223,
		Difficulty:  "B2",
		Lang:        "es",
	},
	// --- Spanish: Advanced (C1) ---
	{
		Filename:    "bookdata/2000.txt",
		Title:       "Don Quijote de la Mancha",
		Author:      "Miguel de Cervantes",
		Description: "The masterpiece of Spanish literature — the adventures of the idealistic knight-errant and his loyal squire Sancho Panza.",
		GutenbergID: 2000,
		Difficulty:  "C1",
		Lang:        "es",
	},
}

// seedBooks inserts the curated starter books into the database (idempotent).
func (db *DB) seedBooks() error {
	for _, sb := range seedBookList {
		// If already seeded, backfill difficulty if missing.
		if db.BookExistsByGutenbergID(sb.GutenbergID) {
			if sb.Difficulty != "" {
				db.Exec(`UPDATE books SET difficulty = ? WHERE gutenberg_id = ? AND difficulty = ''`, sb.Difficulty, sb.GutenbergID)
			}
			continue
		}

		data, err := bookFiles.ReadFile(sb.Filename)
		if err != nil {
			log.Printf("seedBooks: read %s: %v", sb.Filename, err)
			continue
		}

		text := gutenberg.StripBoilerplate(string(data))
		chapters := gutenberg.SplitChapters(text)

		gid := sb.GutenbergID
		lang := sb.Lang
		if lang == "" {
			lang = "fi"
		}
		source := "seed"
		if lang == "es" {
			source = "seed_es"
		}
		bookID, err := db.InsertBook(sb.Title, sb.Author, sb.Description, &gid, source, lang)
		if err != nil {
			log.Printf("seedBooks: insert book %q: %v", sb.Title, err)
			continue
		}

		if sb.Difficulty != "" {
			if err := db.UpdateBookDifficulty(bookID, sb.Difficulty); err != nil {
				log.Printf("seedBooks: set difficulty for %q: %v", sb.Title, err)
			}
		}

		for _, ch := range chapters {
			if _, err := db.InsertChapter(bookID, ch.Number, ch.Title, ch.Body); err != nil {
				log.Printf("seedBooks: insert chapter %d of %q: %v", ch.Number, sb.Title, err)
			}
		}

		log.Printf("seedBooks: seeded %q (%d chapters)", sb.Title, len(chapters))
	}
	return nil
}
