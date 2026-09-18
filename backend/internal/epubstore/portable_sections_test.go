package epubstore

import (
	"database/sql"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func checkPortableSectionIndexes(t *testing.T, s *Store) error {
	t.Helper()
	tx, err := s.db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	return validatePortableSectionIndexes(t.Context(), tx)
}

func TestPortableSectionIndexes(t *testing.T) {
	for _, test := range []struct{ name, mutation string }{
		{"ordinal-gap", `UPDATE epub_sections SET ordinal=ordinal+1 WHERE ordinal=1`},
		{"overlap", `UPDATE epub_sections SET offset=offset-1 WHERE ordinal=1`},
		{"missing-tail", `DELETE FROM epub_sections WHERE ordinal=1`},
		{"oversized-span", `UPDATE epub_sections SET length=9223372036854775807 WHERE ordinal=0`},
		{"unknown-format", `UPDATE epub_preparations SET format_version=2`},
		{"queued-output", `UPDATE epub_preparations SET state='queued'`},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, _ := receiptStore(t)
			r := acquiredFixture(t, store, epub.OriginalImages)
			a, err := store.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Prepare(t.Context(), r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableSectionIndexes(t, store); err != nil {
				t.Fatal("valid index rejected", err)
			}
			if _, err = store.db.Exec(test.mutation); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableSectionIndexes(t, store); err == nil {
				t.Fatal("invalid index accepted")
			}
		})
	}
}

func TestPortableSectionIndexesRetainFailedOutputEvidence(t *testing.T) {
	store, root := receiptStore(t)
	r := acquiredFixture(t, store, epub.OriginalImages)
	a, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Prepare(t.Context(), r.ID, a.Generation); err != nil {
		t.Fatal(err)
	}
	// Recovery can discard incomplete output while retaining its durable index.
	if _, err = store.db.Exec(`UPDATE epub_preparations SET state='failed' WHERE file_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if err = root.RemoveAll(preparationPath(r.ID, a.Generation)); err != nil {
		t.Fatal(err)
	}
	next, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkPortableSectionIndexes(t, store); err != nil {
		t.Fatal("failed output evidence rejected", err)
	}
	// The new queued generation must not claim any output ranges.
	if _, err = store.db.Exec(`INSERT INTO epub_sections(file_id,generation,ordinal,offset,length) VALUES(?,?,0,0,1)`, r.ID, next.Generation); err != nil {
		t.Fatal(err)
	}
	if err = checkPortableSectionIndexes(t, store); err == nil {
		t.Fatal("queued section accepted")
	}
}

func TestPortableSectionIndexesRejectOrphans(t *testing.T) {
	store, _ := receiptStore(t)
	tx, err := store.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`PRAGMA defer_foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO epub_sections(file_id,generation,ordinal,offset,length) VALUES('ORPHAN',1,0,0,1)`); err != nil {
		t.Fatal(err)
	}
	if err = validatePortableSectionIndexes(t.Context(), tx); err == nil {
		t.Fatal("orphan index accepted")
	}
}
