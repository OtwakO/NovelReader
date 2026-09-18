package epubstore

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/readerstore"
)

// Only cleanup and ownership checks are registered in this isolated fixture.
// This does not prove complete untrusted EPUB portable validation.
func TestPortablePreparationOwnership(t *testing.T) {
	for _, phase := range []string{"queued", "running", "before-move", "after-move", "ready"} {
		t.Run(phase, func(t *testing.T) {
			manager, err := readerstore.NewManager(t.TempDir(), 2, readerstore.ReaderSchema{Initialize: initializeSchema, PreparePortable: preparePortable, ValidatePortableFiles: validatePortableOwnership})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { manager.Close() })
			alice := readerstore.UserID("11111111-1111-4111-8111-111111111111")
			bob := readerstore.UserID("22222222-2222-4222-8222-222222222222")
			for _, id := range []readerstore.UserID{alice, bob} {
				if err = manager.Create(t.Context(), id); err != nil {
					t.Fatal(err)
				}
			}
			home, err := manager.Open(t.Context(), alice)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { home.Close() })
			root, err := home.Files().OpenRoot()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { root.Close() })
			source := NewStore(home.DB(), home.Files())
			receipt := acquiredFixture(t, source, epub.OriginalImages)
			original, err := root.ReadFile(receipt.Path)
			if err != nil {
				t.Fatal(err)
			}
			attempt, err := source.QueuePreparation(t.Context(), receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "ready" {
				if err = source.Prepare(t.Context(), receipt.ID, attempt.Generation); err != nil {
					t.Fatal(err)
				}
			} else if phase != "queued" {
				attempt, err = source.ClaimPreparation(t.Context(), receipt.ID, attempt.Generation)
				if err != nil {
					t.Fatal(err)
				}
				if phase != "running" {
					staged := stageReceipt(t, root, receipt)
					unlock, err := source.files.LockMutation(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					err = source.persistPreparationIntent(t.Context(), attempt, staged)
					if err == nil && phase == "after-move" {
						destination := preparationPath(receipt.ID, attempt.Generation)
						err = root.MkdirAll(path.Dir(destination), 0700)
						if err == nil {
							err = root.Rename(path.Join(receiptPreparationWorkPath(receipt.ID), staged.Directory()), destination)
						}
					}
					unlock()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			liveBefore, err := source.GetPreparation(t.Context(), receipt.ID, attempt.Generation)
			if err != nil {
				t.Fatal(err)
			}
			var localStage string
			if err = source.db.QueryRow(`SELECT stage_name FROM epub_preparations WHERE file_id=?`, receipt.ID).Scan(&localStage); err != nil {
				t.Fatal(err)
			}
			snapshot := filepath.Join(t.TempDir(), "snapshot")
			if err = manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
				t.Fatal(err)
			}
			liveAfter, err := source.GetPreparation(t.Context(), receipt.ID, attempt.Generation)
			if err != nil || liveAfter != liveBefore {
				t.Fatal("live attempt changed", err)
			}
			var unchangedStage string
			if err = source.db.QueryRow(`SELECT stage_name FROM epub_preparations WHERE file_id=?`, receipt.ID).Scan(&unchangedStage); err != nil || unchangedStage != localStage {
				t.Fatal("live staging authority changed", err)
			}
			if _, err = os.Stat(filepath.Join(snapshot, readerstore.FilesDirectory, readerstore.WorkDirectory)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("copied temporary work", err)
			}
			// Replacement invokes the hook again, covering idempotency on the same copy.
			replacement, err := manager.PrepareReplacement(t.Context(), bob, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory))
			if err != nil {
				t.Fatal(err)
			}
			if err = manager.PublishReplacement(t.Context(), bob, replacement); err != nil {
				t.Fatal(err)
			}
			restoredHome, err := manager.Open(t.Context(), bob)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { restoredHome.Close() })
			restored := NewStore(restoredHome.DB(), restoredHome.Files())
			var copiedStage string
			if err = restored.db.QueryRow(`SELECT stage_name FROM epub_preparations WHERE file_id=?`, receipt.ID).Scan(&copiedStage); err != nil || copiedStage != "" {
				t.Fatal("local staging authority retained", err)
			}
			copiedAttempt, err := restored.GetPreparation(t.Context(), receipt.ID, attempt.Generation)
			expectedCopy := liveBefore
			if phase == "running" {
				expectedCopy.State = PreparationFailed
				expectedCopy.Error = "preparation interrupted by portable copy; retry required"
			}
			if err != nil || copiedAttempt != expectedCopy {
				t.Fatal("unexpected copied attempt", copiedAttempt, err)
			}
			if err = restored.RecoverAcquisitions(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err = restored.RecoverPreparations(t.Context()); err != nil {
				t.Fatal(err)
			}
			got, err := restored.GetPreparation(t.Context(), receipt.ID, attempt.Generation)
			if err != nil {
				t.Fatal(err)
			}
			want := PreparationFailed
			if phase == "queued" {
				want = PreparationQueued
			} else if phase == "after-move" || phase == "ready" {
				want = PreparationReady
			}
			if got.State != want {
				t.Fatalf("state=%s want=%s", got.State, want)
			}
			if want == PreparationReady {
				if _, err = restored.PreparedSection(t.Context(), receipt.ID, attempt.Generation, 1); err != nil {
					t.Fatal(err)
				}
			}
			restoredRoot, err := restoredHome.Files().OpenRoot()
			if err != nil {
				t.Fatal(err)
			}
			defer restoredRoot.Close()
			copied, err := restoredRoot.ReadFile(receipt.Path)
			if err != nil || !bytes.Equal(copied, original) {
				t.Fatal("original changed", err)
			}
		})
	}
}
