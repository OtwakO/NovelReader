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

func TestOpenDatabasesGatesLegacyRootBeforeCreatingDatabase(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(legacyPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(root, "novelreader.db")

	systemStore, db, err := openDatabases(root, databasePath)
	if systemStore != nil {
		systemStore.Close()
	}
	if db != nil {
		db.Close()
	}
	if !errors.Is(err, readerstore.ErrLegacyRoot) {
		t.Fatalf("err = %v", err)
	}
	for _, instruction := range []string{"remove or rename DATA_DIR", "re-import test BookSources"} {
		if !strings.Contains(err.Error(), instruction) {
			t.Errorf("error %q does not contain %q", err, instruction)
		}
	}
	if _, statErr := os.Stat(databasePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("database unexpectedly created: %v", statErr)
	}
}

func TestOpenDatabasesRejectsPathOutsideDataRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "data")
	databasePath := filepath.Join(parent, "outside.db")

	systemStore, db, err := openDatabases(root, databasePath)
	if systemStore != nil {
		systemStore.Close()
	}
	if db != nil {
		db.Close()
	}
	if err == nil {
		t.Fatal("expected database path rejection")
	}
	if _, statErr := os.Stat(root); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data root unexpectedly created: %v", statErr)
	}
	if _, statErr := os.Stat(databasePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("database unexpectedly created: %v", statErr)
	}
}

func TestOpenDatabasesRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "database-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	databasePath := filepath.Join(link, "novelreader.db")

	systemStore, db, err := openDatabases(root, databasePath)
	if systemStore != nil {
		systemStore.Close()
	}
	if db != nil {
		db.Close()
	}
	if err == nil {
		t.Fatal("expected symlink escape rejection")
	}
	if _, statErr := os.Stat(databasePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("outside database unexpectedly created: %v", statErr)
	}
}

func TestOpenDatabasesInitializesEmptyRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	databasePath := filepath.Join(root, "novelreader.db")

	systemStore, db, err := openDatabases(root, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer systemStore.Close()
	defer db.Close()
	if _, err := os.Stat(filepath.Join(root, readerstore.RootManifestName)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, auth.SystemDatabaseName)); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDatabasesRejectsNewerSystemSchemaBeforeCreatingFeatureDatabase(t *testing.T) {
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
	featurePath := filepath.Join(root, "novelreader.db")

	systemStore, featureDB, err := openDatabases(root, featurePath)
	if systemStore != nil {
		systemStore.Close()
	}
	if featureDB != nil {
		featureDB.Close()
	}
	if !errors.Is(err, auth.ErrNewerSystemSchema) {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(featurePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("feature database unexpectedly created: %v", statErr)
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
