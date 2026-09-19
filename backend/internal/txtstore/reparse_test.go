package txtstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
)

func TestReparseImpactApplyAndRetry(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, original := analyzedReceipt(t, store)
	item, err := store.Accept(t.Context(), receipt.ID, original.Version, "Novel", "Author")
	if err != nil {
		t.Fatal(err)
	}
	shared := library.NewStore(home.DB())
	mark := library.Bookmark{ID: "exact", BookID: item.ID, ChapterIndex: 2, ChapterTitle: original.Analysis.Sections[2].Title, Position: .4, Note: "Keep me"}
	state, err := shared.AddBookmark(t.Context(), &mark, item.Revision())
	if err != nil {
		t.Fatal(err)
	}
	state, err = shared.UpdateProgress(t.Context(), item.ID, library.Revision{Content: item.ContentRevision, State: state}, library.Location{ChapterIndex: 2, ChapterTitle: mark.ChapterTitle, Position: .7})
	if err != nil {
		t.Fatal(err)
	}
	// Merging the first two sections changes ordinals but leaves the final range exact.
	generation, err := store.QueueReparse(t.Context(), item.ID, item.ContentRevision, 0, txt.Options{Preset: txt.CustomPattern, Pattern: "第[一三]章.*"})
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := analyzeNext(t.Context(), store); !worked || err != nil {
		t.Fatalf("analysis=%v: %v", worked, err)
	}
	impact, err := store.ReparseImpact(t.Context(), item.ID, generation)
	if err != nil || impact.Resume == nil || impact.Resume.ChapterIndex != 1 || impact.Resume.Position != .7 || impact.PreservedBookmarks != 1 || impact.UnresolvedBookmarks != 0 {
		t.Fatalf("impact=%+v: %v", impact, err)
	}
	request := ApplyReparseRequest{Generation: generation, ActiveGeneration: original.Version, Expected: library.Revision{Content: item.ContentRevision, State: state}}
	// Scrolling after review invalidates only impact, not the prepared candidate.
	state, err = shared.UpdateProgress(t.Context(), item.ID, request.Expected, library.Location{ChapterIndex: 2, ChapterTitle: mark.ChapterTitle, Position: .8})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyReparse(t.Context(), item.ID, request); !errors.Is(err, library.ErrStateChanged) {
		t.Fatalf("stale review=%v", err)
	}
	request.Expected.State = state
	// Fail at promotion after library/bookmark writes to prove transaction rollback.
	if _, err := home.DB().Exec(`CREATE TRIGGER fail_reparse BEFORE UPDATE OF role ON txt_interpretations WHEN NEW.role='active' BEGIN SELECT RAISE(ABORT,'synthetic promotion failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyReparse(t.Context(), item.ID, request); err == nil {
		t.Fatal("expected rollback")
	}
	before, err := shared.Get(t.Context(), item.ID)
	if err != nil || before.Revision() != request.Expected || before.DurChapterIndex != 2 {
		t.Fatalf("partial library write=%+v: %v", before, err)
	}
	marks, err := shared.GetBookmarks(t.Context(), item.ID)
	if err != nil || !reflect.DeepEqual(marks, []library.Bookmark{mark}) {
		t.Fatalf("partial bookmark write=%+v: %v", marks, err)
	}
	if _, err := home.DB().Exec(`DROP TRIGGER fail_reparse`); err != nil {
		t.Fatal(err)
	}
	result, err := store.ApplyReparse(t.Context(), item.ID, request)
	if err != nil || result.AlreadyApplied || result.Item.ContentRevision != item.ContentRevision+1 || result.Item.StateVersion != state+1 || result.Item.DurChapterIndex != 1 || result.Item.DurChapterPos != .8 || result.Item.Name != item.Name || result.Item.Author != item.Author {
		t.Fatalf("apply=%+v: %v", result, err)
	}
	marks, err = shared.GetBookmarks(t.Context(), item.ID)
	if err != nil || len(marks) != 1 || marks[0].ChapterIndex != 1 || marks[0].ContentRevision != result.Item.ContentRevision || marks[0].Position != mark.Position || marks[0].Note != mark.Note || marks[0].Orphaned {
		t.Fatalf("mapped mark=%+v: %v", marks, err)
	}
	if _, err := store.ReadSection(t.Context(), item.ID, item.ContentRevision, 1); !errors.Is(err, library.ErrStateChanged) {
		t.Fatalf("stale content=%v", err)
	}
	retried, err := store.ApplyReparse(t.Context(), item.ID, request)
	if err != nil || !retried.AlreadyApplied || !reflect.DeepEqual(retried.Item, result.Item) {
		t.Fatalf("retry=%+v: %v", retried, err)
	}
}

func TestReparseRequiresExplicitResumeAndNeverRevivesOrphans(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt, original := analyzedReceipt(t, store)
	item, err := store.Accept(t.Context(), receipt.ID, original.Version, "Novel", "")
	if err != nil {
		t.Fatal(err)
	}
	shared := library.NewStore(home.DB())
	mark := library.Bookmark{ID: "unmapped", BookID: item.ID, ChapterIndex: 0, ChapterTitle: item.CurrentChapterTitle, Position: .5, Note: "Retain original location"}
	state, err := shared.AddBookmark(t.Context(), &mark, item.Revision())
	if err != nil {
		t.Fatal(err)
	}
	generation, err := store.QueueReparse(t.Context(), item.ID, item.ContentRevision, 0, txt.Options{Preset: txt.GeneratedSections})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := analyzeNext(t.Context(), store); err != nil {
		t.Fatal(err)
	}
	impact, err := store.ReparseImpact(t.Context(), item.ID, generation)
	if err != nil || impact.Resume != nil || impact.UnresolvedBookmarks != 1 {
		t.Fatalf("impact=%+v: %v", impact, err)
	}
	request := ApplyReparseRequest{Generation: generation, ActiveGeneration: original.Version, Expected: library.Revision{Content: item.ContentRevision, State: state}}
	if _, err := store.ApplyReparse(t.Context(), item.ID, request); !errors.Is(err, ErrResumeRequired) {
		t.Fatalf("missing choice=%v", err)
	}
	index := 5
	request.ResumeChapter = &index
	if _, err := store.ApplyReparse(t.Context(), item.ID, request); !errors.Is(err, ErrInvalidResume) {
		t.Fatalf("invalid choice=%v", err)
	}
	index = 0
	result, err := store.ApplyReparse(t.Context(), item.ID, request)
	if err != nil || result.Item.DurChapterPos != 0 {
		t.Fatalf("explicit resume=%+v: %v", result, err)
	}
	mark.Orphaned = true
	marks, err := shared.GetBookmarks(t.Context(), item.ID)
	if err != nil || !reflect.DeepEqual(marks, []library.Bookmark{mark}) {
		t.Fatalf("orphan lost original data=%+v: %v", marks, err)
	}
	// Recreating the original boundaries is not authority to revive an old bookmark.
	generation, err = store.QueueReparse(t.Context(), item.ID, result.Item.ContentRevision, 0, txt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := analyzeNext(t.Context(), store); err != nil {
		t.Fatal(err)
	}
	impact, err = store.ReparseImpact(t.Context(), item.ID, generation)
	if err != nil || impact.PreservedBookmarks != 0 || impact.UnresolvedBookmarks != 1 {
		t.Fatalf("revived old mark=%+v: %v", impact, err)
	}
}
