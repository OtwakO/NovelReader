package txtstore

import (
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
)

func reviewedLeftover(t *testing.T, store *Store, home *readerstore.Home) (Receipt, *os.Root) {
	t.Helper()
	receipt, preview := analyzedReceipt(t, store)
	if _, err := store.Accept(t.Context(), receipt.ID, preview.Version, "Novel", ""); err != nil {
		t.Fatal(err)
	}
	inbox, err := home.Files().OpenInbox()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inbox.Close() })
	if err := inbox.WriteFile(receipt.OriginalName, []byte(novel), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, receipt.OriginalName, receipt.ID); err != nil {
		t.Fatal(err)
	}
	return receipt, inbox
}

func TestInboxConfirmationIsScopedAndKeepsPublication(t *testing.T) {
	store, manager, home, managed := receiptStore(t)
	receipt, inbox := reviewedLeftover(t, store, home)
	review, err := store.ReviewInbox(t.Context(), receipt.ID)
	if err != nil || !review.CanRemove {
		t.Fatalf("review=%+v %v", review, err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), &InboxReview{Claim: review.Claim, CanRemove: true}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("display fields authorized removal: %v", err)
	}
	if err := manager.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	other, err := manager.Open(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := NewStore(other.DB(), other.Files()).ConfirmInboxRemoval(t.Context(), review); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("cross-reader proof accepted: %v", err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.Stat(receipt.OriginalName); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("duplicate remains: %v", err)
	}
	if content, err := managed.ReadFile(receipt.Path); err != nil || string(content) != novel {
		t.Fatalf("managed original changed: %q %v", content, err)
	}
	if item, err := library.NewStore(home.DB()).Get(t.Context(), receipt.ID); err != nil || item == nil {
		t.Fatalf("publication removed: %+v %v", item, err)
	}
	if err := inbox.WriteFile(receipt.OriginalName, []byte("new input"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	if content, err := inbox.ReadFile(receipt.OriginalName); err != nil || string(content) != "new input" {
		t.Fatalf("retry deleted a new entry: %q %v", content, err)
	}
}

func TestInboxReviewRejectsContentChangeEvenWithPreservedMetadata(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, inbox := reviewedLeftover(t, store, home)
	review, err := store.ReviewInbox(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := inbox.Stat(receipt.OriginalName)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(novel, "First", "Other", 1)
	if err := inbox.WriteFile(receipt.OriginalName, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := inbox.Chtimes(receipt.OriginalName, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), review); !errors.Is(err, ErrInboxChanged) {
		t.Fatalf("changed content approved: %v", err)
	}
	if err := store.ReleaseInbox(t.Context(), review); !errors.Is(err, ErrInboxChanged) {
		t.Fatalf("stale release approved: %v", err)
	}
	current, err := store.ReviewInbox(t.Context(), receipt.ID)
	if err != nil || current.CanRemove {
		t.Fatalf("different input marked duplicate: %+v %v", current, err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), current); !errors.Is(err, ErrInboxNotDuplicate) {
		t.Fatalf("unique input deletion allowed: %v", err)
	}
	if err := store.ReleaseInbox(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	if content, err := inbox.ReadFile(receipt.OriginalName); err != nil || string(content) != changed {
		t.Fatalf("release changed bytes: %q %v", content, err)
	}
	retried, err := store.AcquireInbox(t.Context(), rand.Text(), receipt.OriginalName)
	if err != nil || retried.ID == receipt.ID {
		t.Fatalf("released input not independently acquirable: %+v %v", retried, err)
	}
}

func TestInboxConfirmationCannotDeleteTheOnlySurvivingCopy(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, inbox := reviewedLeftover(t, store, home)
	review, err := store.ReviewInbox(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Discard(t.Context(), receipt.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmInboxRemoval(t.Context(), review); !errors.Is(err, ErrInboxNotDuplicate) {
		t.Fatalf("discarded managed copy still authorized deletion: %v", err)
	}
	if content, err := inbox.ReadFile(receipt.OriginalName); err != nil || string(content) != novel {
		t.Fatalf("sole original lost: %q %v", content, err)
	}
	if err := store.ReleaseInbox(t.Context(), review); err != nil {
		t.Fatal(err)
	}
	if claims, err := store.PendingInbox(t.Context(), "", 100); err != nil || len(claims) != 0 {
		t.Fatalf("release retained claim: %+v %v", claims, err)
	}
}

func TestAbsentInboxEntryCanBeReleasedButNotReplacedUnderOldReview(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, inbox := reviewedLeftover(t, store, home)
	if err := inbox.Remove(receipt.OriginalName); err != nil {
		t.Fatal(err)
	}
	review, err := store.ReviewInbox(t.Context(), receipt.ID)
	if err != nil || review.InputPresent {
		t.Fatalf("absent review=%+v %v", review, err)
	}
	if err := inbox.WriteFile(receipt.OriginalName, []byte(novel), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseInbox(t.Context(), review); !errors.Is(err, ErrInboxChanged) {
		t.Fatalf("appearance did not invalidate review: %v", err)
	}
	if err := inbox.Remove(receipt.OriginalName); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseInbox(t.Context(), review); err != nil {
		t.Fatal(err)
	}
}
