package txtstore

import (
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/txt"
)

// A representative indexed Apply check, not a throughput benchmark or latency gate.
// ASCII has identical source ranges under both encodings, but that coincidence
// cannot authorize correspondence for an encoding change.
func TestReparseApplyIndexedCatalogAndEncodingChange(t *testing.T) {
	store, _, _, _ := receiptStore(t)
	var source strings.Builder
	const sections = 2000
	for index := range sections {
		fmt.Fprintf(&source, "Chapter %d\nParagraph.\n", index+1)
	}
	receipt, err := store.Receive(t.Context(), rand.Text(), "indexed.txt", strings.NewReader(source.String()))
	if err != nil {
		t.Fatal(err)
	}
	original, err := store.Analyze(t.Context(), receipt.ID, txt.Options{Encoding: txt.UTF8, Preset: txt.EnglishChapters})
	if err != nil || len(original.Analysis.Sections) != sections {
		t.Fatalf("original sections=%d: %v", len(original.Analysis.Sections), err)
	}
	item, err := store.Accept(t.Context(), receipt.ID, original.Version, "Indexed novel", "")
	if err != nil {
		t.Fatal(err)
	}
	generation, err := store.QueueReparse(t.Context(), item.ID, item.ContentRevision, 0, txt.Options{Encoding: txt.GB18030, Preset: txt.EnglishChapters})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AnalyzeNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	impact, err := store.ReparseImpact(t.Context(), item.ID, generation)
	if err != nil || impact.Resume != nil || impact.TotalSections != sections {
		t.Fatalf("encoding correspondence=%+v: %v", impact, err)
	}
	index := sections / 2
	start := time.Now()
	result, err := store.ApplyReparse(t.Context(), item.ID, ApplyReparseRequest{Generation: generation, ActiveGeneration: original.Version, Expected: item.Revision(), ResumeChapter: &index})
	elapsed := time.Since(start)
	if err != nil || result.Item.TotalChapterNum != sections || result.Item.DurChapterIndex != index {
		t.Fatalf("apply=%+v: %v", result, err)
	}
	t.Logf("Apply with %d sections (including old-index deletion): %s", sections, elapsed)
}
