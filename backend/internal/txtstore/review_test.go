package txtstore

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/txt"
)

func TestReceiptPagesAndVersionedReview(t *testing.T) {
	store, _, _, _ := receiptStore(t)
	first, analysis := analyzedReceipt(t, store)
	second := mustReceive(t, store)
	page, err := store.List(t.Context(), "", "", 1)
	if err != nil || len(page) != 1 {
		t.Fatalf("page: %+v %v", page, err)
	}
	next, err := store.List(t.Context(), page[0].ID, "", 1)
	if err != nil || len(next) != 1 || next[0].ID == page[0].ID {
		t.Fatalf("next page: %+v %v", next, err)
	}
	filtered, err := store.List(t.Context(), "", Received, 10)
	if err != nil || len(filtered) != 1 || filtered[0].ID != second.ID {
		t.Fatalf("filter: %+v %v", filtered, err)
	}
	review, err := store.Review(t.Context(), first.ID, analysis.Version, 1, 1)
	if err != nil || review.TotalSections != 3 || len(review.Headings) != 1 || review.Headings[0].Index != 1 || !strings.Contains(review.Sample, "Second paragraph") || !review.HasMore {
		t.Fatalf("review: %+v %v", review, err)
	}
	if err := store.QueueAnalysis(t.Context(), first.ID, analysis.Version, txt.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Review(t.Context(), first.ID, analysis.Version, 1, 1); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale review: %v", err)
	}
	long, err := store.Receive(t.Context(), rand.Text(), "long.txt", strings.NewReader(strings.Repeat("文字", 5000)))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := store.Analyze(t.Context(), long.ID, txt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	review, err = store.Review(t.Context(), long.ID, parsed.Version, 0, 10)
	if err != nil || len(review.Sample) > reviewSampleBytes || !review.SampleTruncated {
		t.Fatalf("unbounded sample: %d %v", len(review.Sample), err)
	}
}

func TestPendingDiscardCannotRemovePublishedReceipt(t *testing.T) {
	store, _, _, root := receiptStore(t)
	receipt, analysis := analyzedReceipt(t, store)
	if _, err := store.Accept(t.Context(), receipt.ID, analysis.Version, "Publication", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DiscardPending(t.Context(), receipt.ID); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("published receipt discarded: %v", err)
	}
	if _, err := root.Stat(receipt.Path); err != nil {
		t.Fatal(err)
	}
	pending := mustReceive(t, store)
	if cleanup, err := store.DiscardPending(t.Context(), pending.ID); err != nil || cleanup {
		t.Fatalf("discard: %v %v", cleanup, err)
	}
	if _, err := store.Get(t.Context(), pending.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("receipt retained: %v", err)
	}
}
