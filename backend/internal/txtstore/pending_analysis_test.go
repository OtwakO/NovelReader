package txtstore

import (
	"context"
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/txt"
)

func TestPendingAnalysisPersistsOptionsAndRevokesPreview(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, preview := analyzedReceipt(t, store)
	options := txt.Options{Encoding: txt.UTF8, Preset: txt.GeneratedSections}
	if err := store.QueueAnalysis(t.Context(), receipt.ID, preview.Version, options); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Accept(t.Context(), receipt.ID, preview.Version, "Novel", ""); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("queued work accepted a stale preview: %v", err)
	}
	if err := store.QueueAnalysis(t.Context(), receipt.ID, preview.Version, txt.Options{}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale options replaced queued work: %v", err)
	}
	reopened := NewStore(home.DB(), home.Files())
	if worked, err := reopened.AnalyzeNext(t.Context()); err != nil || !worked {
		t.Fatalf("pending analysis=%v %v", worked, err)
	}
	current, err := reopened.Preview(t.Context(), receipt.ID)
	if err != nil || current.Options != options || current.Version <= preview.Version {
		t.Fatalf("saved request lost: %+v %v", current, err)
	}
	if worked, err := reopened.AnalyzeNext(t.Context()); err != nil || worked {
		t.Fatalf("completed analysis repeated: %v %v", worked, err)
	}
}

func TestFailedPendingFileDoesNotBlockNextOriginal(t *testing.T) {
	store, _, _, root := receiptStore(t)
	broken := mustReceive(t, store)
	if err := root.Remove(broken.Path); err != nil {
		t.Fatal(err)
	}
	if worked, err := store.AnalyzeNext(t.Context()); !worked || err == nil {
		t.Fatalf("missing original result=%v %v", worked, err)
	}
	failed, err := store.Get(t.Context(), broken.ID)
	if err != nil || failed.State != AnalysisFailed || failed.Error == "" {
		t.Fatalf("failure not recorded: %+v %v", failed, err)
	}
	next := mustReceive(t, store)
	if worked, err := store.AnalyzeNext(t.Context()); !worked || err != nil {
		t.Fatalf("next file blocked: %v %v", worked, err)
	}
	if _, err := store.Preview(t.Context(), next.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledAnalysisReturnsOnlyItsClaimToPending(t *testing.T) {
	store, _, _, _ := receiptStore(t)
	receipt := mustReceive(t, store)
	// Stop after the durable claim, at the seam used by both analysis entry points.
	if err := store.QueueAnalysis(t.Context(), receipt.ID, receipt.AnalysisVersion, txt.Options{Preset: txt.GeneratedSections}); err != nil {
		t.Fatal(err)
	}
	claim, err := store.claimAnalysis(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.analyzeClaim(ctx, claim); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
	pending, err := store.Get(t.Context(), receipt.ID)
	if err != nil || pending.State != Received || pending.Options != claim.Options {
		t.Fatalf("cancelled request lost: %+v %v", pending, err)
	}
	if worked, err := store.AnalyzeNext(t.Context()); !worked || err != nil {
		t.Fatalf("resume=%v %v", worked, err)
	}
	current, err := store.Preview(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.analyzeClaim(ctx, claim); !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation=%v", err)
	}
	after, err := store.Preview(t.Context(), receipt.ID)
	if err != nil || after.Version != current.Version {
		t.Fatalf("old claim invalidated current result: %+v %v", after, err)
	}
}
