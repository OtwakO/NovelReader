package readerstore

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerSnapshotAndReplacementArePortableAcrossAccounts(t *testing.T) {
	manager := newBackupTestManager(t)
	ctx := context.Background()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(ctx, testUserBob); err != nil {
		t.Fatal(err)
	}
	alice, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alice.DB().Exec(`INSERT INTO portable_value VALUES ('alice data')`); err != nil {
		t.Fatal(err)
	}
	if err := alice.Files().WriteFile([]byte("font bytes"), 0o600, FontsDirectory, "portable-font"); err != nil {
		t.Fatal(err)
	}
	if err := alice.Close(); err != nil {
		t.Fatal(err)
	}

	snapshotHome := filepath.Join(t.TempDir(), "reader-home")
	if err := manager.SnapshotHome(ctx, testUserAlice, snapshotHome); err != nil {
		t.Fatal(err)
	}
	staging, err := manager.PrepareReplacement(ctx, testUserBob, filepath.Join(snapshotHome, ReaderDatabaseName), filepath.Join(snapshotHome, FilesDirectory))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := readHomeManifest(filepath.Join(staging, HomeManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Format != HomeFormat || manifest.Version != CurrentHomeVersion {
		t.Fatalf("manifest=%#v", manifest)
	}
	if err := manager.PublishReplacement(ctx, testUserBob, staging); err != nil {
		t.Fatal(err)
	}

	bob, err := manager.Open(ctx, testUserBob)
	if err != nil {
		t.Fatal(err)
	}
	defer bob.Close()
	var value string
	if err := bob.DB().QueryRow(`SELECT value FROM portable_value`).Scan(&value); err != nil || value != "alice data" {
		t.Fatalf("value=%q error=%v", value, err)
	}
	font, err := bob.Files().ReadFile(FontsDirectory, "portable-font")
	if err != nil || string(font) != "font bytes" {
		t.Fatalf("font=%q error=%v", font, err)
	}
}

func TestManagerPublishReplacementRollsBackInvalidStage(t *testing.T) {
	manager := newBackupTestManager(t)
	ctx := context.Background()
	if err := manager.Create(ctx, testUserBob); err != nil {
		t.Fatal(err)
	}
	bob, err := manager.Open(ctx, testUserBob)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bob.DB().Exec(`INSERT INTO retained_value VALUES ('keep')`); err != nil {
		t.Fatal(err)
	}
	if err := bob.Close(); err != nil {
		t.Fatal(err)
	}

	homePath := filepath.Join(manager.root, UsersDirectory, string(testUserBob))
	stagingPath := homePath + backupStagingSuffix
	if err := os.Mkdir(stagingPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := manager.PublishReplacement(ctx, testUserBob, stagingPath); err == nil {
		t.Fatal("invalid replacement succeeded")
	}
	if _, err := os.Stat(homePath + backupRollbackSuffix); !os.IsNotExist(err) {
		t.Fatalf("rollback remains: %v", err)
	}
	assertDatabaseValue(t, filepath.Join(homePath, ReaderDatabaseName), `SELECT value FROM retained_value`, "keep")
}

func newBackupTestManager(t *testing.T) *Manager {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	manager, err := NewManager(root, 2, ReaderSchema{Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE portable_value (value TEXT NOT NULL); CREATE TABLE retained_value (value TEXT NOT NULL)`)
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("close manager: %v", err)
		}
	})
	return manager
}

func assertDatabaseValue(t *testing.T, path, query, want string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var got string
	if err := db.QueryRow(query).Scan(&got); err != nil || got != want {
		t.Fatalf("value=%q want=%q error=%v", got, want, err)
	}
}
