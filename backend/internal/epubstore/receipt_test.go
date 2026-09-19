package epubstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path"
	"testing"
	"testing/iotest"

	"database/sql"
	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
)

func receiptStore(t *testing.T) (*Store, *os.Root) {
	t.Helper()
	// Fixture composition keeps each schema owner explicit.
	manager, err := readerstore.NewManager(t.TempDir(), 1, readerstore.ReaderSchema{Initialize: initializeFixtureSchema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	id := readerstore.UserID("11111111-1111-4111-8111-111111111111")
	if err = manager.Create(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { home.Close() })
	root, err := home.Files().OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return NewStore(home.DB(), home.Files()), root
}

func TestReceiptRetainsOriginalAndPolicy(t *testing.T) {
	store, root := receiptStore(t)
	data := stagedFixture(t, false)
	id := rand.Text()
	r, err := store.Receive(t.Context(), id, "Novel.EPUB", bytes.NewReader(data), epub.OptimizedImages)
	if err != nil || r.State != Acquired || r.Size != int64(len(data)) || r.ImageMode != epub.OptimizedImages {
		t.Fatalf("receipt: %+v %v", r, err)
	}
	stored, err := store.Get(t.Context(), id)
	if err != nil || stored != r {
		t.Fatalf("persisted receipt: %+v %v", stored, err)
	}
	// An accidental retry cannot replace the bytes or the snapshotted policy.
	if _, err = store.Receive(t.Context(), id, "Other.epub", bytes.NewReader([]byte("replacement")), epub.OriginalImages); err == nil {
		t.Fatal("duplicate ID accepted")
	}
	got, err := root.ReadFile(r.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("original changed", err)
	}
	if _, err = root.Stat(transferPath(id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("work retained", err)
	}
	if err = store.Discard(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if err = store.Discard(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Get(t.Context(), id); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = root.Stat(r.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestReceiptFailureAndInputBoundaries(t *testing.T) {
	store, root := receiptStore(t)
	cause := errors.New("synthetic stream interruption")
	r, err := store.Receive(t.Context(), rand.Text(), "novel.epub", iotest.ErrReader(cause), "")
	if !errors.Is(err, cause) || r.State != Failed || r.ImageMode != epub.OriginalImages {
		t.Fatalf("failed receipt: %+v %v", r, err)
	}
	stored, err := store.Get(t.Context(), r.ID)
	if err != nil || stored != r || stored.State != Failed || stored.Error == "" {
		t.Fatalf("failure not retained: %+v %v", stored, err)
	}
	if _, err = root.Stat(transferPath(r.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, name string
		mode     epub.ImageMode
		want     error
	}{
		{"../escape", "novel.epub", "", readerstore.ErrInvalidFilePath},
		{rand.Text(), "../novel.epub", "", ErrInvalidFilename},
		{rand.Text(), "novel.epub", "unknown", epub.ErrImagePolicy},
	} {
		if _, err = store.Receive(t.Context(), tc.id, tc.name, bytes.NewReader(nil), tc.mode); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = store.Receive(ctx, rand.Text(), "novel.epub", bytes.NewReader(nil), ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAcquisitionRecoveryAtMoveBoundaries(t *testing.T) {
	for _, boundary := range []string{"before-move", "after-move", "incomplete", "wrong-size", "removing"} {
		t.Run(boundary, func(t *testing.T) {
			store, root := receiptStore(t)
			data := stagedFixture(t, false)
			r, err := store.Receive(t.Context(), rand.Text(), "novel.epub", bytes.NewReader(data), "")
			if err != nil {
				t.Fatal(err)
			}
			state := Finalizing
			switch boundary {
			case "before-move", "incomplete":
				if err = root.MkdirAll(path.Dir(transferPath(r.ID)), 0700); err != nil {
					t.Fatal(err)
				}
				if err = root.Rename(r.Path, transferPath(r.ID)); err != nil {
					t.Fatal(err)
				}
				if boundary == "incomplete" {
					state = Receiving
				}
			case "wrong-size":
				if err = root.WriteFile(r.Path, []byte("truncated"), 0600); err != nil {
					t.Fatal(err)
				}
			case "removing":
				state = Removing
			}
			if _, err = store.db.ExecContext(t.Context(), `UPDATE epub_files SET state=? WHERE id=?`, state, r.ID); err != nil {
				t.Fatal(err)
			}
			if err = store.RecoverAcquisitions(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err = store.RecoverAcquisitions(t.Context()); err != nil {
				t.Fatal("repeated recovery", err)
			}
			got, err := store.Get(t.Context(), r.ID)
			if boundary == "removing" {
				if !errors.Is(err, ErrNotFound) {
					t.Fatal(err)
				}
				if _, err = root.Stat(r.Path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				return
			}
			want := Acquired
			if boundary == "incomplete" || boundary == "wrong-size" {
				want = Failed
			}
			if err != nil || got.State != want {
				t.Fatalf("state: %+v %v", got, err)
			}
			if want == Acquired {
				got, err := root.ReadFile(r.Path)
				if err != nil || !bytes.Equal(got, data) {
					t.Fatal("recovered bytes", err)
				}
			}
			if _, err = root.Stat(transferPath(r.ID)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("temporary bytes remain", err)
			}
		})
	}
}

func TestAcquisitionRecoveryRejectsForeignPaths(t *testing.T) {
	store, root := receiptStore(t)
	if err := root.WriteFile("sentinel", []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := store.db.ExecContext(t.Context(), `INSERT INTO epub_files(id,original_name,state,image_mode,created_at,updated_at) VALUES('../sentinel','novel.epub','removing','original',0,0)`)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RecoverAcquisitions(t.Context()); !errors.Is(err, readerstore.ErrInvalidFilePath) {
		t.Fatal(err)
	}
	if data, err := root.ReadFile("sentinel"); err != nil || string(data) != "keep" {
		t.Fatal("foreign file changed", err)
	}
}

// Exercise a real metadata failure after Rename, not just a fabricated receipt.
func TestReceiveRetainsIntentWhenCommitFails(t *testing.T) {
	store, root := receiptStore(t)
	_, err := store.db.ExecContext(t.Context(), `CREATE TRIGGER fail_acquired BEFORE UPDATE ON epub_files WHEN NEW.state='acquired' BEGIN SELECT RAISE(FAIL,'synthetic metadata failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	data := stagedFixture(t, false)
	r, err := store.Receive(t.Context(), rand.Text(), "novel.epub", bytes.NewReader(data), epub.OriginalImages)
	if err == nil || r.State != Finalizing {
		t.Fatalf("lost finalization intent: %+v %v", r, err)
	}
	stored, err := store.Get(t.Context(), r.ID)
	if err != nil || stored.State != Finalizing {
		t.Fatal("intent not durable", err)
	}
	if got, err := root.ReadFile(r.Path); err != nil || !bytes.Equal(got, data) {
		t.Fatal("original not retained", err)
	}
	if _, err = store.db.ExecContext(t.Context(), `DROP TRIGGER fail_acquired`); err != nil {
		t.Fatal(err)
	}
	if err = store.RecoverAcquisitions(t.Context()); err != nil {
		t.Fatal(err)
	}
	stored, err = store.Get(t.Context(), r.ID)
	if err != nil || stored.State != Acquired {
		t.Fatal("not recovered", err)
	}
}

func initializeFixtureSchema(tx *sql.Tx) error {
	if err := library.ReaderSchema().Initialize(tx); err != nil {
		return err
	}
	return initializeSchema(tx)
}
