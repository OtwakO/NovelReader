package readerstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// A home generation identifies replacement, not an interpretation or a session.
// It is public cache identity, never a credential or a resource-signing key.
func newHomeGeneration() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}

func writeHomeManifest(path string) error {
	return saveHomeManifest(path, HomeManifest{Format: HomeFormat, Version: CurrentHomeVersion, Generation: newHomeGeneration()})
}

func writePortableHomeManifest(path string) error {
	// A manual restore copies this manifest too; opening it initializes a new
	// identity even when restoring the same archive more than once.
	return saveHomeManifest(path, HomeManifest{Format: HomeFormat, Version: CurrentHomeVersion})
}

func saveHomeManifest(path string, manifest HomeManifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("readerstore: encode reader manifest: %w", err)
	}
	file, err := os.CreateTemp(path, ".manifest-*")
	if err != nil {
		return fmt.Errorf("readerstore: stage reader manifest: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(append(encoded, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("readerstore: write reader manifest: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("readerstore: close reader manifest: %w", closeErr)
	}
	if err := os.Rename(file.Name(), filepath.Join(path, HomeManifestName)); err != nil {
		return fmt.Errorf("readerstore: publish reader manifest: %w", err)
	}
	return nil
}

func readHomeManifest(path string) (HomeManifest, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return HomeManifest{}, err
	}
	var manifest HomeManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return HomeManifest{}, err
	}
	if manifest.Generation != "" {
		value, err := hex.DecodeString(manifest.Generation)
		if err != nil || len(value) != 16 {
			return HomeManifest{}, fmt.Errorf("%w: invalid home generation", ErrInvalidHome)
		}
	}
	return manifest, nil
}

// Called under Manager.mu after home validation; only legacy/portable manifests
// need initialization. Invalid identities are errors, not silently regenerated.
func ensureHomeGeneration(path string) (string, error) {
	manifest, err := readHomeManifest(filepath.Join(path, HomeManifestName))
	if err != nil {
		return "", err
	}
	if manifest.Generation == "" {
		manifest.Generation = newHomeGeneration()
		if err := saveHomeManifest(path, manifest); err != nil {
			return "", err
		}
	}
	return manifest.Generation, nil
}
