package epubstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/inboxfiles"
)

func inboxStore(t *testing.T) (*Store, *os.Root, *os.Root) {
	t.Helper()
	store, managed := receiptStore(t)
	tx, err := store.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = initializeInboxSchema(tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	inbox, err := store.files.OpenInbox()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inbox.Close() })
	return store, managed, inbox
}

func TestInboxAcquiresOriginalWithoutPreparing(t *testing.T) {
	store, managed, inbox := inboxStore(t)
	data := []byte("acquisition retains bytes; parsing is separate")
	if err := inbox.WriteFile("novel.EPUB", data, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := inbox.Stat("novel.EPUB")
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.AcquireInbox(t.Context(), rand.Text(), "novel.EPUB", epub.OptimizedImages)
	if err != nil || r.State != Acquired || r.PreparationGeneration != 0 || r.ImageMode != epub.OptimizedImages {
		t.Fatalf("receipt=%+v err=%v", r, err)
	}
	after, err := managed.Stat(r.Path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("rename: %v", err)
	}
	got, err := managed.ReadFile(r.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("original: %q %v", got, err)
	}
	if _, err = inbox.Stat(r.OriginalName); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if claims, err := store.PendingInbox(t.Context(), "", 10); err != nil || len(claims) != 0 {
		t.Fatalf("claims=%v %v", claims, err)
	}
}

func TestInboxIntentAndPostRenameRecovery(t *testing.T) {
	store, managed, inbox := inboxStore(t)
	if err := inbox.WriteFile("novel.epub", []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	// Failed claim insertion must roll back receipt creation and leave input alone.
	if _, err := store.db.Exec(`CREATE TRIGGER fail_claim BEFORE INSERT ON epub_inbox_claims BEGIN SELECT RAISE(ABORT,'test claim failure'); END`); err != nil {
		t.Fatal(err)
	}
	id := rand.Text()
	if _, err := store.AcquireInbox(t.Context(), id, "novel.epub", ""); err == nil {
		t.Fatal("claim failure ignored")
	}
	if _, err := store.Get(t.Context(), id); !errors.Is(err, ErrNotFound) {
		t.Fatal("orphan receipt", err)
	}
	if _, err := store.db.Exec(`DROP TRIGGER fail_claim; CREATE TRIGGER fail_acquired BEFORE UPDATE OF state ON epub_files WHEN NEW.state='acquired' BEGIN SELECT RAISE(ABORT,'test metadata failure'); END`); err != nil {
		t.Fatal(err)
	}
	r, err := store.AcquireInbox(t.Context(), id, "novel.epub", "")
	if err == nil || r.State != Finalizing {
		t.Fatalf("receipt=%+v %v", r, err)
	}
	if data, err := managed.ReadFile(r.Path); err != nil || string(data) != "original" {
		t.Fatalf("installed=%q %v", data, err)
	}
	if _, err := store.db.Exec(`DROP TRIGGER fail_acquired`); err != nil {
		t.Fatal(err)
	}
	// A new producer's file must not be touched by recovery or silent replay.
	if err := inbox.WriteFile("novel.epub", []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.SettleAcquisition(t.Context(), id)
	if err != nil || recovered.State != Acquired {
		t.Fatalf("recovered=%+v %v", recovered, err)
	}
	if existing, err := store.AcquireInbox(t.Context(), rand.Text(), "novel.epub", ""); !errors.Is(err, ErrInboxPending) || existing.ID != id {
		t.Fatalf("replay=%+v %v", existing, err)
	}
	review, err := store.ReviewInbox(t.Context(), id)
	if err != nil || review.CanRemove {
		t.Fatalf("review=%+v %v", review, err)
	}
	if err := store.ReleaseInbox(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	if data, err := inbox.ReadFile("novel.epub"); err != nil || string(data) != "replacement" {
		t.Fatalf("replacement changed: %q %v", data, err)
	}
}

func TestInboxCopyAndCancelledClaim(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "complete"
		if cancelled {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			store, managed, inbox := inboxStore(t)
			if err := inbox.WriteFile("novel.epub", []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			info, err := inbox.Stat("novel.epub")
			if err != nil {
				t.Fatal(err)
			}
			id := rand.Text()
			now := time.Now().UnixMilli()
			r := Receipt{ID: id, OriginalName: "novel.epub", Path: originalPath(id), State: Receiving, Size: info.Size(), ImageMode: epub.OriginalImages, CreatedAt: now, UpdatedAt: now}
			if err := store.recordInboxIntent(t.Context(), r); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if cancelled {
				cancel()
			}
			// Exercise the cross-device branch without requiring a second mounted disk.
			got, err := store.copyInbox(ctx, inbox, managed, r, info)
			if !cancelled {
				if err != nil || got.State != Acquired {
					t.Fatalf("copy=%+v %v", got, err)
				}
				if _, err := inbox.Stat(r.OriginalName); !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, context.Canceled) || got.State != Failed {
				t.Fatalf("cancelled=%+v %v", got, err)
			}
			if err := store.Discard(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			if _, err := store.GetInboxClaim(t.Context(), id); err != nil {
				t.Fatal("claim lost on discard", err)
			}
			review, err := store.ReviewInbox(t.Context(), id)
			if err != nil || review.CanRemove || !review.InputPresent {
				t.Fatalf("review=%+v %v", review, err)
			}
			if err := store.ReleaseInbox(t.Context(), review); err != nil {
				t.Fatal(err)
			}
			if data, err := inbox.ReadFile(r.OriginalName); err != nil || string(data) != "original" {
				t.Fatalf("source=%q %v", data, err)
			}
		})
	}
}

func TestInboxRemovalRevalidatesOwnedEvidence(t *testing.T) {
	store, managed, inbox := inboxStore(t)
	r, err := store.Receive(t.Context(), rand.Text(), "novel.epub", bytes.NewBufferString("original"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.WriteFile(r.OriginalName, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO epub_inbox_claims(name,receipt_id) VALUES(?,?)`, r.OriginalName, r.ID); err != nil {
		t.Fatal(err)
	}
	review, err := store.ReviewInbox(t.Context(), r.ID)
	if err != nil || !review.CanRemove {
		t.Fatalf("review=%+v %v", review, err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), &InboxReview{Claim: review.Claim, CanRemove: true}); !errors.Is(err, ErrStateChanged) {
		t.Fatal("forged proof", err)
	}
	other, _, _ := inboxStore(t)
	if err := other.ConfirmInboxRemoval(t.Context(), review); !errors.Is(err, ErrStateChanged) {
		t.Fatal("cross-home proof", err)
	}
	info, err := inbox.Stat(r.OriginalName)
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.WriteFile(r.OriginalName, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := inbox.Chtimes(r.OriginalName, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), review); !errors.Is(err, inboxfiles.ErrChanged) {
		t.Fatal("stale proof", err)
	}
	if err := inbox.WriteFile(r.OriginalName, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	review, err = store.ReviewInbox(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	if data, err := managed.ReadFile(r.Path); err != nil || string(data) != "original" {
		t.Fatalf("managed=%q %v", data, err)
	}
}

func TestPortableInboxDropsLocalAuthorityOnly(t *testing.T) {
	store, _, _ := inboxStore(t)
	id := rand.Text()
	if _, err := store.db.Exec(`INSERT INTO epub_inbox_claims(name,receipt_id) VALUES(?,?)`, "novel.epub", id); err != nil {
		t.Fatal(err)
	}
	tx, err := store.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := validatePortableInbox(t.Context(), tx); err == nil {
		t.Fatal("portable authority accepted")
	}
	if err := preparePortableInbox(t.Context(), tx); err != nil {
		t.Fatal(err)
	}
	if err := validatePortableInbox(t.Context(), tx); err != nil {
		t.Fatal(err)
	}
	// Roll back this simulated private-copy cleanup; the live claim must persist.
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetInboxClaim(t.Context(), id); err != nil {
		t.Fatal(err)
	}
}

func TestInboxScanUsesEPUBPolicyAndClaims(t *testing.T) {
	store, _, inbox := inboxStore(t)
	for _, name := range []string{"a.epub", "b.txt", "c.EPUB"} {
		if err := inbox.WriteFile(name, []byte("input"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	id := rand.Text()
	if _, err := store.db.Exec(`INSERT INTO epub_inbox_claims(name,receipt_id) VALUES(?,?)`, "a.epub", id); err != nil {
		t.Fatal(err)
	}
	file, err := inbox.OpenFile("c.EPUB", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = errors.Join(file.Truncate(epub.MaxInputBytes+1), file.Close())
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.ScanInbox(t.Context(), "", 1)
	if err != nil || len(page) != 1 || page[0].Name != "a.epub" || page[0].ReceiptID != id {
		t.Fatalf("page=%+v %v", page, err)
	}
	page, err = store.ScanInbox(t.Context(), "a.epub", 10)
	if err != nil || len(page) != 1 || page[0].Name != "c.EPUB" || page[0].Problem != "too_large" {
		t.Fatalf("page=%+v %v", page, err)
	}
}
