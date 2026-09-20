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

// Cleanup, ownership, index, publication semantics, and image-resource checks
// run in isolated homes here, not through live application/schema registration.
func TestPortablePreparationOwnership(t *testing.T) {
	for _, phase := range []string{"queued", "running", "before-move", "after-move", "damaged-after-move", "ready", "optimized-ready", "published"} {
		t.Run(phase, func(t *testing.T) {
			manager, err := readerstore.NewManager(t.TempDir(), 2, readerstore.ReaderSchema{Initialize: initializeFixtureSchema, PreparePortable: preparePortable, ValidatePortableFiles: validatePortableFiles})
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
			mode := epub.OriginalImages
			if phase == "optimized-ready" {
				mode = epub.OptimizedImages
			}
			receipt := acquiredFixture(t, source, mode)
			original, err := root.ReadFile(receipt.Path)
			if err != nil {
				t.Fatal(err)
			}
			attempt, err := source.QueuePreparation(t.Context(), receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "ready" || phase == "optimized-ready" || phase == "published" {
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
					if err == nil && (phase == "after-move" || phase == "damaged-after-move") {
						destination := preparationPath(receipt.ID, attempt.Generation)
						err = root.MkdirAll(path.Dir(destination), 0700)
						if err == nil {
							err = root.Rename(path.Join(receiptPreparationWorkPath(receipt.ID), staged.Directory()), destination)
						}
					}
					if err == nil && phase == "damaged-after-move" {
						err = root.WriteFile(path.Join(preparationPath(receipt.ID, attempt.Generation), sectionStreamFile), []byte("truncated"), 0600)
					}
					unlock()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if phase == "published" {
				if _, err = source.Accept(t.Context(), receipt.ID, attempt.Generation, "Portable book", "Author"); err != nil {
					t.Fatal(err)
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
			if phase == "published" {
				if _, err := source.db.Exec(`INSERT INTO epub_inbox_claims(name,receipt_id) VALUES(?,?)`, "synthetic.epub", receipt.ID); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := filepath.Join(t.TempDir(), "snapshot")
			if err = manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
				t.Fatal(err)
			}
			liveAfter, err := source.GetPreparation(t.Context(), receipt.ID, attempt.Generation)
			if err != nil || liveAfter != liveBefore {
				t.Fatal("live attempt changed", err)
			}
			if phase == "published" {
				var count int
				if err := source.db.QueryRow(`SELECT COUNT(*) FROM epub_inbox_claims`).Scan(&count); err != nil || count != 1 {
					t.Fatalf("live claims=%d %v", count, err)
				}
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
			var copiedClaims int
			if err := restored.db.QueryRow(`SELECT COUNT(*) FROM epub_inbox_claims`).Scan(&copiedClaims); err != nil || copiedClaims != 0 {
				t.Fatalf("restored claims=%d %v", copiedClaims, err)
			}
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
			} else if phase == "after-move" || phase == "ready" || phase == "optimized-ready" || phase == "published" {
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
			if phase == "published" {
				if _, revision, err := restored.GetCatalog(t.Context(), receipt.ID); err != nil || revision != 1 {
					t.Fatal("restored publication", revision, err)
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

func TestPortableHookRejectsCorruptReplacement(t *testing.T) {
	manager, err := readerstore.NewManager(t.TempDir(), 2, readerstore.ReaderSchema{Initialize: initializeFixtureSchema, PreparePortable: preparePortable, ValidatePortableFiles: validatePortableFiles})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	alice := readerstore.UserID("11111111-1111-4111-8111-111111111111")
	bob := readerstore.UserID("22222222-2222-4222-8222-222222222222")
	for _, id := range []readerstore.UserID{alice, bob} {
		if err = manager.Create(t.Context(), id); err != nil {
			t.Fatal(err)
		}
	}
	sourceHome, err := manager.Open(t.Context(), alice)
	if err != nil {
		t.Fatal(err)
	}
	defer sourceHome.Close()
	targetHome, err := manager.Open(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	defer targetHome.Close()
	source := NewStore(sourceHome.DB(), sourceHome.Files())
	receipt := acquiredFixture(t, source, epub.OriginalImages)
	attempt, err := source.QueuePreparation(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.Prepare(t.Context(), receipt.ID, attempt.Generation); err != nil {
		t.Fatal(err)
	}
	// Give the target real data to protect, not merely an empty home.
	target := NewStore(targetHome.DB(), targetHome.Files())
	targetReceipt := acquiredFixture(t, target, epub.OriginalImages)
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err = manager.SnapshotHome(t.Context(), alice, snapshot); err != nil {
		t.Fatal(err)
	}
	stream := filepath.Join(snapshot, readerstore.FilesDirectory, preparationPath(receipt.ID, attempt.Generation), sectionStreamFile)
	data, err := os.ReadFile(stream)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte(`"anchor":"a1"`), []byte(`"anchor":"a9"`), 1)
	if bytes.Equal(data, changed) {
		t.Fatal("fixture lacks anchor target")
	}
	if err = os.WriteFile(stream, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.PrepareReplacement(t.Context(), bob, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory)); !errors.Is(err, epub.ErrPreparedSection) {
		t.Fatal("corrupt replacement accepted", err)
	}
	var generation int64
	if err = target.db.QueryRow(`SELECT preparation_generation FROM epub_files WHERE id=?`, targetReceipt.ID).Scan(&generation); err != nil || generation != 0 {
		t.Fatal("target receipt changed", generation, err)
	}
	section, err := source.PreparedSection(t.Context(), receipt.ID, attempt.Generation, 0)
	if err != nil || section.Root.Kind != "group" {
		t.Fatal("source preparation changed", err)
	}
}
