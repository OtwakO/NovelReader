package txtstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/otwako/novelreader/internal/readerstore"
)

const alice readerstore.UserID = "11111111-1111-4111-8111-111111111111"
const bob readerstore.UserID = "22222222-2222-4222-8222-222222222222"

func receiptStore(t *testing.T) (*Store, *readerstore.Manager, *readerstore.Home, *os.Root) {
	t.Helper()
	manager, err := readerstore.NewManager(t.TempDir(), 2, ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	if err := manager.Create(t.Context(), alice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), alice)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { home.Close() })
	root, err := home.Files().OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return NewStore(home.DB(), home.Files()), manager, home, root
}

func mustReceive(t *testing.T, store *Store) Receipt {
	t.Helper()
	value, err := store.Receive(t.Context(), "小說.txt", strings.NewReader("第一章\nOriginal bytes.\n"))
	if err != nil || value.State != Received {
		t.Fatalf("receipt=%+v err=%v", value, err)
	}
	return value
}

func TestReceiptOriginalIsPortableAndDiscardIsIsolated(t *testing.T) {
	store, manager, _, root := receiptStore(t)
	value := mustReceive(t, store)
	original, err := root.ReadFile(value.Path)
	if err != nil || string(original) != "第一章\nOriginal bytes.\n" {
		t.Fatalf("original=%q err=%v", original, err)
	}
	if _, err := root.Stat(workPath(value.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("work remains: %v", err)
	}
	// Temporary bytes are not part of a portable snapshot, regardless of owner.
	if err := root.WriteFile(path.Join(readerstore.WorkDirectory, "txt", "partial"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(snapshot, readerstore.FilesDirectory, readerstore.WorkDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot included temporary work: %v", err)
	}
	if err := manager.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	stage, err := manager.PrepareReplacement(t.Context(), bob, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.PublishReplacement(t.Context(), bob, stage); err != nil {
		t.Fatal(err)
	}
	restored, err := manager.Open(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restoredStore := NewStore(restored.DB(), restored.Files())
	copied, err := restoredStore.Get(t.Context(), value.ID)
	if err != nil || copied.Path != value.Path || copied.Size != int64(len(original)) {
		t.Fatalf("restored=%+v err=%v", copied, err)
	}
	restoredRoot, err := restored.Files().OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer restoredRoot.Close()
	content, err := restoredRoot.ReadFile(copied.Path)
	if err != nil || !bytes.Equal(content, original) {
		t.Fatalf("restored bytes=%q err=%v", content, err)
	}
	for range 2 {
		if err := restoredStore.Discard(t.Context(), value.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := restoredStore.Get(t.Context(), value.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted receipt=%v", err)
	}
	if _, err := restoredRoot.Stat(path.Dir(value.Path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned directory remains: %v", err)
	}
	if _, err := root.Stat(value.Path); err != nil {
		t.Fatalf("other reader changed: %v", err)
	}
}

func TestInterruptedReceiptRecoveryRetainsCompleteOriginals(t *testing.T) {
	store, _, home, root := receiptStore(t)
	complete := mustReceive(t, store)
	removing := mustReceive(t, store)
	partial, err := store.Receive(t.Context(), "partial.txt", iotest.ErrReader(io.ErrUnexpectedEOF))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	for _, value := range []Receipt{complete, partial} {
		if _, err := home.DB().Exec(`UPDATE txt_files SET state=? WHERE id=?`, Receiving, value.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := root.WriteFile(workPath(partial.ID), []byte("incomplete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`UPDATE txt_files SET state=? WHERE id=?`, Removing, removing.ID); err != nil {
		t.Fatal(err)
	}
	if err := root.Remove(removing.Path); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("txt/unreferenced.txt", []byte("retain"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	acquired, err := store.Get(t.Context(), complete.ID)
	if err != nil || acquired.State != Received {
		t.Fatalf("complete=%+v err=%v", acquired, err)
	}
	failed, err := store.Get(t.Context(), partial.ID)
	if err != nil || failed.State != Failed || failed.Error == "" {
		t.Fatalf("partial=%+v err=%v", failed, err)
	}
	if _, err := root.Stat(workPath(partial.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial work remains: %v", err)
	}
	if _, err := store.Get(t.Context(), removing.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removal not recovered: %v", err)
	}
	if _, err := root.Stat("txt/unreferenced.txt"); err != nil {
		t.Fatalf("unreferenced original deleted: %v", err)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(data []byte) (int, error) { return f(data) }

func TestCancelledTransferRemainsAnExplicitFailedReceipt(t *testing.T) {
	store, _, _, root := receiptStore(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	value, err := store.Receive(ctx, "cancelled.txt", readerFunc(func(data []byte) (int, error) { cancel(); return copy(data, "partial"), nil }))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	stored, err := store.Get(t.Context(), value.ID)
	if err != nil || stored.State != Failed {
		t.Fatalf("cancelled=%+v err=%v", stored, err)
	}
	if _, err := root.Stat(workPath(value.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled work remains: %v", err)
	}
	if err := store.Discard(t.Context(), value.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDiscardRejectsForeignPersistedPaths(t *testing.T) {
	store, _, home, root := receiptStore(t)
	value := mustReceive(t, store)
	if err := root.WriteFile("fonts/protected", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`UPDATE txt_files SET path=? WHERE id=?`, "fonts/protected", value.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Discard(t.Context(), value.ID); !errors.Is(err, readerstore.ErrInvalidFilePath) {
		t.Fatalf("foreign path error=%v", err)
	}
	if content, err := root.ReadFile("fonts/protected"); err != nil || string(content) != "keep" {
		t.Fatalf("foreign file changed: %q %v", content, err)
	}
}

func TestRemovalFailureRemainsRetryable(t *testing.T) {
	store, _, _, root := receiptStore(t)
	value := mustReceive(t, store)
	extra := path.Join(path.Dir(value.Path), "unexpected")
	if err := root.WriteFile(extra, []byte("retain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Discard(t.Context(), value.ID); err == nil {
		t.Fatal("reported complete cleanup of a nonempty directory")
	}
	pending, err := store.Get(t.Context(), value.ID)
	if err != nil || pending.State != Removing || pending.Error == "" {
		t.Fatalf("pending removal=%+v err=%v", pending, err)
	}
	if err := root.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), value.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removal retry failed: %v", err)
	}
}
