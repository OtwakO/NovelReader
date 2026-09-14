package txtstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txt"
)

func TestPublishedPortableReferencesAndMissingOriginal(t *testing.T) {
	store, manager, _, root := receiptStore(t)
	receipt, preview := analyzedReceipt(t, store)
	preview, err := store.Analyze(t.Context(), receipt.ID, txt.Options{Preset: txt.CustomPattern, Pattern: `Chapter [0-9]+`})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Accept(t.Context(), receipt.ID, preview.Version, "Novel", "")
	if err != nil {
		t.Fatal(err)
	}
	// Saved content remains portable/readable without recompiling request syntax.
	if _, err := store.db.Exec(`UPDATE txt_interpretations SET requested_pattern='(' WHERE file_id=?`, receipt.ID); err != nil {
		t.Fatal(err)
	}
	// A portable candidate is preparation, not permission to replace active content.
	generation, err := store.QueueReparse(t.Context(), item.ID, item.ContentRevision, 0, txt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.claimAnalysis(t.Context(), receipt.ID); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(snapshot, readerstore.ReaderDatabaseName)
	files := filepath.Join(snapshot, readerstore.FilesDirectory)
	stage, err := manager.PrepareReplacement(t.Context(), bob, database, files)
	if err != nil {
		t.Fatal(err)
	}
	stagedOriginal := filepath.Join(stage, readerstore.FilesDirectory, filepath.FromSlash(receipt.Path))
	if err := os.Remove(stagedOriginal); err != nil {
		t.Fatal(err)
	}
	if err := manager.PublishReplacement(t.Context(), bob, stage); err == nil {
		t.Fatal("published invalid staged references")
	}
	if err := os.WriteFile(stagedOriginal, []byte(novel), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.PublishReplacement(t.Context(), bob, stage); err != nil {
		t.Fatal(err)
	}
	restored, err := manager.Open(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	restoredTXT := NewStore(restored.DB(), restored.Files())
	content, err := restoredTXT.ReadSection(t.Context(), item.ID, item.ContentRevision, 0)
	if err != nil || !strings.Contains(content.Text, "First paragraph.") {
		t.Fatalf("restored read=%+v err=%v", content, err)
	}
	if err := restoredTXT.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if worked, err := restoredTXT.AnalyzeNext(t.Context()); !worked || err != nil {
		t.Fatalf("restored candidate=%v: %v", worked, err)
	}
	status, err := restoredTXT.ReparseStatus(t.Context(), item.ID)
	if err != nil || status.ActiveGeneration != preview.Version || status.Candidate == nil || status.Candidate.Generation != generation {
		t.Fatalf("recovery changed active/candidate identity=%+v: %v", status, err)
	}
	index := 1
	applied, err := restoredTXT.ApplyReparse(t.Context(), item.ID, ApplyReparseRequest{Generation: generation, ActiveGeneration: preview.Version, Expected: item.Revision(), ResumeChapter: &index})
	if err != nil {
		t.Fatal(err)
	}
	item = applied.Item
	restored.Close()
	if err := manager.SnapshotHome(t.Context(), bob, filepath.Join(t.TempDir(), "applied-snapshot")); err != nil {
		t.Fatalf("applied index is not portable: %v", err)
	}
	// An incomplete archive must not replace the working reader home.
	if err := os.Remove(filepath.Join(files, filepath.FromSlash(receipt.Path))); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareReplacement(t.Context(), bob, database, files); err == nil {
		t.Fatal("accepted missing archive original")
	}
	restored, err = manager.Open(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewStore(restored.DB(), restored.Files()).ReadSection(t.Context(), item.ID, item.ContentRevision, 0)
	restored.Close()
	if err != nil {
		t.Fatalf("failed staging damaged published home: %v", err)
	}
	// The same invariant applies to exports, without deleting the live receipt.
	if err := root.Remove(receipt.Path); err != nil {
		t.Fatal(err)
	}
	rejected := filepath.Join(t.TempDir(), "rejected")
	if err := manager.SnapshotHome(t.Context(), alice, rejected); err == nil {
		t.Fatal("exported missing original")
	}
	if _, err := os.Stat(rejected); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed snapshot remains: %v", err)
	}
	if value, err := store.Get(t.Context(), receipt.ID); err != nil || value.State != Published {
		t.Fatalf("live receipt changed: %+v %v", value, err)
	}
}

func TestPortableOwnershipRejectsBrokenLinks(t *testing.T) {
	for _, corruption := range []string{"missing-library", "wrong-provider", "missing-receipt", "foreign-path", "wrong-size", "missing-active", "section-gap", "generation-counter"} {
		t.Run(corruption, func(t *testing.T) {
			store, manager, home, root := receiptStore(t)
			receipt, preview := analyzedReceipt(t, store)
			item, err := store.Accept(t.Context(), receipt.ID, preview.Version, "Novel", "")
			if err != nil {
				t.Fatal(err)
			}
			switch corruption {
			case "missing-active":
				if _, err := home.DB().Exec(`DELETE FROM txt_interpretations WHERE file_id=?`, receipt.ID); err != nil {
					t.Fatal(err)
				}
			case "section-gap":
				if _, err := home.DB().Exec(`UPDATE txt_sections SET start_byte=start_byte+1 WHERE file_id=? AND idx=1`, receipt.ID); err != nil {
					t.Fatal(err)
				}
			case "generation-counter":
				if _, err := home.DB().Exec(`UPDATE txt_files SET generation=0 WHERE id=?`, receipt.ID); err != nil {
					t.Fatal(err)
				}
			case "missing-library":
				tx, err := home.DB().BeginTx(t.Context(), nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if err := library.DeleteTx(t.Context(), tx, item.ID); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			case "wrong-provider":
				// Deliberately corrupt the fixture, not a supported shared-library mutation.
				_, err = home.DB().Exec(`UPDATE library_items SET provider=? WHERE id=?`, library.BookSource, item.ID)
			case "missing-receipt":
				_, err = home.DB().Exec(`DELETE FROM txt_files WHERE id=?`, receipt.ID)
			case "foreign-path":
				_, err = home.DB().Exec(`UPDATE txt_files SET path=? WHERE id=?`, "fonts/protected", receipt.ID)
			case "wrong-size":
				err = root.WriteFile(receipt.Path, []byte("changed"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.SnapshotHome(t.Context(), alice, filepath.Join(t.TempDir(), "snapshot")); err == nil {
				t.Fatal("exported broken ownership")
			}
		})
	}
}

func TestPortableUnfinishedReceiptsRemainRecoverable(t *testing.T) {
	store, manager, _, root := receiptStore(t)
	receiving := mustReceive(t, store)
	removing := mustReceive(t, store)
	interruptAcquisition(t, store, receiving.ID)
	if err := store.beginRemoval(t.Context(), removing); err != nil {
		t.Fatal(err)
	}
	for _, value := range []Receipt{receiving, removing} {
		if err := root.Remove(value.Path); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	stage, err := manager.PrepareReplacement(t.Context(), bob, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory))
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
	if value, err := restoredStore.Get(t.Context(), receiving.ID); err != nil || value.State != Failed {
		t.Fatalf("interrupted receipt=%+v %v", value, err)
	}
	if _, err := restoredStore.Get(t.Context(), removing.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removal recovery=%v", err)
	}
}
