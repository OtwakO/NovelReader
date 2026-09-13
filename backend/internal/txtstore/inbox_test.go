package txtstore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestInboxRenameConsumesOnlyTheOwnedEntry(t *testing.T) {
	store, manager, home, managed := receiptStore(t)
	inbox, err := home.Files().OpenInbox()
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Close()
	if err := inbox.WriteFile("novel.txt", []byte(novel), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := inbox.Stat("novel.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	other, err := manager.Open(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	otherInbox, err := other.Files().OpenInbox()
	if err != nil {
		t.Fatal(err)
	}
	defer otherInbox.Close()
	if err := otherInbox.WriteFile("novel.txt", []byte("other reader"), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := store.AcquireInbox(t.Context(), rand.Text(), "novel.txt")
	if err != nil || value.State != Received {
		t.Fatalf("claim=%+v err=%v", value, err)
	}
	after, err := managed.Stat(value.Path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("same-filesystem claim copied instead of moving: %v", err)
	}
	if _, err := inbox.Stat("novel.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original remains: %v", err)
	}
	if claims, err := store.PendingInbox(t.Context(), "", 100); err != nil || len(claims) != 0 {
		t.Fatalf("finished claim retained: %+v %v", claims, err)
	}
	if content, err := otherInbox.ReadFile("novel.txt"); err != nil || string(content) != "other reader" {
		t.Fatalf("other reader changed: %q %v", content, err)
	}
	if err := home.Files().MoveInboxTo("../novel.txt", value.Path); !errors.Is(err, readerstore.ErrInvalidFilePath) {
		t.Fatalf("unsafe ingress name accepted: %v", err)
	}
}

func TestInboxCopyFallbackAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "complete"
		if cancelled {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			store, _, home, managed := receiptStore(t)
			inbox, err := home.Files().OpenInbox()
			if err != nil {
				t.Fatal(err)
			}
			defer inbox.Close()
			if err := inbox.WriteFile("novel.txt", []byte(novel), 0o600); err != nil {
				t.Fatal(err)
			}
			info, err := inbox.Stat("novel.txt")
			if err != nil {
				t.Fatal(err)
			}
			value := Receipt{ID: rand.Text(), OriginalName: "novel.txt", State: Receiving, Size: info.Size(), CreatedAt: time.Now().UnixMilli()}
			value.Path, err = managedPath(value.OriginalName, value.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.recordInboxIntent(t.Context(), value); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if cancelled {
				cancel()
			}
			// Exercise the actual fallback after cross-device rename classification,
			// without requiring a developer mount or privileged CI setup.
			received, err := store.copyInbox(ctx, inbox, managed, value, info)
			if cancelled {
				if !errors.Is(err, context.Canceled) || received.State != Failed {
					t.Fatalf("cancelled=%+v %v", received, err)
				}
				if err := store.Discard(t.Context(), value.ID); err != nil {
					t.Fatal(err)
				}
				if claims, err := store.PendingInbox(t.Context(), "", 100); err != nil || len(claims) != 1 {
					t.Fatalf("discard forgot unresolved input: %+v %v", claims, err)
				}
				if content, err := inbox.ReadFile(value.OriginalName); err != nil || string(content) != novel {
					t.Fatalf("cancel lost original: %q %v", content, err)
				}
			} else {
				if err != nil || received.State != Received {
					t.Fatalf("copy=%+v %v", received, err)
				}
				if content, err := managed.ReadFile(value.Path); err != nil || string(content) != novel {
					t.Fatalf("copy changed bytes: %q %v", content, err)
				}
				if _, err := inbox.Stat(value.OriginalName); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("copy retained input: %v", err)
				}
			}
		})
	}
}

func TestInterruptedInboxClaimIsNotReplayedOrExported(t *testing.T) {
	store, manager, home, _ := receiptStore(t)
	inbox, err := home.Files().OpenInbox()
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Close()
	if err := inbox.WriteFile("novel.txt", []byte(novel), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := store.AcquireInbox(t.Context(), rand.Text(), "novel.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Crash after moving bytes but before publishing the receipt/clearing intent.
	if _, err := home.DB().Exec(`UPDATE txt_files SET state=? WHERE id=?`, Receiving, value.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, value.OriginalName, value.ID); err != nil {
		t.Fatal(err)
	}
	if err := inbox.WriteFile(value.OriginalName, []byte("new occupant"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	existing, err := store.AcquireInbox(t.Context(), rand.Text(), value.OriginalName)
	if !errors.Is(err, ErrInboxPending) || existing.ID != value.ID {
		t.Fatalf("unresolved input re-imported: %+v %v", existing, err)
	}
	if content, err := inbox.ReadFile(value.OriginalName); err != nil || string(content) != "new occupant" {
		t.Fatalf("recovery consumed uncertain input: %q %v", content, err)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(snapshot, readerstore.ReaderDatabaseName)
	copied, err := sql.Open("sqlite", database)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := copied.QueryRow(`SELECT count(*) FROM txt_inbox_claims`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("exported authority=%d %v", count, err)
	}
	// Restore sanitizes input too, rather than trusting the exporter.
	if _, err := copied.Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, value.OriginalName, value.ID); err != nil {
		t.Fatal(err)
	}
	if err := copied.Close(); err != nil {
		t.Fatal(err)
	}
	if claims, err := store.PendingInbox(t.Context(), "", 100); err != nil || len(claims) != 1 {
		t.Fatalf("export changed live intent: %+v %v", claims, err)
	}
	if err := manager.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	stage, err := manager.PrepareReplacement(t.Context(), bob, database, filepath.Join(snapshot, readerstore.FilesDirectory))
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
	if err := restoredStore.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if claims, err := restoredStore.PendingInbox(t.Context(), "", 100); err != nil || len(claims) != 0 {
		t.Fatalf("restored deletion authority: %+v %v", claims, err)
	}
}
