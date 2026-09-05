package book

import (
	"fmt"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Compare the current full-TOC read with narrow SQL prototypes, before and
// after a composite index. This measures warm local storage, not HTTP latency.
func BenchmarkChapterLookup(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			manager, err := readerstore.NewManager(b.TempDir(), 1, ReaderSchema())
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { manager.Close() })
			const reader readerstore.UserID = "11111111-1111-4111-8111-111111111111"
			if err := manager.Create(b.Context(), reader); err != nil {
				b.Fatal(err)
			}
			home, err := manager.Open(b.Context(), reader)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { home.Close() })
			store := NewStore(home.DB())
			if err := store.AddBook(&Book{ID: "fixture", Name: "Synthetic benchmark"}); err != nil {
				b.Fatal(err)
			}
			chapters := make([]Chapter, count)
			for i := range chapters {
				chapters[i] = Chapter{Index: i, Title: fmt.Sprintf("Chapter %d", i), URL: fmt.Sprintf("https://example.test/chapter/%d", i)}
			}
			if err := store.SaveChapters("fixture", chapters); err != nil {
				b.Fatal(err)
			}
			target := count / 2
			b.Run("full_catalog", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rows, err := store.GetChapters("fixture")
					if err != nil || len(rows) != count {
						b.Fatalf("full catalog: rows=%d error=%v", len(rows), err)
					}
				}
			})
			for _, indexed := range []bool{false, true} {
				label := "existing_index"
				if indexed {
					if _, err := home.DB().Exec(`CREATE INDEX benchmark_chapters_book_index ON chapters(book_id, idx)`); err != nil {
						b.Fatal(err)
					}
					label = "composite_index"
				}
				b.Run(label+"/chapter_pair", func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						rows, err := home.DB().QueryContext(b.Context(), `SELECT `+chapterColumns+` FROM chapters WHERE book_id = ? AND idx >= ? ORDER BY idx LIMIT 2`, "fixture", target)
						if err != nil {
							b.Fatal(err)
						}
						pair, err := scanChapters(rows)
						closeErr := rows.Close()
						if err != nil || closeErr != nil || len(pair) != 2 || pair[0].Index != target {
							b.Fatalf("chapter pair: %v %v %v", pair, err, closeErr)
						}
					}
				})
				b.Run(label+"/readable_chapter", func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						var readable bool
						err := home.DB().QueryRowContext(b.Context(), `SELECT EXISTS(SELECT 1 FROM chapters WHERE book_id = ? AND idx = ? AND is_volume = 0)`, "fixture", target).Scan(&readable)
						if err != nil || !readable {
							b.Fatalf("readability: %v %v", readable, err)
						}
					}
				})
			}
		})
	}
}
