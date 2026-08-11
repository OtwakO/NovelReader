package fontstore

import (
	"context"
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
