package fontstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	fontAlice readerstore.UserID = "11111111-1111-4111-8111-111111111111"
	fontBob   readerstore.UserID = "22222222-2222-4222-8222-222222222222"
)

func TestStoreKeepsEqualFontIDsInsideReaderHome(t *testing.T) {
	manager, err := readerstore.NewManager(t.TempDir(), 2, ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for _, userID := range []readerstore.UserID{fontAlice, fontBob} {
		if err := manager.Create(context.Background(), userID); err != nil {
			t.Fatal(err)
		}
	}
	aliceHome, _ := manager.Open(context.Background(), fontAlice)
	defer aliceHome.Close()
	bobHome, _ := manager.Open(context.Background(), fontBob)
	defer bobHome.Close()
	alice := NewStore(aliceHome.DB(), aliceHome.Files())
	bob := NewStore(bobHome.DB(), bobHome.Files())
	if _, err := alice.Add("Alice Font", "same-id", []byte("alice font")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := bob.Read("same-id"); err != nil || data != nil {
		t.Fatalf("bob read before add: data=%q err=%v", data, err)
	}
	if _, err := bob.Add("Bob Font", "same-id", []byte("bob font")); err != nil {
		t.Fatal(err)
	}
	_, aliceData, err := alice.Read("same-id")
	if err != nil || string(aliceData) != "alice font" {
		t.Fatalf("alice data=%q err=%v", aliceData, err)
	}
	_, bobData, err := bob.Read("same-id")
	if err != nil || string(bobData) != "bob font" {
		t.Fatalf("bob data=%q err=%v", bobData, err)
	}
	if err := alice.Delete("same-id"); err != nil {
		t.Fatal(err)
	}
	if _, err := aliceHome.Files().ReadFile(readerstore.FontsDirectory, "same-id"); err == nil {
		t.Fatal("alice font file still exists")
	}
	if data, err := bobHome.Files().ReadFile(readerstore.FontsDirectory, "same-id"); err != nil || string(data) != "bob font" {
		t.Fatalf("bob font removed: data=%q err=%v", data, err)
	}
}

func TestReplacementAndInterruptedCleanup(t *testing.T) {
	root := t.TempDir()
	manager, err := readerstore.NewManager(root, 1, ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if err := manager.Create(t.Context(), fontAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), fontAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	store := NewStore(home.DB(), home.Files())
	if _, err := store.Add("Fixture", "old", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("Fixture", "new", []byte("new")); err != nil {
		t.Fatal(err)
	}
	if _, err := home.Files().ReadFile(readerstore.FontsDirectory, "old"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file not retired: %v", err)
	}
	fonts, err := store.List()
	if err != nil || len(fonts) != 1 || fonts[0].ID != "new" {
		t.Fatalf("fonts=%v err=%v", fonts, err)
	}
	// A nonempty directory produces a deterministic removal failure, including
	// when tests run as root (unlike permission-bit based fixtures).
	path := filepath.Join(root, readerstore.UsersDirectory, string(fontAlice), readerstore.FilesDirectory, readerstore.FontsDirectory, "new")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(path, "blocker")
	if err := os.WriteFile(child, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("new"); err == nil {
		t.Fatal("cleanup failure was hidden")
	}
	var pending int
	if err := home.DB().QueryRow(`SELECT COUNT(*) FROM font_cleanup WHERE file_name='new'`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("cleanup reference lost: %d %v", pending, err)
	}
	if err := os.Remove(child); err != nil {
		t.Fatal(err)
	}
	// Simulate file removal followed by interruption before acknowledging it.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reopened := NewStore(home.DB(), home.Files())
	if err := reopened.Cleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Cleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := home.DB().QueryRow(`SELECT COUNT(*) FROM font_cleanup`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("cleanup pending=%d err=%v", pending, err)
	}
}

func TestConcurrentStoresReplaceOneName(t *testing.T) {
	manager, err := readerstore.NewManager(t.TempDir(), 1, ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if err := manager.Create(t.Context(), fontAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), fontAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	start, done := make(chan struct{}), make(chan error, 2)
	for _, id := range []string{"first", "second"} {
		go func() {
			<-start
			_, err := NewStore(home.DB(), home.Files()).Add("Shared", id, []byte(id))
			done <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	store := NewStore(home.DB(), home.Files())
	fonts, err := store.List()
	if err != nil || len(fonts) != 1 {
		t.Fatalf("fonts=%v err=%v", fonts, err)
	}
	for _, id := range []string{"first", "second"} {
		data, err := home.Files().ReadFile(readerstore.FontsDirectory, id)
		if id == fonts[0].ID {
			if err != nil || string(data) != id {
				t.Fatalf("live font lost: %q %v", data, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("obsolete font retained: %v", err)
		}
	}
}
