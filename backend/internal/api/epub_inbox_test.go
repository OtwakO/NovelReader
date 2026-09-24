package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/epubstore"
)

func TestEPUBInboxImportAndCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		home, err := server.runtimes.readers.Open(t.Context(), alice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		inbox, err := home.Files().OpenInbox()
		if err != nil {
			t.Fatal(err)
		}
		defer inbox.Close()
		archive := readingEPUBBytes(t)
		for _, name := range []string{"Book.epub", "Text.txt"} {
			if err := inbox.WriteFile(name, archive, 0600); err != nil {
				t.Fatal(err)
			}
		}
		request := func(method, path string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, nil)
		}
		requireTXTStatus(t, request(http.MethodGet, "/api/imports/epub/inbox?limit=101"), http.StatusBadRequest)
		scan := request(http.MethodGet, "/api/imports/epub/inbox")
		requireTXTStatus(t, scan, http.StatusOK)
		if !strings.Contains(scan.Body.String(), "Book.epub") || strings.Contains(scan.Body.String(), "Text.txt") {
			t.Fatal(scan.Body.String())
		}
		admitted := request(http.MethodPost, "/api/imports/admission")
		requireTXTStatus(t, admitted, http.StatusOK)
		var ticket struct{ ID string }
		if err := json.Unmarshal(admitted.Body.Bytes(), &ticket); err != nil {
			t.Fatal(err)
		}
		path := "/api/imports/epub/inbox/acquisitions/" + ticket.ID + "?filename=Book.epub&imageMode=optimized"
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodPost, path, nil), http.StatusNotFound)
		requireTXTStatus(t, request(http.MethodPost, path+"-invalid"), http.StatusBadRequest)
		acquired := request(http.MethodPost, path)
		requireTXTStatus(t, acquired, http.StatusCreated)
		if acquired.Header().Get("Location") != "/api/imports/epub/receipts/"+ticket.ID {
			t.Fatal(acquired.Header())
		}
		synctest.Wait()
		response := request(http.MethodGet, acquired.Header().Get("Location"))
		requireTXTStatus(t, response, http.StatusOK)
		var receipt epubReceiptResponse
		if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.PreparationState != epubstore.PreparationReady || receipt.ImageMode != "optimized" || receipt.Generation != 1 {
			t.Fatalf("receipt=%+v", receipt)
		}
		if _, err := inbox.Stat("Book.epub"); !os.IsNotExist(err) {
			t.Fatal("source not moved", err)
		}
		// Reconstruct a cross-device leftover. Only a reader/format-bound server proof
		// may remove it; another reader or a TXT route cannot consume that proof.
		if err := inbox.WriteFile("Book.epub", archive, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := home.DB().Exec(`INSERT INTO epub_inbox_claims(name,receipt_id) VALUES(?,?)`, "Book.epub", ticket.ID); err != nil {
			t.Fatal(err)
		}
		review := request(http.MethodPost, "/api/imports/epub/inbox/claims/"+ticket.ID+"/review")
		requireTXTStatus(t, review, http.StatusOK)
		var proof struct {
			Token     string
			CanRemove bool
		}
		if err := json.Unmarshal(review.Body.Bytes(), &proof); err != nil {
			t.Fatal(err)
		}
		if !proof.CanRemove || proof.Token == "" {
			t.Fatal(review.Body.String())
		}
		for _, field := range []string{"digest", "owner", "sha256", "original.epub"} {
			if strings.Contains(review.Body.String(), field) {
				t.Fatal("private evidence exposed", field)
			}
		}
		route := "/api/imports/epub/inbox/reviews/" + proof.Token + "/confirm"
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodPost, route, nil), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodPost, strings.Replace(route, "/epub/", "/txt/", 1)), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodPost, route), http.StatusNoContent)
		requireTXTStatus(t, request(http.MethodPost, route), http.StatusConflict)
		if _, err := inbox.Stat("Book.epub"); !os.IsNotExist(err) {
			t.Fatal("duplicate not removed", err)
		}
	})
}

func TestEPUBInboxReviewSettlesOnlyInactiveClaim(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		home, err := server.runtimes.readers.Open(t.Context(), alice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		admission, err := server.fileAdmission.Request(alice)
		if err != nil {
			t.Fatal(err)
		}
		ticket := admission.ID
		ctx, release, err := server.fileAdmission.Begin(t.Context(), alice, ticket)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		if _, err := home.DB().Exec(`INSERT INTO epub_files(id,original_name,state,size,image_mode,created_at,updated_at) VALUES(?,'Interrupted.epub','finalizing',4,'original',1,1); INSERT INTO epub_inbox_claims(name,receipt_id) VALUES('Interrupted.epub',?)`, ticket, ticket); err != nil {
			t.Fatal(err)
		}
		route := "/api/imports/epub/inbox/claims/" + ticket + "/review"
		request := func(method, path string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, nil)
		}
		requireTXTStatus(t, request(http.MethodPost, route), http.StatusConflict)
		if ctx.Err() != nil {
			t.Fatal("review cancelled active acquisition")
		}
		store := epubstore.NewStore(home.DB(), home.Files())
		value, err := store.Get(t.Context(), ticket)
		if err != nil || value.State != epubstore.Finalizing {
			t.Fatalf("active=%+v %v", value, err)
		}
		release()
		review := request(http.MethodPost, route)
		requireTXTStatus(t, review, http.StatusOK)
		value, err = store.Get(t.Context(), ticket)
		if err != nil || value.State != epubstore.Failed {
			t.Fatalf("settled=%+v %v", value, err)
		}
		var proof struct{ Token string }
		if err := json.Unmarshal(review.Body.Bytes(), &proof); err != nil {
			t.Fatal(err)
		}
		requireTXTStatus(t, request(http.MethodDelete, "/api/imports/epub/inbox/reviews/"+proof.Token), http.StatusNoContent)
		requireTXTStatus(t, request(http.MethodPost, "/api/imports/epub/inbox/reviews/"+proof.Token+"/release"), http.StatusConflict)
		review = request(http.MethodPost, route)
		requireTXTStatus(t, review, http.StatusOK)
		if err := json.Unmarshal(review.Body.Bytes(), &proof); err != nil {
			t.Fatal(err)
		}
		server.services.fileInbox.invalidate(alice)
		requireTXTStatus(t, request(http.MethodPost, fmt.Sprintf("/api/imports/epub/inbox/reviews/%s/release", proof.Token)), http.StatusConflict)
		if _, err := store.GetInboxClaim(t.Context(), ticket); err != nil {
			t.Fatal("invalidation removed durable claim", err)
		}
	})
}
