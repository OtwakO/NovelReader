package epubstore

import (
	"database/sql"
	"os"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func checkPortableOwnership(t *testing.T, s *Store, root *os.Root) error {
	t.Helper()
	tx, err := s.db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	return validatePortableOwnership(t.Context(), tx, root)
}

func TestPortableOwnershipRejectsBrokenReferences(t *testing.T) {
	for _, problem := range []string{"missing-original", "wrong-size", "missing-current", "future-generation", "changed-policy", "running", "local-staging", "superseded-active"} {
		t.Run(problem, func(t *testing.T) {
			store, root := receiptStore(t)
			r := acquiredFixture(t, store, epub.OriginalImages)
			a, err := store.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = checkPortableOwnership(t, store, root); err != nil {
				t.Fatal("valid ownership rejected", err)
			}
			switch problem {
			case "missing-original":
				err = root.Remove(r.Path)
			case "wrong-size":
				err = root.WriteFile(r.Path, []byte("damaged"), 0600)
			case "missing-current":
				_, err = store.db.Exec(`UPDATE epub_files SET preparation_generation=2 WHERE id=?`, r.ID)
			case "future-generation":
				_, err = store.db.Exec(`UPDATE epub_files SET preparation_generation=0 WHERE id=?`, r.ID)
			case "changed-policy":
				_, err = store.db.Exec(`UPDATE epub_preparations SET image_mode='optimized' WHERE file_id=?`, r.ID)
			case "running":
				_, err = store.ClaimPreparation(t.Context(), r.ID, a.Generation)
			case "local-staging":
				_, err = store.db.Exec(`UPDATE epub_preparations SET state='finalizing',format_version=1,metadata_json='{}',stream_size=1,stage_name='epub-LOCAL' WHERE file_id=?`, r.ID)
			case "superseded-active":
				_, err = store.QueuePreparation(t.Context(), r.ID)
				if err == nil {
					err = checkPortableOwnership(t, store, root)
				} // failed history is legal
				if err == nil {
					_, err = store.db.Exec(`UPDATE epub_preparations SET state='queued' WHERE file_id=? AND generation=?`, r.ID, a.Generation)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = checkPortableOwnership(t, store, root); err == nil {
				t.Fatal("accepted broken ownership")
			}
		})
	}
}

func TestPortableOwnershipPreservesInterruptedAcquisitions(t *testing.T) {
	for _, state := range []AcquisitionState{Receiving, Finalizing, Failed, Removing} {
		t.Run(string(state), func(t *testing.T) {
			store, root := receiptStore(t)
			r := acquiredFixture(t, store, epub.OriginalImages)
			if _, err := store.db.Exec(`UPDATE epub_files SET state=? WHERE id=?`, state, r.ID); err != nil {
				t.Fatal(err)
			}
			if state == Failed || state == Removing {
				if err := root.WriteFile(r.Path, []byte("damaged bytes retained for cleanup"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := checkPortableOwnership(t, store, root); err != nil {
				t.Fatal("retained original rejected", err)
			}
			if err := root.Remove(r.Path); err != nil {
				t.Fatal(err)
			}
			if err := checkPortableOwnership(t, store, root); err != nil {
				t.Fatal("missing interrupted original rejected", err)
			}
		})
	}
}

func TestPortableOwnershipRejectsOrphanPreparation(t *testing.T) {
	store, root := receiptStore(t)
	tx, err := store.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// Model inconsistent imported rows without weakening the fixture's schema.
	if _, err = tx.Exec(`PRAGMA defer_foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO epub_preparations(file_id,generation,state,image_mode,created_at,updated_at) VALUES('ORPHAN',1,'queued','original',0,0)`); err != nil {
		t.Fatal(err)
	}
	if err = validatePortableOwnership(t.Context(), tx, root); err == nil {
		t.Fatal("orphan preparation accepted")
	}
}
