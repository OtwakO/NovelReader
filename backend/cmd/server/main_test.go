// Graceful shutdown coverage for container stop signals.
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestOpenDatabaseGatesLegacyRootBeforeCreatingDatabase(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy.txt")
	if err := os.WriteFile(legacyPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(root, "novelreader.db")

	db, err := openDatabase(root, databasePath)
	if db != nil {
		db.Close()
	}
	if !errors.Is(err, readerstore.ErrLegacyRoot) {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(databasePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("database unexpectedly created: %v", statErr)
	}
}

func TestOpenDatabaseRejectsPathOutsideDataRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "data")
	databasePath := filepath.Join(parent, "outside.db")

	db, err := openDatabase(root, databasePath)
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

func TestOpenDatabaseRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "database-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	databasePath := filepath.Join(link, "novelreader.db")

	db, err := openDatabase(root, databasePath)
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

func TestOpenDatabaseInitializesEmptyRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	databasePath := filepath.Join(root, "novelreader.db")

	db, err := openDatabase(root, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := os.Stat(filepath.Join(root, readerstore.RootManifestName)); err != nil {
		t.Fatal(err)
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
