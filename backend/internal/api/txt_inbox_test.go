package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/txtstore"
)

type inboxReviewResponse struct {
	Token     string `json:"token"`
	CanRemove bool   `json:"canRemove"`
}

func TestTXTInboxAcquisitionAndExplicitLeftoverResolution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, readers, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		home, err := readers.Open(t.Context(), alice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		inbox, err := home.Files().OpenInbox()
		if err != nil {
			t.Fatal(err)
		}
		defer inbox.Close()
		const text = "Chapter 1\nFirst synthetic paragraph, not a real book.\nChapter 2\nSecond synthetic paragraph.\n"
		if err := inbox.WriteFile("example.txt", []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := inbox.WriteFile("copy.txt.part", []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		scan := txtImportRequest(t, server, sessions, alice, http.MethodGet, "/api/imports/txt/inbox?limit=1", nil)
		requireTXTStatus(t, scan, http.StatusOK)
		if !strings.Contains(scan.Body.String(), "example.txt") || strings.Contains(scan.Body.String(), "copy.txt.part") {
			t.Fatalf("scan: %s", scan.Body.String())
		}
		acquire := func() (string, int, string) {
			ticket, err := server.fileAdmission.Request(alice)
			if err != nil {
				t.Fatal(err)
			}
			response := txtImportRequest(t, server, sessions, alice, http.MethodPost, "/api/imports/txt/inbox/acquisitions/"+ticket.ID+"?filename=example.txt", nil)
			return ticket.ID, response.Code, response.Body.String()
		}
		review := func(id string) inboxReviewResponse {
			response := txtImportRequest(t, server, sessions, alice, http.MethodPost, "/api/imports/txt/inbox/claims/"+id+"/review", nil)
			requireTXTStatus(t, response, http.StatusOK)
			var result inboxReviewResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			return result
		}
		// The original can be safely acquired even when journal cleanup fails.
		if _, err := home.DB().Exec(`CREATE TRIGGER block_inbox_cleanup BEFORE DELETE ON txt_inbox_claims BEGIN SELECT RAISE(FAIL, 'synthetic cleanup failure'); END`); err != nil {
			t.Fatal(err)
		}
		id, status, body := acquire()
		if status != http.StatusCreated || !strings.Contains(body, "txt_inbox_cleanup_pending") {
			t.Fatalf("acquire: %d %s", status, body)
		}
		if _, err := home.DB().Exec(`DROP TRIGGER block_inbox_cleanup`); err != nil {
			t.Fatal(err)
		}
		pending := review(id)
		if pending.CanRemove {
			t.Fatal("already-moved inbox file reported removable")
		}
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, "/api/imports/txt/inbox/reviews/"+pending.Token+"/release", nil), http.StatusNoContent)
		synctest.Wait()
		store := txtstore.NewStore(home.DB(), home.Files())
		value, err := store.Get(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		accept := fmt.Sprintf(`{"analysisVersion":%d,"name":"Synthetic"}`, value.AnalysisVersion)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, "/api/imports/txt/receipts/"+id+"/accept", strings.NewReader(accept)), http.StatusOK)
		if _, err := inbox.Stat("example.txt"); !os.IsNotExist(err) {
			t.Fatalf("input not consumed: %v", err)
		}
		// A retained claim may survive independently of a publication/receipt.
		if err := inbox.WriteFile("example.txt", []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := home.DB().Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, "example.txt", id); err != nil {
			t.Fatal(err)
		}
		_, status, body = acquire()
		if status != http.StatusConflict || !strings.Contains(body, id) {
			t.Fatalf("silent reimport: %d %s", status, body)
		}
		proof := review(id)
		if !proof.CanRemove {
			t.Fatal("duplicate not recognized")
		}
		confirm := "/api/imports/txt/inbox/reviews/" + proof.Token + "/confirm"
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodPost, confirm, nil), http.StatusConflict)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, confirm, nil), http.StatusNoContent)
		if _, err := inbox.Stat("example.txt"); !os.IsNotExist(err) {
			t.Fatalf("duplicate retained: %v", err)
		}
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodGet, "/api/books/"+id, nil), http.StatusOK)
		if err := inbox.WriteFile("example.txt", []byte("a different input"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := home.DB().Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, "example.txt", id); err != nil {
			t.Fatal(err)
		}
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, confirm, nil), http.StatusConflict)
		proof = review(id)
		if proof.CanRemove {
			t.Fatal("unique input marked removable")
		}
		release := "/api/imports/txt/inbox/reviews/" + proof.Token + "/release"
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, release, nil), http.StatusNoContent)
		if _, err := inbox.Stat("example.txt"); err != nil {
			t.Fatal("release consumed input", err)
		}
		fresh, status, body := acquire()
		if status != http.StatusCreated || fresh == id {
			t.Fatalf("fresh acquisition: %d %s", status, body)
		}
	})
}
