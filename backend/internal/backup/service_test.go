package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	backupAlice readerstore.UserID = "11111111-1111-4111-8111-111111111111"
	backupBob   readerstore.UserID = "22222222-2222-4222-8222-222222222222"
)

func TestServiceExportsTimestampedPortableArchiveAndRestoresAcrossReaders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	readers, err := readerstore.NewManager(root, 2, readerstore.ReaderSchema{Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE values_table (value TEXT NOT NULL)`)
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	for _, id := range []readerstore.UserID{backupAlice, backupBob} {
		if err := readers.Create(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	alice, _ := readers.Open(context.Background(), backupAlice)
	if _, err := alice.DB().Exec(`INSERT INTO values_table VALUES ('alice')`); err != nil {
		t.Fatal(err)
	}
	alice.Close()
	service, err := NewService(readers, root, func(context.Context, readerstore.UserID) error { return nil }, func(readerstore.UserID) {})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.now = func() time.Time { return time.Date(2026, 8, 29, 21, 45, 30, 0, time.FixedZone("UTC+8", 8*60*60)) }

	var archive bytes.Buffer
	info, err := service.Export(context.Background(), backupAlice, "Alice / 測試", service.now(), &archive)
	if err != nil {
		t.Fatal(err)
	}
	if info.Filename != "novelreader-Alice-測試-backup-20260829-214530+0800.tar.gz" {
		t.Fatalf("filename=%q", info.Filename)
	}
	prepared, err := service.PrepareRestore(context.Background(), backupBob, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ExportedFromUsername != "Alice / 測試" || prepared.Compatibility != "compatible" {
		t.Fatalf("prepared=%#v", prepared)
	}
	if _, err := service.CommitRestore(context.Background(), backupBob, prepared.ID); err != nil {
		t.Fatal(err)
	}
	bob, err := readers.Open(context.Background(), backupBob)
	if err != nil {
		t.Fatal(err)
	}
	defer bob.Close()
	var value string
	if err := bob.DB().QueryRow(`SELECT value FROM values_table`).Scan(&value); err != nil || value != "alice" {
		t.Fatalf("value=%q error=%v", value, err)
	}
}

func TestExtractArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _, entry := range []tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg, Size: 1},
		{Name: PayloadRoot + "/link", Typeflag: tar.TypeSymlink, Linkname: "/tmp"},
	} {
		t.Run(strings.ReplaceAll(entry.Name, "/", "_"), func(t *testing.T) {
			var archive bytes.Buffer
			gzipWriter := gzip.NewWriter(&archive)
			tarWriter := tar.NewWriter(gzipWriter)
			manifest, _ := json.Marshal(NewManifest("alice", time.Now()))
			_ = writeTarBytes(tarWriter, ManifestPath, manifest, 0o600, time.Now())
			if err := tarWriter.WriteHeader(&entry); err != nil {
				t.Fatal(err)
			}
			if entry.Size > 0 {
				_, _ = tarWriter.Write([]byte("x"))
			}
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			if _, _, err := extractArchive(context.Background(), bytes.NewReader(archive.Bytes()), t.TempDir()); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestFilenameFallsBackForUnsafeUsername(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := Filename(" ../../ ", createdAt); got != "novelreader-reader-backup-20260102-030405+0000.tar.gz" {
		t.Fatalf("filename=%q", got)
	}
}

func TestPreparedRestoreIsOwnerScopedAndCancelable(t *testing.T) {
	root := t.TempDir()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	for _, id := range []readerstore.UserID{backupAlice, backupBob} {
		if err := readers.Create(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	service, err := NewService(readers, root, func(context.Context, readerstore.UserID) error { return nil }, func(readerstore.UserID) {})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	var archive bytes.Buffer
	if _, err := service.Export(context.Background(), backupAlice, "alice", service.now(), &archive); err != nil {
		t.Fatal(err)
	}
	prepared, err := service.PrepareRestore(context.Background(), backupAlice, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetRestore(backupBob, prepared.ID); err == nil {
		t.Fatal("other reader accessed restore")
	}
	if err := service.CancelRestore(backupAlice, prepared.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "users", string(backupAlice)+".restore-staging")); !os.IsNotExist(err) {
		t.Fatalf("restore staging remains: %v", err)
	}
}

func TestServiceJanitorRemovesExpiredRestoreWithoutAPITraffic(t *testing.T) {
	root := t.TempDir()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if err := readers.Create(context.Background(), backupAlice); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time)
	service, err := newService(readers, root, func(context.Context, readerstore.UserID) error { return nil }, func(readerstore.UserID) {}, ticks)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	now := time.Date(2026, 8, 29, 22, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	var archive bytes.Buffer
	if _, err := service.Export(context.Background(), backupAlice, "alice", now, &archive); err != nil {
		t.Fatal(err)
	}
	prepared, err := service.PrepareRestore(context.Background(), backupAlice, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(root, "users", string(backupAlice)+".restore-staging")
	now = now.Add(restoreLifetime)
	ticks <- now
	for range 100 {
		if _, err := os.Stat(staging); os.IsNotExist(err) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("expired staging remains: %v", err)
	}
	if _, err := service.GetRestore(backupAlice, prepared.ID); !errors.Is(err, ErrRestoreNotFound) {
		t.Fatalf("expired restore error=%v", err)
	}
}

func TestServiceStartupRemovesOnlyOwnedAbandonedWorkspaces(t *testing.T) {
	root := t.TempDir()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	for _, name := range []string{exportWorkspacePrefix + "old", restoreWorkspacePrefix + "old", "keep-me"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	service, err := NewService(readers, root, func(context.Context, readerstore.UserID) error { return nil }, func(readerstore.UserID) {})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for _, name := range []string{exportWorkspacePrefix + "old", restoreWorkspacePrefix + "old"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("workspace %q remains: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "keep-me")); err != nil {
		t.Fatalf("unowned directory removed: %v", err)
	}
}
