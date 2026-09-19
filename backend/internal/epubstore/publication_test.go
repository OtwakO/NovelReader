package epubstore

import (
	"errors"
	"os"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/library"
)

func TestPublicationIdentityResourcesAndRemoval(t *testing.T) {
	for _, mode := range []epub.ImageMode{epub.OriginalImages, epub.OptimizedImages} {
		t.Run(string(mode), func(t *testing.T) {
			s, root := receiptStore(t)
			r := acquiredFixture(t, s, mode)
			old, err := s.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			a, err := s.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Accept(t.Context(), r.ID, a.Generation, "Book", "Author"); err == nil {
				t.Fatal("accepted queued preparation")
			}
			if err = s.Prepare(t.Context(), r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Accept(t.Context(), r.ID, old.Generation, "Book", "Author"); !errors.Is(err, ErrStateChanged) {
				t.Fatal("stale acceptance", err)
			}
			if _, _, err = s.GetCatalog(t.Context(), r.ID); !errors.Is(err, ErrNotFound) {
				t.Fatal("draft readable", err)
			}
			resources, err := s.preparedResources(t.Context(), r.ID, a.Generation)
			if err != nil {
				t.Fatal(err)
			}
			resource := resources[0]
			if _, _, err = s.ReadResource(t.Context(), r.ID, 1, resource.ID); !errors.Is(err, ErrNotFound) {
				t.Fatal("draft resource", err)
			}
			if _, err = s.db.Exec(`CREATE TRIGGER fail_publication BEFORE UPDATE OF library_id ON epub_files BEGIN SELECT RAISE(FAIL,'publication unavailable'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Accept(t.Context(), r.ID, a.Generation, "Book", "Author"); err == nil {
				t.Fatal("publication committed despite link failure")
			}
			if orphan, err := library.NewStore(s.db).Get(t.Context(), r.ID); err != nil || orphan != nil {
				t.Fatal("partial publication", orphan, err)
			}
			if _, err = s.db.Exec(`DROP TRIGGER fail_publication`); err != nil {
				t.Fatal(err)
			}
			item, err := s.Accept(t.Context(), r.ID, a.Generation, "Book", "Author")
			if err != nil {
				t.Fatal(err)
			}
			if item.Provider != library.EPUB || item.ContentRevision != 1 || a.Generation == item.ContentRevision || item.CoverURL != resource.ID {
				t.Fatal(item, a)
			}
			again, err := s.Accept(t.Context(), r.ID, a.Generation, "Different", "Other")
			if err != nil || again != item {
				t.Fatal("non-idempotent acceptance", again, err)
			}
			if err = s.Discard(t.Context(), r.ID); !errors.Is(err, ErrStateChanged) {
				t.Fatal("discarded publication", err)
			}
			if _, err = s.QueuePreparation(t.Context(), r.ID); !errors.Is(err, ErrStateChanged) {
				t.Fatal("published reprepare", err)
			}
			content, err := s.ReadSection(t.Context(), r.ID, 1, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(content.Resources) == 0 {
				t.Fatal("missing resource mapping")
			}
			for _, id := range content.Resources {
				if id != resource.ID {
					t.Fatal("wrong binding", id)
				}
			}
			data, mime, err := s.ReadResource(t.Context(), r.ID, 1, resource.ID)
			if err != nil || len(data) == 0 || mime != resource.Image.Info.MediaType {
				t.Fatal("image", mime, err)
			}
			if _, _, err = s.ReadResource(t.Context(), r.ID, 2, resource.ID); !errors.Is(err, library.ErrStateChanged) {
				t.Fatal("stale image", err)
			}
			if _, _, err = s.ReadResource(t.Context(), r.ID, 1, "../private/pic.png"); !errors.Is(err, ErrNotFound) {
				t.Fatal("unregistered image", err)
			}
			// Indexed chapter/content access must remain independent of whole-book JSON.
			var metadata string
			if err = s.db.QueryRow(`SELECT metadata_json FROM epub_preparations WHERE file_id=? AND generation=?`, r.ID, a.Generation).Scan(&metadata); err != nil {
				t.Fatal(err)
			}
			if _, err = s.db.Exec(`UPDATE epub_preparations SET metadata_json='invalid' WHERE file_id=? AND generation=?`, r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			if _, err = s.GetSection(t.Context(), r.ID, 1, 0); err != nil {
				t.Fatal(err)
			}
			if _, err = s.ReadSection(t.Context(), r.ID, 1, 0); err != nil {
				t.Fatal(err)
			}
			if _, err = s.db.Exec(`UPDATE epub_preparations SET metadata_json=? WHERE file_id=? AND generation=?`, metadata, r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			// Fail final bookkeeping after owned files are removed: intent must survive.
			if _, err = s.db.Exec(`CREATE TRIGGER fail_receipt_cleanup BEFORE DELETE ON epub_files BEGIN SELECT RAISE(FAIL,'cleanup unavailable'); END`); err != nil {
				t.Fatal(err)
			}
			pending, err := s.RemovePublication(t.Context(), r.ID)
			if !pending || err == nil {
				t.Fatal("cleanup failure not retained", pending, err)
			}
			hidden, err := library.NewStore(s.db).Get(t.Context(), r.ID)
			if err != nil || hidden != nil {
				t.Fatal("still visible", err)
			}
			remaining, err := s.Get(t.Context(), r.ID)
			if err != nil || remaining.State != Removing || remaining.LibraryID != "" {
				t.Fatal(remaining, err)
			}
			if _, _, err = s.ReadResource(t.Context(), r.ID, 1, resource.ID); !errors.Is(err, ErrNotFound) {
				t.Fatal("removed resource", err)
			}
			if _, err = s.db.Exec(`DROP TRIGGER fail_receipt_cleanup`); err != nil {
				t.Fatal(err)
			}
			if err = s.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			if pending, err = s.RemovePublication(t.Context(), r.ID); pending || err != nil {
				t.Fatal("retry", pending, err)
			}
			if _, err = root.Stat(r.Path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("original retained", err)
			}
		})
	}
}

func TestPortablePublicationBindingAndSectionMetadata(t *testing.T) {
	s, root := receiptStore(t)
	r := acquiredFixture(t, s, epub.OriginalImages)
	a, err := s.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Prepare(t.Context(), r.ID, a.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Accept(t.Context(), r.ID, a.Generation, "Book", "Author"); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{
		"", `UPDATE library_items SET provider='txt'`, `UPDATE epub_files SET library_id=NULL`,
		`UPDATE library_items SET total_chapter_num=99`, `UPDATE library_items SET cover_url='WRONG'`,
		`UPDATE library_items SET current_chapter_title='Wrong'`,
		`UPDATE epub_sections SET title='Wrong' WHERE ordinal=0`, `UPDATE epub_sections SET main=0 WHERE ordinal=0`,
	} {
		t.Run(mutation, func(t *testing.T) {
			tx, err := s.db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if mutation != "" {
				if _, err = tx.Exec(mutation); err != nil {
					t.Fatal(err)
				}
			}
			err = validatePortableFiles(t.Context(), tx, root)
			if (err == nil) != (mutation == "") {
				t.Fatal("portable validation", err)
			}
		})
	}
}
