package epubstore

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func acquiredFixture(t *testing.T, s *Store, mode epub.ImageMode) Receipt {
	t.Helper()
	data := stagedFixture(t, false)
	r, err := s.Receive(t.Context(), rand.Text(), "novel.epub", bytes.NewReader(data), mode)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func stageReceipt(t *testing.T, root *os.Root, r Receipt) *StagedPreparation {
	t.Helper()
	if err := root.MkdirAll(receiptPreparationWorkPath(r.ID), 0700); err != nil {
		t.Fatal(err)
	}
	work, err := root.OpenRoot(receiptPreparationWorkPath(r.ID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { work.Close() })
	original, err := root.Open(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Stage(t.Context(), work, original, r.Size, r.ImageMode)
	if closeErr := original.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { staged.Close() })
	return staged
}

func TestPreparePersistsIndexedOutput(t *testing.T) {
	for _, mode := range []epub.ImageMode{epub.OriginalImages, epub.OptimizedImages} {
		t.Run(string(mode), func(t *testing.T) {
			store, root := receiptStore(t)
			r := acquiredFixture(t, store, mode)
			a, err := store.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Prepare(t.Context(), r.ID, a.Generation); err != nil {
				t.Fatal(err)
			}
			a, err = store.GetPreparation(t.Context(), r.ID, a.Generation)
			if err != nil || a.State != PreparationReady {
				t.Fatal(a, err)
			}
			metadata, err := store.PreparedMetadata(t.Context(), r.ID, a.Generation)
			if err != nil || len(metadata.Sections) != 2 || metadata.ImageProcessing.Mode != mode {
				t.Fatal(metadata, err)
			}
			resources, err := store.preparedResources(t.Context(), r.ID, a.Generation)
			if err != nil || len(resources) != 1 || resources[0].Image != *metadata.Cover {
				t.Fatal("resource identity", err)
			}
			for _, ordinal := range []int{1, 0} {
				section, err := store.PreparedSection(t.Context(), r.ID, a.Generation, ordinal)
				if err != nil || section.Ordinal != ordinal {
					t.Fatal(section, err)
				}
				for _, image := range section.Images {
					if image != resources[0].Image {
						t.Fatal("binding mismatch")
					}
				}
			}
			if _, err = store.QueuePreparation(t.Context(), r.ID); !errors.Is(err, ErrStateChanged) {
				t.Fatal("superseded ready output", err)
			}
			directory := preparationPath(r.ID, a.Generation)
			if _, err = root.Stat(path.Join(directory, sectionStreamFile)); err != nil {
				t.Fatal(err)
			}
			if mode == epub.OptimizedImages && (resources[0].DerivativeBytes <= 0 || metadata.ImageProcessing.Backend == "") {
				t.Fatal("encoder evidence lost")
			}
			if err = store.Discard(t.Context(), r.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = root.Stat(directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("managed output retained", err)
			}
			for _, table := range []string{"epub_sections", "epub_resources"} {
				var count int
				if err = store.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
					t.Fatal("orphan index", err)
				}
			}
		})
	}
}

func TestFinalizationRejectsStaleGeneration(t *testing.T) {
	store, root := receiptStore(t)
	r := acquiredFixture(t, store, epub.OriginalImages)
	a, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	a, err = store.ClaimPreparation(t.Context(), r.ID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	staged := stageReceipt(t, root, r)
	next, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.finalizePreparation(t.Context(), root, a, staged); !errors.Is(err, ErrStateChanged) {
		t.Fatal(err)
	}
	got, err := store.GetPreparation(t.Context(), r.ID, next.Generation)
	if err != nil || !reflect.DeepEqual(got, next) {
		t.Fatal("changed replacement", err)
	}
	if _, err = root.Stat(preparationPath(r.ID, a.Generation)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("installed stale files", err)
	}
	var count int
	if err = store.db.QueryRow(`SELECT count(*) FROM epub_sections`).Scan(&count); err != nil || count != 0 {
		t.Fatal("stale index", err)
	}
}

func TestPreparationRecoveryAroundDirectoryMove(t *testing.T) {
	for _, boundary := range []string{"before-move", "after-move", "missing-derivative", "index-unavailable"} {
		t.Run(boundary, func(t *testing.T) {
			store, root := receiptStore(t)
			r := acquiredFixture(t, store, epub.OptimizedImages)
			original, err := root.ReadFile(r.Path)
			if err != nil {
				t.Fatal(err)
			}
			a, err := store.QueuePreparation(t.Context(), r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "before-move" {
				a, err = store.ClaimPreparation(t.Context(), r.ID, a.Generation)
				if err != nil {
					t.Fatal(err)
				}
				staged := stageReceipt(t, root, r)
				unlock, err := store.files.LockMutation(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				err = store.persistPreparationIntent(t.Context(), a, staged)
				unlock()
				if err != nil {
					t.Fatal(err)
				}
				// Simulate process interruption: no worker retains an open work root.
				if err = staged.work.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				_, err = store.db.Exec(`CREATE TRIGGER fail_ready BEFORE UPDATE ON epub_preparations WHEN NEW.state='ready' BEGIN SELECT RAISE(FAIL,'synthetic readiness failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
				if err = store.Prepare(t.Context(), r.ID, a.Generation); err == nil {
					t.Fatal("expected failure")
				}
				if _, err = store.db.Exec(`DROP TRIGGER fail_ready`); err != nil {
					t.Fatal(err)
				}
			}
			got, err := store.GetPreparation(t.Context(), r.ID, a.Generation)
			if err != nil || got.State != PreparationFinalizing {
				t.Fatal("lost intent", err)
			}
			if _, err = store.PreparedSection(t.Context(), r.ID, a.Generation, 0); !errors.Is(err, ErrNotFound) {
				t.Fatal("exposed incomplete generation", err)
			}
			if boundary == "missing-derivative" {
				resources, err := store.preparedResources(t.Context(), r.ID, a.Generation)
				if err != nil {
					t.Fatal(err)
				}
				if err = root.Remove(path.Join(preparationPath(r.ID, a.Generation), "images", resources[0].Image.DerivativeID+".webp")); err != nil {
					t.Fatal(err)
				}
			}
			if boundary == "index-unavailable" {
				if _, err = store.db.Exec(`ALTER TABLE epub_sections RENAME TO unavailable_sections`); err != nil {
					t.Fatal(err)
				}
				if err = store.RecoverPreparations(t.Context()); err == nil {
					t.Fatal("expected index error")
				}
				if _, err = root.Stat(preparationPath(r.ID, a.Generation)); err != nil {
					t.Fatal("deleted output after database error", err)
				}
				if _, err = store.db.Exec(`ALTER TABLE unavailable_sections RENAME TO epub_sections`); err != nil {
					t.Fatal(err)
				}
			}
			if err = store.RecoverPreparations(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err = store.RecoverPreparations(t.Context()); err != nil {
				t.Fatal(err)
			}
			got, err = store.GetPreparation(t.Context(), r.ID, a.Generation)
			if err != nil {
				t.Fatal(err)
			}
			want := PreparationReady
			if boundary == "missing-derivative" {
				want = PreparationFailed
			}
			if got.State != want {
				t.Fatalf("%+v", got)
			}
			if want == PreparationReady {
				if _, err = store.PreparedSection(t.Context(), r.ID, a.Generation, 1); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err = root.Stat(preparationPath(r.ID, a.Generation)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("incomplete output retained", err)
				}
			}
			if data, err := root.ReadFile(r.Path); err != nil || !bytes.Equal(data, original) {
				t.Fatal("original changed", err)
			}
		})
	}
}
