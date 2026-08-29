package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
)

const ownershipBob readerstore.UserID = "22222222-2222-4222-8222-222222222222"

func TestAuthenticatedServerDeniesAnonymousReaderDataAndIsolatesEqualIDs(t *testing.T) {
	server, sessions, readers, aliceID, closeStores := newOwnershipServer(t)
	defer closeStores()

	anonymous := httptest.NewRecorder()
	server.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/books", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d body=%s", anonymous.Code, anonymous.Body.String())
	}

	aliceHome, err := readers.Open(context.Background(), aliceID)
	if err != nil {
		t.Fatal(err)
	}
	aliceBook := book.NewStore(aliceHome.DB())
	if err := aliceBook.AddBook(&book.Book{ID: "same-id", Name: "Alice Book", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	aliceHome.Close()

	alice := authenticatedOwnershipRequest(t, server, sessions, aliceID, "/api/books/same-id")
	if alice.Code != http.StatusOK || !containsJSONName(alice.Body.Bytes(), "Alice Book") {
		t.Fatalf("alice status=%d body=%s", alice.Code, alice.Body.String())
	}
	bob := authenticatedOwnershipRequest(t, server, sessions, ownershipBob, "/api/books/same-id")
	if bob.Code != http.StatusNotFound {
		t.Fatalf("bob status=%d body=%s", bob.Code, bob.Body.String())
	}
}

func TestAuthenticatedServerExportsAndStagesReaderBackupWithoutRuntimeLease(t *testing.T) {
	server, sessions, _, aliceID, closeStores := newOwnershipServer(t)
	defer closeStores()
	credential, err := sessions.Create(context.Background(), aliceID, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	exportRequest := httptest.NewRequest(http.MethodGet, "/api/backups/export", nil)
	exportRequest.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: credential.Token})
	exportResponse := httptest.NewRecorder()
	server.ServeHTTP(exportResponse, exportRequest)
	if exportResponse.Code != http.StatusOK || exportResponse.Header().Get("Content-Type") != "application/gzip" || exportResponse.Body.Len() == 0 {
		t.Fatalf("export status=%d headers=%v bytes=%d", exportResponse.Code, exportResponse.Header(), exportResponse.Body.Len())
	}

	restoreRequest := httptest.NewRequest(http.MethodPost, "/api/backups/restores", bytes.NewReader(exportResponse.Body.Bytes()))
	restoreRequest.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: credential.Token})
	restoreResponse := httptest.NewRecorder()
	server.ServeHTTP(restoreResponse, restoreRequest)
	if restoreResponse.Code != http.StatusCreated {
		t.Fatalf("restore status=%d body=%s", restoreResponse.Code, restoreResponse.Body.String())
	}
	var prepared struct {
		OperationID   string `json:"operationId"`
		Compatibility string `json:"compatibility"`
	}
	if err := json.Unmarshal(restoreResponse.Body.Bytes(), &prepared); err != nil || prepared.OperationID == "" || prepared.Compatibility != "compatible" {
		t.Fatalf("prepared=%#v error=%v", prepared, err)
	}
	if _, release, err := server.runtimes.acquire(context.Background(), aliceID); err != nil {
		t.Fatalf("prepared restore blocked reader runtime: %v", err)
	} else {
		release()
	}
}

func newOwnershipServer(t *testing.T) (*Server, *auth.SessionService, *readerstore.Manager, readerstore.UserID, func()) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	if _, err := readerstore.PrepareRoot(root); err != nil {
		t.Fatal(err)
	}
	system, err := auth.OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 4,
		booksource.ReaderSchema(), book.ReaderSchema(), fontstore.ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	setup := auth.NewSetupService(system, readers, "bootstrap authority value")
	alice, err := setup.CreateInitialAdministrator(context.Background(), "bootstrap authority value", "alice", "alice password value", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := readers.Create(context.Background(), ownershipBob); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewAccountService(system).CreateReaderAccount(context.Background(), ownershipBob, "bob", "bob password value", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	authHandler, err := auth.NewHTTPHandler(system, auth.HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	limits := book.DefaultSearcherLimits()
	limits.ConcurrentGlobal = 1
	limits.ConcurrentPerSearch = 1
	limits.MaxSessions = 8
	limits.SessionTTL = time.Minute
	jsVM := analyzer.NewJSVMWithPoolSize(1)
	searcher := book.NewSearcherWithLimits(nil, jsVM, analyzer.NewCacheManager(), nil, nil, limits)
	server, err := NewAuthenticatedServer(authHandler, readers, root, searcher, jsVM, limits, processor.Config{}, system, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return server, auth.NewSessionService(system), readers, alice.ID, func() {
		server.Close()
		readers.Close()
		system.Close()
	}
}

func authenticatedOwnershipRequest(t *testing.T, server *Server, sessions *auth.SessionService, userID readerstore.UserID, path string) *httptest.ResponseRecorder {
	t.Helper()
	credential, err := sessions.Create(context.Background(), userID, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: credential.Token})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func containsJSONName(data []byte, want string) bool {
	var value map[string]any
	return json.Unmarshal(data, &value) == nil && value["name"] == want
}
