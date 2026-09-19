package epubstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func checkPortableStreams(t *testing.T, s *Store, root *os.Root) error {
	t.Helper()
	tx, err := s.db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = validatePortableSectionIndexes(t.Context(), tx); err != nil {
		return err
	}
	return validatePortableStreams(t.Context(), tx, root)
}

func TestPortableStreamIntegrity(t *testing.T) {
	for _, damage := range []string{"missing", "truncated", "invalid-json", "wrong-ordinal"} {
		t.Run(damage, func(t *testing.T) {
			store, root := receiptStore(t)
			r := acquiredFixture(t, store, epub.OriginalImages)
			a, err := store.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Prepare(t.Context(), r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableStreams(t, store, root); err != nil {
				t.Fatal("valid stream rejected", err)
			}
			name := path.Join(preparationPath(r.ID, a.Generation), sectionStreamFile)
			data, err := root.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "missing":
				err = root.Remove(name)
			case "truncated":
				err = root.WriteFile(name, data[:len(data)-1], 0600)
			case "invalid-json":
				data[0] = '!'
				err = root.WriteFile(name, data, 0600)
			case "wrong-ordinal":
				changed := bytes.Replace(data, []byte(`"Ordinal":0`), []byte(`"Ordinal":9`), 1)
				if bytes.Equal(changed, data) {
					t.Fatal("fixture lacks ordinal")
				}
				err = root.WriteFile(name, changed, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = checkPortableStreams(t, store, root); err == nil {
				t.Fatal("damaged ready stream accepted")
			}
			// The same damage during interrupted installation must remain retryable.
			if _, err = store.db.Exec(`UPDATE epub_preparations SET state='finalizing' WHERE file_id=?`, r.ID); err != nil {
				t.Fatal(err)
			}
			if err = checkPortableStreams(t, store, root); err != nil {
				t.Fatal("interrupted finalization rejected", err)
			}
			got, err := store.GetPreparation(t.Context(), r.ID, a.Generation)
			if err != nil || got.State != PreparationFinalizing {
				t.Fatal("validation mutated state", got, err)
			}
			if err = store.RecoverPreparations(t.Context()); err != nil {
				t.Fatal(err)
			}
			got, err = store.GetPreparation(t.Context(), r.ID, a.Generation)
			if err != nil || got.State != PreparationFailed {
				t.Fatal("damaged finalization became ready", got, err)
			}
			if err = checkPortableStreams(t, store, root); err != nil {
				t.Fatal("discarded failed output rejected", err)
			}
		})
	}
}

func TestIncompletePreparationPreservesOtherFailures(t *testing.T) {
	if !isIncompletePreparation(fmt.Errorf("read: %w", errors.Join(io.ErrUnexpectedEOF, os.ErrNotExist))) {
		t.Fatal("known incomplete causes rejected")
	}
	for _, err := range []error{nil, context.Canceled, errors.Join(io.ErrUnexpectedEOF, os.ErrPermission), fmt.Errorf("close: %w", errors.Join(errIncompletePreparation, context.DeadlineExceeded))} {
		if isIncompletePreparation(err) {
			t.Fatalf("failure incorrectly permits incomplete-output cleanup: %v", err)
		}
	}
}

func TestPortableSemanticDamageIsNotInterruptedOutput(t *testing.T) {
	for _, damage := range []struct{ name, old, replacement string }{
		{"node kind", `"kind":"group"`, `"kind":"bogus"`},
		{"forward anchor", `"anchor":"a1"`, `"anchor":"a9"`},
	} {
		t.Run(damage.name, func(t *testing.T) {
			store, root := receiptStore(t)
			receipt := acquiredFixture(t, store, epub.OriginalImages)
			attempt, err := store.QueuePreparation(t.Context(), receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Prepare(t.Context(), receipt.ID, attempt.Generation); err != nil {
				t.Fatal(err)
			}
			name := path.Join(preparationPath(receipt.ID, attempt.Generation), sectionStreamFile)
			data, err := root.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			changed := bytes.Replace(data, []byte(damage.old), []byte(damage.replacement), 1)
			if bytes.Equal(data, changed) {
				t.Fatal("fixture lacks target for corruption")
			}
			if err = root.WriteFile(name, changed, 0600); err != nil {
				t.Fatal(err)
			}
			for _, state := range []PreparationState{PreparationReady, PreparationFinalizing} {
				if _, err = store.db.Exec(`UPDATE epub_preparations SET state=? WHERE file_id=?`, state, receipt.ID); err != nil {
					t.Fatal(err)
				}
				if err = checkPortableStreams(t, store, root); !errors.Is(err, epub.ErrPreparedSection) {
					t.Fatalf("%s admitted malformed semantics: %v", state, err)
				}
			}
		})
	}
}

func TestPortableSectionRegistryAgreement(t *testing.T) {
	store, root := receiptStore(t)
	receipt := acquiredFixture(t, store, epub.OriginalImages)
	attempt, err := store.QueuePreparation(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Prepare(t.Context(), receipt.ID, attempt.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`UPDATE epub_resources SET width=width+1 WHERE file_id=?`, receipt.ID); err != nil {
		t.Fatal(err)
	}
	for _, state := range []PreparationState{PreparationReady, PreparationFinalizing} {
		if _, err = store.db.Exec(`UPDATE epub_preparations SET state=? WHERE file_id=?`, state, receipt.ID); err != nil {
			t.Fatal(err)
		}
		if err = checkPortableStreams(t, store, root); !errors.Is(err, epub.ErrPreparedSection) {
			t.Fatalf("%s accepted mismatched resource: %v", state, err)
		}
	}
}
