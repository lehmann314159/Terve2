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
}

var seedBookList = []seedBook{
	{
		Filename:    "bookdata/11940.txt",
		Title:       "Seitsemän veljestä",
		Author:      "Aleksis Kivi",
		Description: "The Finnish national novel about seven brothers growing up in rural Finland.",
		GutenbergID: 11940,
	},
	{
		Filename:    "bookdata/13593.txt",
		Title:       "Yksin",
		Author:      "Juhani Aho",
		Description: "A short, accessible prose work by one of Finland's most celebrated authors.",
		GutenbergID: 13593,
	},
	{
		Filename:    "bookdata/11296.txt",
		Title:       "Työmiehen vaimo",
		Author:      "Minna Canth",
		Description: "A social realist drama exploring the life of a working man's wife.",
		GutenbergID: 11296,
	},
	{
		Filename:    "bookdata/13173.txt",
		Title:       "Anna Liisa",
		Author:      "Minna Canth",
		Description: "A short play dealing with guilt, morality, and redemption.",
		GutenbergID: 13173,
	},
	{
		Filename:    "bookdata/16223.txt",
		Title:       "Lukemisia lapsille 1",
		Author:      "Zacharias Topelius",
		Description: "Children's stories — among the easiest Finnish literature to read.",
		GutenbergID: 16223,
	},
	{
		Filename:    "bookdata/12688.txt",
		Title:       "Vänrikki Stoolin tarinat",
		Author:      "J.L. Runeberg",
		Description: "Classic Finnish poetry collection, tales of Ensign Stål.",
		GutenbergID: 12688,
	},
	{
		Filename:    "bookdata/77649.txt",
		Title:       "Aamusta iltaan",
		Author:      "Reino Rauanheimo",
		Description: "A novel following daily life from morning to evening.",
		GutenbergID: 77649,
	},
	{
		Filename:    "bookdata/77618.txt",
		Title:       "Joulu-yön tarina",
		Author:      "Larin-Kyösti",
		Description: "A short Christmas story.",
		GutenbergID: 77618,
	},
}

// seedBooks inserts the curated starter books into the database (idempotent).
func (db *DB) seedBooks() error {
	for _, sb := range seedBookList {
		// Skip if already seeded
		if db.BookExistsByGutenbergID(sb.GutenbergID) {
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
		bookID, err := db.InsertBook(sb.Title, sb.Author, sb.Description, &gid, "seed")
		if err != nil {
			log.Printf("seedBooks: insert book %q: %v", sb.Title, err)
			continue
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
