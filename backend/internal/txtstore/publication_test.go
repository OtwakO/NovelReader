package txtstore

import (
	"errors"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
)

const novel = "第一章 開始\nFirst paragraph.\n第二章 後續\nSecond paragraph.\n第三章 結束\nLast paragraph.\n"

func analyzedReceipt(t *testing.T, store *Store) (Receipt, Interpretation) {
	t.Helper()
	receipt, err := store.Receive(t.Context(), "novel.txt", strings.NewReader(novel))
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := store.Analyze(t.Context(), receipt.ID, txt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return receipt, analysis
}

func TestSavedInterpretationAdmissionAndBoundedReading(t *testing.T) {
	store, _, home, root := receiptStore(t)
	receipt, analysis := analyzedReceipt(t, store)
	// A new store uses the persisted index, not analyzer-local state.
	reopened := NewStore(home.DB(), home.Files())
	preview, err := reopened.Preview(t.Context(), receipt.ID)
	if err != nil || !reflect.DeepEqual(preview, analysis) {
		t.Fatalf("saved preview=%+v err=%v", preview, err)
	}
	item, err := reopened.Accept(t.Context(), receipt.ID, preview.Version, "Same title", "Author")
	if err != nil || item.Provider != library.TXT || item.TotalChapterNum != len(preview.Analysis.Sections) {
		t.Fatalf("publication=%+v err=%v", item, err)
	}
	retry, err := reopened.Accept(t.Context(), receipt.ID, preview.Version, "Same title", "Author")
	if err != nil || !reflect.DeepEqual(retry, item) {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	stored, err := reopened.Get(t.Context(), receipt.ID)
	if err != nil || stored.Path != receipt.Path || stored.LibraryID != item.ID {
		t.Fatalf("file moved or link missing: %+v %v", stored, err)
	}
	var reconstructed strings.Builder
	for index := range preview.Analysis.Sections {
		content, err := reopened.ReadSection(t.Context(), item.ID, item.ContentRevision, index)
		if err != nil {
			t.Fatal(err)
		}
		reconstructed.WriteString(content.Text)
	}
	if reconstructed.String() != novel {
		t.Fatalf("decoded text changed: %q", reconstructed.String())
	}
	if _, err := reopened.ReadSection(t.Context(), item.ID, item.ContentRevision+1, 0); !errors.Is(err, library.ErrStateChanged) {
		t.Fatalf("stale revision error=%v", err)
	}
	if _, err := reopened.Analyze(t.Context(), receipt.ID, txt.Options{Preset: txt.GeneratedSections}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("published interpretation changed: %v", err)
	}
	if original, err := root.ReadFile(receipt.Path); err != nil || string(original) != novel {
		t.Fatalf("original changed: %q %v", original, err)
	}
	second, secondPreview := analyzedReceipt(t, store)
	other, err := store.Accept(t.Context(), second.ID, secondPreview.Version, "Same title", "Author")
	if err != nil || other.ID == item.ID {
		t.Fatalf("independent publication merged: %+v %v", other, err)
	}
}

func TestReanalysisRevokesStalePreview(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, first := analyzedReceipt(t, store)
	second, err := store.Analyze(t.Context(), receipt.ID, txt.Options{Preset: txt.GeneratedSections})
	if err != nil || second.Version <= first.Version {
		t.Fatalf("new preview=%+v err=%v", second, err)
	}
	if _, err := store.Accept(t.Context(), receipt.ID, first.Version, "Novel", ""); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale approval error=%v", err)
	}
	if err := store.saveInterpretation(t.Context(), receipt.ID, first.Version, first.Analysis); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale result error=%v", err)
	}
	if _, err := store.Analyze(t.Context(), receipt.ID, txt.Options{Encoding: txt.UTF16LE}); err == nil {
		t.Fatal("BOM-less original accepted as UTF-16")
	}
	failed, err := store.Get(t.Context(), receipt.ID)
	if err != nil || failed.State != AnalysisFailed || failed.Error == "" {
		t.Fatalf("failure=%+v err=%v", failed, err)
	}
	if _, err := store.Accept(t.Context(), receipt.ID, second.Version, "Novel", ""); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("failed replacement left old approval usable: %v", err)
	}
	// Restart recovery makes a claimed analysis retryable without publishing its old index.
	if _, err := home.DB().Exec(`UPDATE txt_files SET state=? WHERE id=?`, Analyzing, receipt.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Get(t.Context(), receipt.ID)
	if err != nil || recovered.State != Received || recovered.Options.Encoding != txt.UTF16LE {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}
	if _, err := store.Preview(t.Context(), receipt.ID); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("exposed old index during recovery: %v", err)
	}
}

func TestAdmissionFailureRollsBackProviderState(t *testing.T) {
	store, _, home, root := receiptStore(t)
	receipt, preview := analyzedReceipt(t, store)
	tx, err := home.DB().BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := library.InsertTx(t.Context(), tx, library.Item{ID: receipt.ID, Provider: "fixture", Name: "Existing"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Accept(t.Context(), receipt.ID, preview.Version, "Replacement", ""); err == nil {
		t.Fatal("overwrote an existing library ID")
	}
	pending, err := store.Get(t.Context(), receipt.ID)
	if err != nil || pending.LibraryID != "" || pending.State == Published {
		t.Fatalf("partial admission=%+v err=%v", pending, err)
	}
	item, err := library.NewStore(home.DB()).Get(t.Context(), receipt.ID)
	if err != nil || item.Name != "Existing" {
		t.Fatalf("existing item changed: %+v %v", item, err)
	}
	if _, err := root.Stat(receipt.Path); err != nil {
		t.Fatalf("admission failure lost original: %v", err)
	}
}

func TestPublishedRemovalHidesLibraryBeforeRetryableFileCleanup(t *testing.T) {
	store, _, home, root := receiptStore(t)
	receipt, preview := analyzedReceipt(t, store)
	item, err := store.Accept(t.Context(), receipt.ID, preview.Version, "Novel", "")
	if err != nil {
		t.Fatal(err)
	}
	shared := library.NewStore(home.DB())
	mark := library.Bookmark{ID: "mark", BookID: item.ID, ChapterIndex: 0, ChapterTitle: item.CurrentChapterTitle, Position: .25}
	version, err := shared.AddBookmark(t.Context(), &mark, library.Revision{Content: item.ContentRevision, State: item.StateVersion})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := shared.UpdateProgress(t.Context(), item.ID, library.Revision{Content: item.ContentRevision, State: version}, library.Location{ChapterIndex: 0, Position: .5, ChapterTitle: item.CurrentChapterTitle}); err != nil {
		t.Fatal(err)
	}
	extra := path.Join(path.Dir(receipt.Path), "unexpected")
	if err := root.WriteFile(extra, []byte("retain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Discard(t.Context(), receipt.ID); err == nil {
		t.Fatal("reported successful cleanup")
	}
	if visible, err := shared.Get(t.Context(), item.ID); err != nil || visible != nil {
		t.Fatalf("removed item still visible: %+v %v", visible, err)
	}
	if marks, err := shared.GetBookmarks(t.Context(), item.ID); err != nil || len(marks) != 0 {
		t.Fatalf("bookmarks survived: %+v %v", marks, err)
	}
	pending, err := store.Get(t.Context(), receipt.ID)
	if err != nil || pending.State != Removing || pending.LibraryID != "" {
		t.Fatalf("cleanup record=%+v err=%v", pending, err)
	}
	if err := root.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := home.DB().QueryRow(`SELECT count(*) FROM txt_sections WHERE receipt_id=?`, receipt.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan indexes=%d err=%v", count, err)
	}
}
