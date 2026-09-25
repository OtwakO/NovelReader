package api

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/backup"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/txtstore"
)

func TestRestoreReconcilesTXTAndReportsCommittedWarning(t *testing.T) {
	server, sessions, readers, alice, closeStores := newOwnershipServer(t)
	defer closeStores()
	if err := server.quiesceReader(t.Context(), alice); err != nil {
		t.Fatal(err)
	}
	if err := server.fileImports.Notify(alice); !errors.Is(err, fileimport.ErrPaused) {
		t.Fatalf("source not paused: %v", err)
	}
	if _, err := server.fileAdmission.Request(alice); !errors.Is(err, fileimport.ErrPaused) {
		t.Fatalf("intake not paused: %v", err)
	}
	oldTicket, err := server.fileAdmission.Request(ownershipBob)
	if err != nil {
		t.Fatal(err)
	}
	// The other reader is still usable while the source is quiescent.
	if got := authenticatedOwnershipRequest(t, server, sessions, ownershipBob, "/api/books"); got.Code != http.StatusOK {
		t.Fatalf("other reader blocked: %s", got.Body.String())
	}
	home, err := readers.Open(t.Context(), alice)
	if err != nil {
		t.Fatal(err)
	}
	store := txtstore.NewStore(home.DB(), home.Files())
	pending, err := store.Receive(t.Context(), rand.Text(), "Pending.txt", strings.NewReader("Chapter 1\nOriginal bytes.\n"))
	if err != nil {
		t.Fatal(err)
	}
	removing, err := store.Receive(t.Context(), rand.Text(), "Removing.txt", strings.NewReader("Obsolete bytes."))
	if err != nil {
		t.Fatal(err)
	}
	// Model a snapshot taken during analysis and a retryable removal. An
	// unexpected file must survive; it makes cleanup report an honest warning.
	if _, err := home.DB().Exec(`UPDATE txt_interpretations SET state=? WHERE file_id=? AND role='candidate'`, txtstore.Analyzing, pending.ID); err != nil {
		t.Fatal(err)
	}
	root, err := home.Files().OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(filepath.Dir(removing.Path), "keep.txt")
	if err := root.WriteFile(extra, []byte("not owned by TXT cleanup"), 0600); err != nil {
		t.Fatal(err)
	}
	root.Close()
	if pending, err := store.DiscardPending(t.Context(), removing.ID); err == nil || !pending {
		t.Fatalf("expected retryable cleanup: %v %v", pending, err)
	}
	if err := book.NewStore(home.DB()).AddBook(&book.Book{ID: "restored-book", Name: "Restored", SourceURL: "synthetic", BookURL: "/book"}); err != nil {
		t.Fatal(err)
	}
	home.Close()

	var archive bytes.Buffer
	if _, err := server.backups.Export(t.Context(), alice, "Alice", time.Now(), &archive); err != nil {
		t.Fatal(err)
	}
	prepared, err := server.backups.PrepareRestore(t.Context(), ownershipBob, &archive)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := sessions.Create(t.Context(), ownershipBob, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/backups/restores/"+prepared.ID+"/commit", nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: credential.Token})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	var result backup.RestoreResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || !result.Restored || len(result.Warnings) != 1 || result.Warnings[0] != "txt_recovery_incomplete" {
		t.Fatalf("status=%d result=%+v", response.Code, result)
	}
	// Old intake authority is invalidated; fresh admission and existing readers
	// resume despite the warning, using new data.
	if _, err := server.fileAdmission.Status(ownershipBob, oldTicket.ID); !errors.Is(err, fileimport.ErrTicketNotFound) {
		t.Fatalf("pre-restore admission survived: %v", err)
	}
	if _, err := server.fileAdmission.Request(ownershipBob); err != nil {
		t.Fatal(err)
	}
	if err := server.fileImports.Notify(ownershipBob); err != nil {
		t.Fatal(err)
	}
	if got := authenticatedOwnershipRequest(t, server, sessions, ownershipBob, "/api/books/restored-book"); got.Code != http.StatusOK {
		t.Fatalf("restored runtime: %s", got.Body.String())
	}
	if err := server.quiesceReader(t.Context(), ownershipBob); err != nil {
		t.Fatal(err)
	}
	home, err = readers.Open(t.Context(), ownershipBob)
	if err != nil {
		t.Fatal(err)
	}
	store = txtstore.NewStore(home.DB(), home.Files())
	value, err := store.Get(t.Context(), pending.ID)
	if err != nil || value.State == txtstore.Analyzing {
		t.Fatalf("stale analysis was not recovered: %+v %v", value, err)
	}
	value, err = store.Get(t.Context(), removing.ID)
	if err != nil || value.State != txtstore.Removing {
		t.Fatalf("retryable removal lost: %+v %v", value, err)
	}
	root, err = home.Files().OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat(extra); err != nil {
		t.Fatalf("unowned bytes lost: %v", err)
	}
	root.Close()
	home.Close()
	if err := readers.Remove(t.Context(), ownershipBob); err != nil {
		t.Fatal(err)
	}
	if err := server.forgetReader(ownershipBob); err != nil {
		t.Fatal(err)
	}
	server.runtimes.mu.Lock()
	barrier := server.runtimes.deleting[ownershipBob]
	server.runtimes.mu.Unlock()
	if barrier {
		t.Fatal("deleted reader retained an API lifecycle barrier")
	}
}

type failingConversionClose struct{ chineseconv.Service }

func (failingConversionClose) Close() error { return errors.New("conversion cleanup failed") }

func TestServerCloseDrainsWorkersAndRuntimesAfterOtherCleanupFails(t *testing.T) {
	server, _, _, alice, closeStores := newOwnershipServer(t)
	defer closeStores()
	server.services.chineseConversion = failingConversionClose{}
	proof, _, err := server.services.fileInbox.retain(alice, inboxReview{txt: &txtstore.InboxReview{Claim: txtstore.InboxClaim{ReceiptID: "closed-proof"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err == nil || !strings.Contains(err.Error(), "conversion cleanup failed") {
		t.Fatalf("cleanup error lost: %v", err)
	}
	if _, err := server.services.fileInbox.take(alice, proof, "txt"); !errors.Is(err, errInboxProofMissing) {
		t.Fatal("shutdown retained inbox approval")
	}
	if err := server.fileImports.Notify(alice); !errors.Is(err, fileimport.ErrClosed) {
		t.Fatalf("workers still admitted work: %v", err)
	}
	if _, err := server.fileAdmission.Request(alice); !errors.Is(err, fileimport.ErrClosed) {
		t.Fatalf("intake still admitted work: %v", err)
	}
	if err := server.services.chapterResources.Cleanup(t.Context()); err == nil {
		t.Fatal("shared chapter resource store remained open after cleanup error")
	}
	if _, release, err := server.runtimes.acquire(t.Context(), alice); err == nil {
		release()
		t.Fatal("API runtimes remained open after cleanup error")
	}
}
