package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/txtstore"
)

func TestTXTInboxReviewRejectsActiveChangedAndInvalidatedApprovals(t *testing.T) {
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
	ticket, err := server.txtAdmission.Request(alice)
	if err != nil {
		t.Fatal(err)
	}
	ctx, finish, err := server.txtAdmission.Begin(t.Context(), alice, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	const text = "Original synthetic input"
	store := txtstore.NewStore(home.DB(), home.Files())
	value, err := store.Receive(ctx, ticket.ID, "leftover.txt", strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.WriteFile(value.OriginalName, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, value.OriginalName, value.ID); err != nil {
		t.Fatal(err)
	}
	reviewURL := "/api/imports/txt/inbox/claims/" + value.ID + "/review"
	requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, reviewURL, nil), http.StatusConflict)
	if ctx.Err() != nil {
		t.Fatal("review unexpectedly cancelled acquisition")
	}
	finish()
	// Model a finished but incompletely finalized receipt; review settles only it.
	if _, err := home.DB().Exec(`DELETE FROM txt_interpretations WHERE file_id=?`, value.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`UPDATE txt_files SET state=?,generation=0 WHERE id=?`, txtstore.Receiving, value.ID); err != nil {
		t.Fatal(err)
	}
	review := func() inboxReviewResponse {
		response := txtImportRequest(t, server, sessions, alice, http.MethodPost, reviewURL, nil)
		requireTXTStatus(t, response, http.StatusOK)
		var result inboxReviewResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	proof := review()
	before, err := inbox.Stat(value.OriginalName)
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.WriteFile(value.OriginalName, []byte("Modified synthetic input"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := inbox.Chtimes(value.OriginalName, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	confirm := func(token string) {
		response := txtImportRequest(t, server, sessions, alice, http.MethodPost, "/api/imports/txt/inbox/reviews/"+token+"/confirm", strings.NewReader(`{"canRemove":true}`))
		requireTXTStatus(t, response, http.StatusConflict)
	}
	confirm(proof.Token)
	proof = review()
	if proof.CanRemove {
		t.Fatal("changed input marked duplicate")
	}
	confirm(proof.Token) // Client-visible flags cannot fabricate approval.
	if content, err := inbox.ReadFile(value.OriginalName); err != nil || string(content) != "Modified synthetic input" {
		t.Fatalf("unique input lost: %q %v", content, err)
	}
	if err := inbox.WriteFile(value.OriginalName, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	proof = review()
	if err := server.quiesceReader(t.Context(), alice); err != nil {
		t.Fatal(err)
	}
	server.resumeReader(alice)
	confirm(proof.Token) // Draining invalidates even if the same DB remains open.
	first, err := server.services.txtInbox.beginIO()
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	second, err := server.services.txtInbox.beginIO()
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodGet, "/api/imports/txt/inbox", nil), http.StatusTooManyRequests)
	requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodGet, "/api/books", nil), http.StatusOK)
	requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPost, "/api/imports/txt/admission", nil), http.StatusOK)
}
