package readerstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeGenerationSurvivesRestartAndChangesOnRepeatedRestore(t *testing.T) {
	manager := newBackupTestManager(t)
	ctx := t.Context()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	original := home.Generation()
	if original == "" {
		t.Fatal("missing home generation")
	}
	home.Close()
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(ctx, testUserAlice, snapshot); err != nil {
		t.Fatal(err)
	}
	manifest, err := readHomeManifest(filepath.Join(snapshot, HomeManifestName))
	if err != nil || manifest.Generation != "" {
		t.Fatalf("snapshot carried identity: %+v, %v", manifest, err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewManager(manager.root, 1, manager.schemas...)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	generation := func() string {
		t.Helper()
		home, err := reopened.Open(ctx, testUserAlice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		return home.Generation()
	}
	if generation() != original {
		t.Fatal("restart changed identity")
	}
	if err := reopened.Create(ctx, testUserBob); err != nil {
		t.Fatal(err)
	}
	bob, err := reopened.Open(ctx, testUserBob) // evict Alice's idle entry
	if err != nil {
		t.Fatal(err)
	}
	bob.Close()
	if generation() != original {
		t.Fatal("runtime eviction changed identity")
	}
	previous := original
	for range 2 {
		stage, err := reopened.PrepareReplacement(ctx, testUserAlice, filepath.Join(snapshot, ReaderDatabaseName), filepath.Join(snapshot, FilesDirectory))
		if err != nil {
			t.Fatal(err)
		}
		if generation() != previous {
			t.Fatal("staging changed live identity")
		}
		if err := reopened.PublishReplacement(ctx, testUserAlice, stage); err != nil {
			t.Fatal(err)
		}
		current := generation()
		if current == "" || current == previous || current == original {
			t.Fatal("replacement reused an identity")
		}
		previous = current
	}
}

func TestCompatibleManifestInitializesIdentityButRejectsCorruptGeneration(t *testing.T) {
	manager := newBackupTestManager(t)
	if err := manager.Create(t.Context(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	path, err := manager.homePath(testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	// Both legacy homes and manually copied portable manifests omit generation.
	if err := writePortableHomeManifest(path); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	generation := home.Generation()
	home.Close()
	manifest, err := readHomeManifest(filepath.Join(path, HomeManifestName))
	if err != nil || generation == "" || manifest.Generation != generation {
		t.Fatalf("identity was not persisted: %+v, %v", manifest, err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, HomeManifestName), []byte(`{"format":"novelreader-reader-home","version":1,"generation":"broken"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readHomeManifest(filepath.Join(path, HomeManifestName)); err == nil {
		t.Fatal("invalid generation accepted")
	}
}
