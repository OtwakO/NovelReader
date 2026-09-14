package txtstore

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/txt"
)

func TestInterpretationGenerationIsAllocatedBeforeWork(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	receipt := mustReceive(t, store)
	if receipt.AnalysisVersion == 0 {
		t.Fatal("acquisition did not create a candidate")
	}
	var state string
	if err := home.DB().QueryRow(`SELECT state FROM txt_files WHERE id=?`, receipt.ID).Scan(&state); err != nil || state != "acquired" {
		t.Fatalf("file lifecycle=%s: %v", state, err)
	}
	if worked, err := store.AnalyzeNext(t.Context()); err != nil || !worked {
		t.Fatalf("analysis=%v: %v", worked, err)
	}
	preview, err := store.Preview(t.Context(), receipt.ID)
	if err != nil || preview.Version != receipt.AnalysisVersion {
		t.Fatalf("worker changed generation: %+v: %v", preview, err)
	}
	if err := store.QueueAnalysis(t.Context(), receipt.ID, preview.Version, txt.Options{Preset: txt.GeneratedSections}); err != nil {
		t.Fatal(err)
	}
	var sections int
	if err := home.DB().QueryRow(`SELECT count(*) FROM txt_sections WHERE file_id=?`, receipt.ID).Scan(&sections); err != nil || sections != 0 {
		t.Fatalf("superseded sections=%d: %v", sections, err)
	}
	if err := store.saveInterpretation(t.Context(), receipt.ID, preview.Version, preview.Analysis); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("late result=%v", err)
	}
	claim, err := store.claimAnalysis(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.NewReader(strings.Repeat("x", 2*candidateCheckBytes))
	input := &candidateReader{ctx: t.Context(), store: store, fileID: receipt.ID, generation: claim.AnalysisVersion, input: source}
	if err := input.check(); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, candidateCheckBytes)
	if _, err := input.Read(buffer); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueAnalysis(t.Context(), receipt.ID, claim.AnalysisVersion, txt.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Read(buffer); !errors.Is(err, ErrStateChanged) || source.Len() != candidateCheckBytes {
		t.Fatalf("obsolete decoding continued: remaining=%d, %v", source.Len(), err)
	}
}

func TestPublishedReadingIgnoresCandidateWork(t *testing.T) {
	for _, test := range []struct {
		name     string
		encoding txt.Encoding
	}{{"completed", ""}, {"failed", txt.UTF16BE}} {
		t.Run(test.name, func(t *testing.T) {
			store, manager, home, _ := receiptStore(t)
			receipt, original := analyzedReceipt(t, store)
			item, err := store.Accept(t.Context(), receipt.ID, original.Version, "Novel", "")
			if err != nil {
				t.Fatal(err)
			}
			// Arrange the legal published+candidate state. Public reparse controls are a
			// later checkpoint; the existing worker must already honor interpretation roles.
			tx, err := home.DB().BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.Exec(`UPDATE txt_files SET generation=generation+1 WHERE id=?`, receipt.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(`INSERT INTO txt_interpretations(file_id,generation,role,state,requested_preset,requested_encoding,base_content_revision,queued_at,updated_at) SELECT id,generation,'candidate','queued',?,?,?,0,0 FROM txt_files WHERE id=?`, txt.GeneratedSections, test.encoding, item.ContentRevision, receipt.ID); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			claim, err := store.claimAnalysis(t.Context(), receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertActive := func() {
				t.Helper()
				value, err := store.Get(t.Context(), receipt.ID)
				if err != nil || value.State != Published || value.AnalysisVersion != original.Version {
					t.Fatalf("active receipt changed: %+v: %v", value, err)
				}
				catalog, revision, err := store.GetCatalog(t.Context(), item.ID)
				if err != nil || revision != item.ContentRevision || len(catalog) != len(original.Analysis.Sections) {
					t.Fatalf("active catalog changed: %d/%d: %v", len(catalog), revision, err)
				}
				content, err := store.ReadSection(t.Context(), item.ID, revision, 0)
				if err != nil || !strings.Contains(content.Text, "First paragraph.") {
					t.Fatalf("active read=%+v: %v", content, err)
				}
			}
			assertActive()
			if err := manager.SnapshotHome(t.Context(), alice, filepath.Join(t.TempDir(), "snapshot")); err != nil {
				t.Fatalf("active plus analyzing candidate is not portable: %v", err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if _, err := store.analyzeClaim(ctx, claim); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			assertActive()
			if worked, err := store.AnalyzeNext(t.Context()); !worked || (err != nil) != (test.encoding != "") {
				t.Fatalf("resume=%v: %v", worked, err)
			}
			var candidateState State
			if err := home.DB().QueryRow(`SELECT state FROM txt_interpretations WHERE file_id=? AND generation=?`, receipt.ID, claim.AnalysisVersion).Scan(&candidateState); err != nil {
				t.Fatal(err)
			}
			if (test.encoding != "" && candidateState != AnalysisFailed) || (test.encoding == "" && !hasInterpretation(candidateState)) {
				t.Fatalf("candidate result=%s", candidateState)
			}
			assertActive()
			if err := store.Discard(t.Context(), receipt.ID); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := home.DB().QueryRow(`SELECT count(*) FROM txt_interpretations WHERE file_id=?`, receipt.ID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("retained interpretations=%d: %v", count, err)
			}
			if err := store.saveInterpretation(t.Context(), receipt.ID, claim.AnalysisVersion, original.Analysis); !errors.Is(err, ErrStateChanged) {
				t.Fatalf("removed candidate resurrected: %v", err)
			}
		})
	}
}
