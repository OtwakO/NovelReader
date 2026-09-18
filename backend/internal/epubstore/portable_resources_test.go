package epubstore

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func checkPortableResourceFixture(t *testing.T, s *Store, root *os.Root) error {
	t.Helper()
	tx, err := s.db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	return validatePortableResources(t.Context(), tx, root)
}

func TestPortableResourceBytes(t *testing.T) {
	for _, mode := range []epub.ImageMode{epub.OriginalImages, epub.OptimizedImages} {
		t.Run(string(mode), func(t *testing.T) {
			store, root := receiptStore(t)
			r := acquiredFixture(t, store, mode)
			a, err := store.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Prepare(t.Context(), r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableResourceFixture(t, store, root); err != nil {
				t.Fatal("valid resource rejected", err)
			}
			if _, err = store.db.Exec(`UPDATE epub_resources SET width=width+1`); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableResourceFixture(t, store, root); err == nil {
				t.Fatal("forged dimensions accepted")
			}
			if _, err = store.db.Exec(`UPDATE epub_resources SET width=width-1`); err != nil {
				t.Fatal(err)
			}
			resources, err := store.preparedResources(t.Context(), r.ID, a.Generation)
			if err != nil || len(resources) != 1 {
				t.Fatal(resources, err)
			}
			name := r.Path
			if mode == epub.OptimizedImages {
				name = path.Join(preparationPath(r.ID, a.Generation), "images", resources[0].ID+".webp")
			}
			data, err := root.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if err = root.WriteFile(name, make([]byte, len(data)), 0600); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableResourceFixture(t, store, root); err == nil {
				t.Fatal("invalid bytes of correct size accepted")
			}
			if _, err = store.db.Exec(`UPDATE epub_preparations SET state='finalizing'`); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableResourceFixture(t, store, root); err == nil {
				t.Fatal("invalid bytes accepted for stat-only recovery")
			}
			if mode == epub.OptimizedImages {
				if err = root.Remove(name); err != nil {
					t.Fatal(err)
				}
				if err = checkPortableResourceFixture(t, store, root); err != nil {
					t.Fatal("missing interrupted derivative rejected", err)
				}
				if err = store.RecoverPreparations(t.Context()); err != nil {
					t.Fatal(err)
				}
				got, err := store.GetPreparation(t.Context(), r.ID, a.Generation)
				if err != nil || got.State != PreparationFailed {
					t.Fatal(got, err)
				}
				if err = checkPortableResourceFixture(t, store, root); err != nil {
					t.Fatal("failed output evidence rejected", err)
				}
			}
		})
	}
}
