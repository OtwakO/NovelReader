package book

import "testing"

func TestCatalogReplacementParticipatesInCallerTransaction(t *testing.T) {
	for _, commit := range []bool{false, true} {
		name := "rollback"
		if commit {
			name = "commit"
		}
		t.Run(name, func(t *testing.T) {
			store := newCatalogStore(t)
			addCatalogBook(t, store, "source", "https://unused.test/toc")
			if err := store.SaveCatalog("book-1", "source", 0, []Chapter{{Index: 0, Title: "Original"}}); err != nil {
				t.Fatal(err)
			}
			tx, err := store.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err := replaceChaptersTx(tx, "book-1", []Chapter{{Index: 0, Title: "Replacement"}, {Index: 1, Title: "Second"}}, true); err != nil {
				t.Fatal(err)
			}
			// Another owner must be able to change its state in the same
			// transaction; catalog persistence must not commit ahead of it.
			if _, err := tx.Exec(`UPDATE books SET variable_map = ? WHERE id = 'book-1'`, `{"key":"value"}`); err != nil {
				t.Fatalf("catalog ended its caller's transaction: %v", err)
			}
			if commit {
				err = tx.Commit()
			} else {
				err = tx.Rollback()
			}
			if err != nil {
				t.Fatal(err)
			}
			stored, err := store.GetBook("book-1")
			if err != nil || stored == nil {
				t.Fatalf("load book: %v", err)
			}
			chapters, err := store.GetChapters("book-1")
			if err != nil {
				t.Fatal(err)
			}
			wantTitle, wantCount, wantVariables := "Original", 1, ""
			if commit {
				wantTitle, wantCount, wantVariables = "Replacement", 2, `{"key":"value"}`
			}
			if len(chapters) != wantCount || chapters[0].Title != wantTitle || stored.TotalChapterNum != wantCount || stored.VariableMap != wantVariables {
				t.Fatalf("catalog/state were not atomic: book=%+v chapters=%+v", stored, chapters)
			}
		})
	}
}

func TestShelfAdmissionRollsBackInvalidCatalog(t *testing.T) {
	store := newCatalogStore(t)
	candidate := Book{ID: "new-book", Name: "New Book", SourceID: "source", SourceURL: "https://unused.test", BookURL: "https://unused.test/book"}
	_, _, err := store.AddOrMergeBookWithChapters(&candidate, []Chapter{
		{ID: "duplicate", Index: 0, Title: "First"},
		{ID: "duplicate", Index: 1, Title: "Second"},
	})
	if err == nil {
		t.Fatal("invalid catalog was admitted")
	}
	stored, err := store.GetBook(candidate.ID)
	if err != nil || stored != nil {
		t.Fatalf("partial shelf admission: book=%+v error=%v", stored, err)
	}
	chapters, err := store.GetChapters(candidate.ID)
	if err != nil || len(chapters) != 0 {
		t.Fatalf("partial catalog: chapters=%+v error=%v", chapters, err)
	}
}
