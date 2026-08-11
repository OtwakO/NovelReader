// Graceful shutdown coverage for container stop signals.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/readerstore"
	_ "modernc.org/sqlite"
)

func TestOpenStoresGatesLegacyRootBeforeCreatingSystemStore(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(legacyPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	systemStore, readers, err := openStores(root)
	if systemStore != nil {
		systemStore.Close()
	}
	if readers != nil {
		readers.Close()
	}
	if !errors.Is(err, readerstore.ErrLegacyRoot) {
		t.Fatalf("err = %v", err)
	}
	for _, instruction := range []string{"remove or rename DATA_DIR", "re-import test BookSources"} {
		if !strings.Contains(err.Error(), instruction) {
			t.Errorf("error %q does not contain %q", err, instruction)
		}
	}
}

func TestOpenStoresInitializesEmptyRootWithoutLegacyDatabase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	systemStore, readers, err := openStores(root)
	if err != nil {
		t.Fatal(err)
	}
	defer systemStore.Close()
	defer readers.Close()
	for _, path := range []string{readerstore.RootManifestName, auth.SystemDatabaseName} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "novelreader.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy database exists: %v", err)
	}
}

func TestOpenStoresCreatesCompleteCurrentReaderSchema(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	systemStore, readers, err := openStores(root)
	if err != nil {
		t.Fatal(err)
	}
	defer systemStore.Close()
	defer readers.Close()
	userID := readerstore.UserID("11111111-1111-4111-8111-111111111111")
	if err := readers.Create(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	home, err := readers.Open(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()

	var version int
	if err := home.DB().QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != readerstore.CurrentReaderSchemaVersion {
		t.Fatalf("reader schema version=%d", version)
	}
	for _, table := range []string{"book_sources", "books", "chapters", "bookmarks", "chapter_cache", "fonts"} {
		var count int
		if err := home.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s count=%d error=%v", table, count, err)
		}
	}
	var sourceJSONColumns int
	if err := home.DB().QueryRow(`SELECT count(*) FROM pragma_table_info('book_sources') WHERE name='source_json'`).Scan(&sourceJSONColumns); err != nil || sourceJSONColumns != 1 {
		t.Fatalf("source_json columns=%d error=%v", sourceJSONColumns, err)
	}
	var historyTables int
	if err := home.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='readerstore_migrations'`).Scan(&historyTables); err != nil || historyTables != 0 {
		t.Fatalf("schema history tables=%d error=%v", historyTables, err)
	}
}

func TestOpenStoresExcludesSecondServerForSameDataRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	firstStore, firstReaders, err := openStores(root)
	if err != nil {
		t.Fatal(err)
	}
	defer firstStore.Close()
	defer firstReaders.Close()
	secondStore, secondReaders, err := openStores(root)
	if secondStore != nil {
		secondStore.Close()
	}
	if secondReaders != nil {
		secondReaders.Close()
	}
	if !errors.Is(err, readerstore.ErrRootInUse) {
		t.Fatalf("second open error=%v", err)
	}
}

func TestOpenStoresRejectsNewerSystemSchema(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if _, err := readerstore.PrepareRoot(root); err != nil {
		t.Fatal(err)
	}
	systemPath := filepath.Join(root, auth.SystemDatabaseName)
	systemDB, err := sql.Open("sqlite", systemPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := systemDB.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, auth.CurrentSystemSchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	if err := systemDB.Close(); err != nil {
		t.Fatal(err)
	}
	systemStore, readers, err := openStores(root)
	if systemStore != nil {
		systemStore.Close()
	}
	if readers != nil {
		readers.Close()
	}
	if !errors.Is(err, auth.ErrNewerSystemSchema) {
		t.Fatalf("error = %v", err)
	}
}

func TestCredentialOriginMiddlewareAllowsOnlyConfiguredOrigin(t *testing.T) {
	nextCalls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	})
	handler, err := credentialOriginMiddleware("https://reader.example", next)
	if err != nil {
		t.Fatal(err)
	}

	allowed := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	allowed.Header.Set("Origin", "https://reader.example")
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://reader.example" || allowedResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("allowed response status=%d headers=%v", allowedResponse.Code, allowedResponse.Header())
	}

	missing := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusForbidden || nextCalls != 1 {
		t.Fatalf("missing-origin response status=%d calls=%d", missingResponse.Code, nextCalls)
	}

	foreign := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	foreign.Header.Set("Origin", "https://evil.example")
	foreignResponse := httptest.NewRecorder()
	handler.ServeHTTP(foreignResponse, foreign)
	if foreignResponse.Code != http.StatusForbidden || nextCalls != 1 {
		t.Fatalf("foreign response status=%d calls=%d", foreignResponse.Code, nextCalls)
	}

	dynamic, err := credentialOriginMiddleware("", next)
	if err != nil {
		t.Fatal(err)
	}
	dynamicRequest := httptest.NewRequest(http.MethodPost, "http://reader.example-tailnet.ts.net/api/auth/login", nil)
	dynamicRequest.Header.Set("Origin", "https://reader.example-tailnet.ts.net")
	dynamicResponse := httptest.NewRecorder()
	dynamic.ServeHTTP(dynamicResponse, dynamicRequest)
	if dynamicResponse.Code != http.StatusNoContent || dynamicResponse.Header().Get("Access-Control-Allow-Origin") != "https://reader.example-tailnet.ts.net" {
		t.Fatalf("dynamic proxy origin status=%d headers=%v", dynamicResponse.Code, dynamicResponse.Header())
	}
	dynamicForeign := httptest.NewRequest(http.MethodPost, "http://reader.example-tailnet.ts.net/api/auth/login", nil)
	dynamicForeign.Header.Set("Origin", "https://evil.example")
	dynamicForeignResponse := httptest.NewRecorder()
	dynamic.ServeHTTP(dynamicForeignResponse, dynamicForeign)
	if dynamicForeignResponse.Code != http.StatusForbidden {
		t.Fatalf("dynamic foreign status=%d", dynamicForeignResponse.Code)
	}
}

func TestServeWaitsForInflightRequestDuringShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte("ok"))
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener) }()

	responseDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			response.Body.Close()
		}
		responseDone <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("server stopped before request completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-responseDone; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
