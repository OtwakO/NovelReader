package fileimport

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

func pendingEPUB(t *testing.T, home *readerstore.Home) epubstore.PreparationAttempt {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for _, entry := range [][2]string{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"book.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest><item id="section" href="section.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="section"/></spine></package>`},
		{"section.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Synthetic shared worker fixture.</p></body></html>`},
	} {
		w, err := archive.CreateHeader(&zip.FileHeader{Name: entry[0], Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(entry[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	store := epubstore.NewStore(home.DB(), home.Files())
	receipt, err := store.Receive(t.Context(), rand.Text(), "fixture.epub", bytes.NewReader(data.Bytes()), epub.OriginalImages)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := store.QueuePreparation(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func TestMixedWorkOldestFirst(t *testing.T) {
	for _, epubFirst := range []bool{false, true} {
		name := "TXT first"
		if epubFirst {
			name = "EPUB first"
		}
		t.Run(name, func(t *testing.T) {
			readers := recoveryManager(t, 2)
			defer readers.Close()
			home, err := readers.Open(t.Context(), recoveryAlice)
			if err != nil {
				t.Fatal(err)
			}
			defer home.Close()
			text := pendingReceipt(t, home)
			book := pendingEPUB(t, home)
			textTime, epubTime := int64(10), int64(20)
			if epubFirst {
				textTime, epubTime = 20, 10
			}
			if _, err = home.DB().Exec(`UPDATE txt_interpretations SET queued_at=?;`, textTime); err != nil {
				t.Fatal(err)
			}
			if _, err = home.DB().Exec(`UPDATE epub_preparations SET created_at=?;`, epubTime); err != nil {
				t.Fatal(err)
			}
			if worked, err := prepareHome(t.Context(), home); !worked || err != nil {
				t.Fatal(worked, err)
			}
			texts := txtstore.NewStore(home.DB(), home.Files())
			books := epubstore.NewStore(home.DB(), home.Files())
			textPending, bookPending, err := pendingWork(t.Context(), home.DB())
			if err != nil {
				t.Fatal(err)
			}
			if epubFirst {
				if textPending.ReceiptID != text.ID || bookPending.ReceiptID != "" {
					t.Fatal("wrong first format", textPending, bookPending)
				}
			} else if textPending.ReceiptID != "" || bookPending.ReceiptID != book.ReceiptID {
				t.Fatal("wrong first format", textPending, bookPending)
			}
			if worked, err := prepareHome(t.Context(), home); !worked || err != nil {
				t.Fatal(worked, err)
			}
			if _, err = texts.Preview(t.Context(), text.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = books.PreparedSection(t.Context(), book.ReceiptID, book.Generation, 0); err != nil {
				t.Fatal(err)
			}
			if worked, err := prepareHome(t.Context(), home); worked || err != nil {
				t.Fatal("completed work repeated", worked, err)
			}
		})
	}
}

func TestSelectedGenerationsCannotClaimReplacements(t *testing.T) {
	readers := recoveryManager(t, 2)
	defer readers.Close()
	home, err := readers.Open(t.Context(), recoveryAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	text := pendingReceipt(t, home)
	book := pendingEPUB(t, home)
	texts := txtstore.NewStore(home.DB(), home.Files())
	books := epubstore.NewStore(home.DB(), home.Files())
	pending, _, err := pendingWork(t.Context(), home.DB())
	if err != nil {
		t.Fatal(err)
	}
	if err = texts.QueueAnalysis(t.Context(), text.ID, text.AnalysisVersion, txt.Options{}); err != nil {
		t.Fatal(err)
	}
	replacement, err := books.QueuePreparation(t.Context(), book.ReceiptID)
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := texts.AnalyzePending(t.Context(), pending); worked || err != nil {
		t.Fatal("stale TXT claim", worked, err)
	}
	if worked, err := books.PreparePending(t.Context(), book); worked || !errors.Is(err, epubstore.ErrStateChanged) {
		t.Fatal("stale EPUB claim", worked, err)
	}
	current, next, err := pendingWork(t.Context(), home.DB())
	if err != nil || current.Generation <= pending.Generation {
		t.Fatal("TXT replacement lost", err)
	}
	if err != nil || next.Generation != replacement.Generation {
		t.Fatal("EPUB replacement lost", err)
	}
}

func TestEPUBQuiesceReleasesLeaseAndResumesDurableWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		readers := recoveryManager(t, 2)
		defer readers.Close()
		home, err := readers.Open(t.Context(), recoveryAlice)
		if err != nil {
			t.Fatal(err)
		}
		book := pendingEPUB(t, home)
		unlock, err := home.Files().LockMutation(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		pool := NewPool(readers)
		defer pool.Close()
		if err = pool.Notify(recoveryAlice); err != nil {
			t.Fatal(err)
		}
		synctest.Wait() // Real EPUB dispatch is waiting for the claim's mutation gate.
		if err = pool.Quiesce(t.Context(), recoveryAlice); err != nil {
			t.Fatal(err)
		}
		unlock()
		home.Close()
		// Snapshot/replacement needs all worker leases released, not just cancellation.
		snapshot := filepath.Join(t.TempDir(), "snapshot")
		if err = readers.SnapshotHome(t.Context(), recoveryAlice, snapshot); err != nil {
			t.Fatal(err)
		}
		replacement, err := readers.PrepareReplacement(t.Context(), recoveryAlice, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory))
		if err != nil {
			t.Fatal(err)
		}
		if err = readers.PublishReplacement(t.Context(), recoveryAlice, replacement); err != nil {
			t.Fatal(err)
		}
		if err = RecoverHome(t.Context(), readers, recoveryAlice); err != nil {
			t.Fatal(err)
		}
		pool.Resume(recoveryAlice)
		synctest.Wait()
		assertRetired(t, pool)
		home, err = readers.Open(t.Context(), recoveryAlice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		if _, err = epubstore.NewStore(home.DB(), home.Files()).PreparedSection(t.Context(), book.ReceiptID, book.Generation, 0); err != nil {
			t.Fatal(err)
		}
	})
}

func TestFailedEPUBDoesNotBlockTXT(t *testing.T) {
	readers := recoveryManager(t, 2)
	defer readers.Close()
	home, err := readers.Open(t.Context(), recoveryAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	books := epubstore.NewStore(home.DB(), home.Files())
	receipt, err := books.Receive(t.Context(), rand.Text(), "broken.epub", bytes.NewBufferString("not an EPUB"), epub.OriginalImages)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := books.QueuePreparation(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	text := pendingReceipt(t, home)
	if _, err = home.DB().Exec(`UPDATE epub_preparations SET created_at=1`); err != nil {
		t.Fatal(err)
	}
	if worked, err := prepareHome(t.Context(), home); !worked || err == nil {
		t.Fatal("failed claim must yield another turn", worked, err)
	}
	failed, err := books.GetPreparation(t.Context(), receipt.ID, attempt.Generation)
	if err != nil || failed.State != epubstore.PreparationFailed {
		t.Fatal("failure not retained", failed, err)
	}
	if worked, err := prepareHome(t.Context(), home); !worked || err != nil {
		t.Fatal("TXT blocked", worked, err)
	}
	if _, err = txtstore.NewStore(home.DB(), home.Files()).Preview(t.Context(), text.ID); err != nil {
		t.Fatal(err)
	}
}
