package readerstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPrepareRootInitializesEmptyDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")

	state, err := PrepareRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if state != RootCurrent {
		t.Fatalf("state = %q, want %q", state, RootCurrent)
	}

	manifest, err := ReadRootManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Format != RootFormat || manifest.Version != CurrentRootVersion {
		t.Fatalf("manifest = %#v", manifest)
	}
	if info, err := os.Stat(filepath.Join(root, UsersDirectory)); err != nil || !info.IsDir() {
		t.Fatalf("users directory: info=%v err=%v", info, err)
	}

	state, err = PrepareRoot(root)
	if err != nil || state != RootCurrent {
		t.Fatalf("second prepare: state=%q err=%v", state, err)
	}
}

func TestPrepareRootInitializesEmptyDirectoryConcurrently(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	const callers = 8
	var wait sync.WaitGroup
	errors := make(chan error, callers)

	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			state, err := PrepareRoot(root)
			if err != nil {
				errors <- err
				return
			}
			if state != RootCurrent {
				errors <- fmt.Errorf("state = %q", state)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if state, err := InspectRoot(root); err != nil || state != RootCurrent {
		t.Fatalf("final root: state=%q err=%v", state, err)
	}
}

func TestPrepareRootRefusesLegacyDataWithoutModification(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "novelreader.db")
	legacyData := []byte("legacy database placeholder")
	if err := os.WriteFile(legacyPath, legacyData, 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := PrepareRoot(root)
	if state != RootLegacy || !errors.Is(err, ErrLegacyRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
	got, readErr := os.ReadFile(legacyPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(legacyData) {
		t.Fatalf("legacy data changed: %q", got)
	}
	if _, statErr := os.Stat(filepath.Join(root, RootManifestName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("manifest unexpectedly created: %v", statErr)
	}
}

func TestInspectRootRejectsNewerVersionWithoutModification(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, RootManifestName)
	original := []byte(`{"format":"novelreader-data-root","version":2}` + "\n")
	if err := os.WriteFile(manifestPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := PrepareRoot(root)
	if state != RootNewer || !errors.Is(err, ErrNewerRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
	got, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("manifest changed: %q", got)
	}
}

func TestInspectRootReportsInterruptedInitialization(t *testing.T) {
	root := t.TempDir()
	stagingPath := filepath.Join(root, rootManifestStagingName)
	if err := os.WriteFile(stagingPath, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := PrepareRoot(root)
	if state != RootInterrupted || !errors.Is(err, ErrInterruptedRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
	got, readErr := os.ReadFile(stagingPath)
	if readErr != nil || string(got) != "partial" {
		t.Fatalf("staging file changed: %q err=%v", got, readErr)
	}
}

func TestInspectRootRejectsInvalidCurrentManifest(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, RootManifestName)
	original := []byte(`{"format":"other","version":1}`)
	if err := os.WriteFile(manifestPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := PrepareRoot(root)
	if state != RootInvalid || !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
	got, readErr := os.ReadFile(manifestPath)
	if readErr != nil || string(got) != string(original) {
		t.Fatalf("manifest changed: %q err=%v", got, readErr)
	}
}

func TestInspectRootRejectsCurrentManifestWithoutUsersDirectory(t *testing.T) {
	root := t.TempDir()
	manifest := []byte(`{"format":"novelreader-data-root","version":1}`)
	if err := os.WriteFile(filepath.Join(root, RootManifestName), manifest, 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := PrepareRoot(root)
	if state != RootInvalid || !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
}

func TestInspectRootRejectsSymlinkedManifest(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "storage.json")
	original := []byte(`{"format":"novelreader-data-root","version":1}`)
	if err := os.WriteFile(outside, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, UsersDirectory), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, RootManifestName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	state, err := PrepareRoot(root)
	if state != RootInvalid || !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
	got, readErr := os.ReadFile(outside)
	if readErr != nil || string(got) != string(original) {
		t.Fatalf("outside manifest changed: %q err=%v", got, readErr)
	}
}

func TestInspectRootRejectsSymlinkedUsersDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, RootManifestName), []byte(`{"format":"novelreader-data-root","version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, UsersDirectory)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	state, err := PrepareRoot(root)
	if state != RootInvalid || !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
}

func TestInspectRootTreatsDanglingStagingSymlinkAsInterrupted(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, rootManifestStagingName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	state, err := PrepareRoot(root)
	if state != RootInterrupted || !errors.Is(err, ErrInterruptedRoot) {
		t.Fatalf("state=%q err=%v", state, err)
	}
}

func TestContainsPathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "database-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	inside, err := ContainsPath(root, filepath.Join(link, "novelreader.db"))
	if err != nil {
		t.Fatal(err)
	}
	if inside {
		t.Fatal("symlink escape classified inside root")
	}
}

func TestContainsPathRejectsSymlinkedRoot(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(parent, "linked")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	inside, err := ContainsPath(linkRoot, filepath.Join(linkRoot, "novelreader.db"))
	if err != nil {
		t.Fatal(err)
	}
	if inside {
		t.Fatal("symlinked root accepted")
	}
}
