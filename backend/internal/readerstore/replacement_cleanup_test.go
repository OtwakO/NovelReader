package readerstore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplacementCleanupFailureStillReportsCommittedHome(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory write permissions")
	}
	manager := newBackupTestManager(t)
	if err := manager.Create(t.Context(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`INSERT INTO portable_value VALUES ('new')`); err != nil {
		t.Fatal(err)
	}
	home.Close()
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(t.Context(), testUserAlice, snapshot); err != nil {
		t.Fatal(err)
	}
	staging, err := manager.PrepareReplacement(t.Context(), testUserAlice, filepath.Join(snapshot, ReaderDatabaseName), filepath.Join(snapshot, FilesDirectory))
	if err != nil {
		t.Fatal(err)
	}
	home, err = manager.Open(t.Context(), testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := home.DB().Exec(`UPDATE portable_value SET value='old'`); err != nil {
		t.Fatal(err)
	}
	home.Close()
	homePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
	locked := filepath.Join(homePath, FilesDirectory, "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(locked, 0o700)
		_ = os.Chmod(filepath.Join(homePath+backupRollbackSuffix, FilesDirectory, "locked"), 0o700)
	})
	if err := manager.PublishReplacement(t.Context(), testUserAlice, staging); !errors.Is(err, ErrReplacementCleanupPending) {
		t.Fatalf("expected committed cleanup warning, got %v", err)
	}
	assertDatabaseValue(t, filepath.Join(homePath, ReaderDatabaseName), `SELECT value FROM portable_value`, "new")
}
