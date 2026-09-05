package book

import (
	"fmt"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Compare full-TOC reads with production narrow queries on the current schema.
// This measures warm local storage, not HTTP latency.
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
			b.Run("chapter_pair", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					chapter, next, err := store.GetChapterWithNext(b.Context(), "fixture", target)
					if err != nil || chapter == nil || chapter.Index != target || next == nil || next.Index != target+1 {
						b.Fatalf("chapter pair: %+v %+v %v", chapter, next, err)
					}
				}
			})
			b.Run("readable_chapter", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					readable, err := store.HasReadableChapter(b.Context(), "fixture", target)
					if err != nil || !readable {
						b.Fatalf("readability: %v %v", readable, err)
					}
				}
			})
		})
	}
}
