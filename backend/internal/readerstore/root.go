// Package readerstore owns the portable per-reader storage root and its lifecycle.
package readerstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	RootFormat         = "novelreader-data-root"
	CurrentRootVersion = 1
	RootManifestName   = "storage.json"
	UsersDirectory     = "users"

	rootManifestStagingName = ".storage.json.staging"
)

type RootState string

const (
	RootEmpty       RootState = "empty"
	RootCurrent     RootState = "current"
	RootLegacy      RootState = "legacy"
	RootNewer       RootState = "newer"
	RootInterrupted RootState = "interrupted"
	RootInvalid     RootState = "invalid"
)

var (
	rootInitializationMu sync.Mutex

	ErrLegacyRoot      = errors.New("legacy development data root is unsupported; stop NovelReader, remove or rename DATA_DIR, restart, then re-import test BookSources")
	ErrNewerRoot       = errors.New("data root was created by a newer NovelReader version")
	ErrInterruptedRoot = errors.New("data root initialization was interrupted")
	ErrInvalidRoot     = errors.New("data root manifest is invalid")
)

type RootManifest struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
}

// prepareAnchoredRoot creates or opens root, verifies that the opened handle
// references the exact non-symlink directory inspected by path, and prepares
// the storage layout exclusively through that retained handle.
func prepareAnchoredRoot(root string, openRoot func(string) (*os.Root, error)) (*os.Root, RootState, error) {
	rootInitializationMu.Lock()
	defer rootInitializationMu.Unlock()

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, RootInvalid, fmt.Errorf("readerstore: create data root: %w", err)
	}
	pathInfo, err := os.Lstat(root)
	if err != nil {
		return nil, RootInvalid, fmt.Errorf("readerstore: inspect data root: %w", err)
	}
	if !pathInfo.IsDir() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, RootInvalid, ErrInvalidRoot
	}
	// On Windows, Lstat may defer loading the file ID until SameFile.
	// Compare the value with itself now so its identity cannot follow a later
	// pathname replacement when it is compared with the anchored handle.
	if !os.SameFile(pathInfo, pathInfo) {
		return nil, RootInvalid, ErrInvalidRoot
	}
	rootHandle, err := openRoot(root)
	if err != nil {
		return nil, RootInvalid, fmt.Errorf("readerstore: anchor data root: %w", err)
	}
	anchoredInfo, err := rootHandle.Stat(".")
	if err != nil {
		_ = rootHandle.Close()
		return nil, RootInvalid, fmt.Errorf("readerstore: inspect anchored data root: %w", err)
	}
	if !os.SameFile(pathInfo, anchoredInfo) {
		_ = rootHandle.Close()
		return nil, RootInvalid, ErrInvalidRoot
	}

	state, err := inspectAnchoredRoot(rootHandle)
	if err != nil {
		_ = rootHandle.Close()
		return nil, state, err
	}
	if state == RootEmpty {
		if err := initializeAnchoredRoot(rootHandle); err != nil {
			_ = rootHandle.Close()
			return nil, RootEmpty, err
		}
		state, err = inspectAnchoredRoot(rootHandle)
	}
	if err != nil {
		_ = rootHandle.Close()
		return nil, state, err
	}
	if state != RootCurrent {
		_ = rootHandle.Close()
		return nil, state, fmt.Errorf("readerstore: unexpected root state %q", state)
	}
	return rootHandle, state, nil
}

func inspectAnchoredRoot(root *os.Root) (RootState, error) {
	directory, err := root.Open(".")
	if err != nil {
		return RootInvalid, fmt.Errorf("readerstore: read data root: %w", err)
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return RootInvalid, fmt.Errorf("readerstore: read data root: %w", readErr)
	}
	if closeErr != nil {
		return RootInvalid, fmt.Errorf("readerstore: close data root: %w", closeErr)
	}

	if manifestInfo, err := root.Lstat(RootManifestName); err == nil {
		if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
			return RootInvalid, ErrInvalidRoot
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RootInvalid, fmt.Errorf("readerstore: inspect root manifest: %w", err)
	}
	if _, err := root.Lstat(rootManifestStagingName); err == nil {
		return RootInterrupted, ErrInterruptedRoot
	} else if !errors.Is(err, os.ErrNotExist) {
		return RootInvalid, fmt.Errorf("readerstore: inspect staging manifest: %w", err)
	}

	contents, err := root.ReadFile(RootManifestName)
	if errors.Is(err, os.ErrNotExist) {
		if len(entries) == 0 {
			return RootEmpty, nil
		}
		return RootLegacy, ErrLegacyRoot
	}
	if err != nil {
		return RootInvalid, fmt.Errorf("%w: %v", ErrInvalidRoot, err)
	}
	var manifest RootManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return RootInvalid, fmt.Errorf("%w: %v", ErrInvalidRoot, err)
	}
	if manifest.Format != RootFormat || manifest.Version < 1 {
		return RootInvalid, ErrInvalidRoot
	}
	if manifest.Version > CurrentRootVersion {
		return RootNewer, ErrNewerRoot
	}
	if manifest.Version < CurrentRootVersion {
		return RootLegacy, ErrLegacyRoot
	}
	usersInfo, err := root.Lstat(UsersDirectory)
	if err != nil || !usersInfo.IsDir() || usersInfo.Mode()&os.ModeSymlink != 0 {
		return RootInvalid, ErrInvalidRoot
	}
	return RootCurrent, nil
}

func initializeAnchoredRoot(root *os.Root) error {
	manifest := RootManifest{Format: RootFormat, Version: CurrentRootVersion}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("readerstore: encode root manifest: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := root.WriteFile(rootManifestStagingName, encoded, 0o600); err != nil {
		return fmt.Errorf("readerstore: stage root manifest: %w", err)
	}
	if err := root.Mkdir(UsersDirectory, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("readerstore: create users directory: %w", err)
	}
	usersInfo, err := root.Lstat(UsersDirectory)
	if err != nil || !usersInfo.IsDir() || usersInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("readerstore: users path must be a directory inside the data root")
	}
	if err := root.Rename(rootManifestStagingName, RootManifestName); err != nil {
		return fmt.Errorf("readerstore: publish root manifest: %w", err)
	}
	return nil
}

// PrepareRoot classifies the data root and initializes it only when it is empty.
// Refused roots are never modified.
func PrepareRoot(root string) (RootState, error) {
	rootInitializationMu.Lock()
	defer rootInitializationMu.Unlock()

	state, err := InspectRoot(root)
	if err != nil {
		return state, err
	}
	if state == RootCurrent {
		return state, nil
	}
	if state != RootEmpty {
		return state, fmt.Errorf("readerstore: unexpected root state %q", state)
	}
	if err := initializeRoot(root); err != nil {
		return RootEmpty, err
	}
	return RootCurrent, nil
}

// InspectRoot reports whether root is empty, current, or unsafe to open.
func InspectRoot(root string) (RootState, error) {
	rootInfo, err := os.Lstat(root)
	if err == nil && rootInfo.Mode()&os.ModeSymlink != 0 {
		return RootInvalid, ErrInvalidRoot
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RootInvalid, fmt.Errorf("readerstore: inspect data root: %w", err)
	}

	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return RootEmpty, nil
	}
	if err != nil {
		return RootInvalid, fmt.Errorf("readerstore: read data root: %w", err)
	}

	manifestPath := filepath.Join(root, RootManifestName)
	if manifestInfo, err := os.Lstat(manifestPath); err == nil {
		if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
			return RootInvalid, ErrInvalidRoot
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RootInvalid, fmt.Errorf("readerstore: inspect root manifest: %w", err)
	}
	if _, err := os.Lstat(filepath.Join(root, rootManifestStagingName)); err == nil {
		return RootInterrupted, ErrInterruptedRoot
	} else if !errors.Is(err, os.ErrNotExist) {
		return RootInvalid, fmt.Errorf("readerstore: inspect staging manifest: %w", err)
	}

	manifest, err := readManifest(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		if len(entries) == 0 {
			return RootEmpty, nil
		}
		return RootLegacy, ErrLegacyRoot
	}
	if err != nil {
		return RootInvalid, fmt.Errorf("%w: %v", ErrInvalidRoot, err)
	}
	if manifest.Format != RootFormat || manifest.Version < 1 {
		return RootInvalid, ErrInvalidRoot
	}
	if manifest.Version > CurrentRootVersion {
		return RootNewer, ErrNewerRoot
	}
	if manifest.Version < CurrentRootVersion {
		return RootLegacy, ErrLegacyRoot
	}
	usersInfo, err := os.Lstat(filepath.Join(root, UsersDirectory))
	if err != nil || !usersInfo.IsDir() || usersInfo.Mode()&os.ModeSymlink != 0 {
		return RootInvalid, ErrInvalidRoot
	}
	return RootCurrent, nil
}

// ContainsPath resolves existing symlinks and reports whether path stays inside root.
// The data root itself must not be a symlink because cold backup copies that directory.
func ContainsPath(root, path string) (bool, error) {
	if info, err := os.Lstat(root); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	resolvedRoot, err := resolveWithMissing(root)
	if err != nil {
		return false, err
	}
	resolvedPath, err := resolveWithMissing(path)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return false, err
	}
	return relative != ".." && !startsWithParent(relative), nil
}

func ReadRootManifest(root string) (RootManifest, error) {
	manifest, err := readManifest(filepath.Join(root, RootManifestName))
	if err != nil {
		return RootManifest{}, fmt.Errorf("readerstore: read root manifest: %w", err)
	}
	return manifest, nil
}

func initializeRoot(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("readerstore: create data root: %w", err)
	}

	manifest := RootManifest{Format: RootFormat, Version: CurrentRootVersion}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("readerstore: encode root manifest: %w", err)
	}
	encoded = append(encoded, '\n')

	stagingPath := filepath.Join(root, rootManifestStagingName)
	manifestPath := filepath.Join(root, RootManifestName)
	if err := os.WriteFile(stagingPath, encoded, 0o600); err != nil {
		return fmt.Errorf("readerstore: stage root manifest: %w", err)
	}
	usersPath := filepath.Join(root, UsersDirectory)
	if err := os.Mkdir(usersPath, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("readerstore: create users directory: %w", err)
	}
	usersInfo, err := os.Lstat(usersPath)
	if err != nil || !usersInfo.IsDir() || usersInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("readerstore: users path must be a directory inside the data root")
	}
	if err := os.Rename(stagingPath, manifestPath); err != nil {
		if _, statErr := os.Stat(manifestPath); statErr == nil {
			_ = os.Remove(stagingPath)
			state, inspectErr := InspectRoot(root)
			if state == RootCurrent && inspectErr == nil {
				return nil
			}
		}
		return fmt.Errorf("readerstore: publish root manifest: %w", err)
	}
	return nil
}

func resolveWithMissing(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolute
	var missing []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("readerstore: no existing ancestor for %q", path)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func startsWithParent(relative string) bool {
	return len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator)
}

func readManifest(path string) (RootManifest, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return RootManifest{}, err
	}
	var manifest RootManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return RootManifest{}, err
	}
	return manifest, nil
}
