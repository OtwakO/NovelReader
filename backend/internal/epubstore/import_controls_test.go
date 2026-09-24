package epubstore

import (
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/epub"
)

func TestImportControlsRespectActiveGeneration(t *testing.T) {
	store, root := receiptStore(t)
	receipt := acquiredFixture(t, store, epub.OriginalImages)
	attempt, err := store.RetryPreparation(t.Context(), receipt.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.RetryPreparation(t.Context(), receipt.ID, attempt.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("queued replacement: %v", err)
	}
	if _, err = store.ClaimPreparation(t.Context(), receipt.ID, attempt.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = store.RetryPreparation(t.Context(), receipt.ID, attempt.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("running replacement: %v", err)
	}
	if pending, err := store.DiscardPending(t.Context(), receipt.ID); pending || !errors.Is(err, ErrStateChanged) {
		t.Fatalf("running discard: %v %v", pending, err)
	}
	if err = store.FailPreparation(t.Context(), receipt.ID, attempt.Generation, errors.New("private/native/path")); err != nil {
		t.Fatal(err)
	}
	current, err := store.GetImport(t.Context(), receipt.ID)
	if err != nil || current.PreparationState != PreparationFailed || PreparationErrorCode(current.PreparationError) != "epub_preparation_failed" {
		t.Fatalf("status: %+v %v", current, err)
	}
	next, err := store.RetryPreparation(t.Context(), receipt.ID, attempt.Generation)
	if err != nil || next.Generation <= attempt.Generation {
		t.Fatalf("retry: %+v %v", next, err)
	}
	if _, err = store.RetryPreparation(t.Context(), receipt.ID, attempt.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale retry: %v", err)
	}
	if pending, err := store.DiscardPending(t.Context(), receipt.ID); pending || err != nil {
		t.Fatalf("queued discard: %v %v", pending, err)
	}
	if _, err = root.Stat(receipt.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original retained: %v", err)
	}
	if _, err = store.ClaimPreparation(t.Context(), receipt.ID, next.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("deleted claim: %v", err)
	}
}

func TestReviewSampleBoundsUnicode(t *testing.T) {
	root := epub.Node{Kind: "paragraph", Children: []epub.Node{{Kind: "text", Text: "Introduction "}, {Kind: "strong", Children: []epub.Node{{Kind: "text", Text: strings.Repeat("阅读", 3000)}}}}}
	sample, truncated, err := reviewSample(t.Context(), root)
	if err != nil || !truncated || len(sample) > reviewSampleBytes || !utf8.ValidString(sample) || !strings.HasPrefix(sample, "Introduction 阅读") {
		t.Fatalf("sample length=%d truncated=%v err=%v", len(sample), truncated, err)
	}
}
