package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/txtstore"
)

func TestTXTDiscardInterruptsBlockedHTTPUploadBeforeRemovingReceipt(t *testing.T) {
	server, sessions, _, alice, cleanup := newOwnershipServer(t)
	defer cleanup()
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	session, err := sessions.Create(t.Context(), alice, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := server.fileAdmission.Request(alice)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", httpServer.Listener.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(connection, "PUT /api/imports/txt/uploads/%s?filename=blocked.txt HTTP/1.1\r\nHost: %s\r\nCookie: %s=%s\r\nContent-Type: application/octet-stream\r\nContent-Length: 1024\r\nExpect: 100-continue\r\n\r\n", ticket.ID, httpServer.Listener.Addr(), auth.SessionCookieName, session.Token)
	if err != nil {
		t.Fatal(err)
	}
	// Go sends 100 Continue only when the handler first reads the body, after the
	// durable receipt intent. Send no bytes: the real HTTP/1 Read is now blocked.
	continued, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodPut})
	if err != nil {
		t.Fatal(err)
	}
	if continued.StatusCode != http.StatusContinue {
		t.Fatalf("expected 100 Continue, got %d", continued.StatusCode)
	}
	continued.Body.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	discard, err := http.NewRequestWithContext(ctx, http.MethodDelete, httpServer.URL+"/api/imports/txt/receipts/"+ticket.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	discard.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	response, err := httpServer.Client().Do(discard)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("discard status: %d", response.StatusCode)
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "deleted" {
		t.Fatalf("discard: %+v", result)
	}
	// Discard waited for receive/failure cleanup; it cannot leave a new orphan
	// receipt or consume the reader's next grant when the old request unwinds.
	requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodGet, "/api/imports/txt/receipts/"+ticket.ID, nil), http.StatusNotFound)
	fresh, err := server.fileAdmission.Request(alice)
	if err != nil || fresh.ID == ticket.ID {
		t.Fatalf("fresh admission: %+v %v", fresh, err)
	}
	home, err := server.runtimes.readers.Open(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	items, err := txtstore.NewStore(home.DB(), home.Files()).List(ctx, "", "", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("late upload recreated receipt: %+v %v", items, err)
	}
}
