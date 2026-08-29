package readerstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	HomeFormat              = "novelreader-reader-home"
	CurrentHomeVersion      = 1
	HomeManifestName        = "manifest.json"
	ReaderDatabaseName      = "reader.db"
	CredentialsDatabaseName = "credentials.db"
	FilesDirectory          = "files"
	FontsDirectory          = "fonts"
	CoversDirectory         = "covers"
	ChapterAssetsDirectory  = "chapter-assets"

	homeStagingSuffix = ".staging"
)

var (
	ErrHomeNotFound    = errors.New("readerstore: reader home not found")
	ErrInvalidHome     = errors.New("readerstore: reader home is invalid")
	ErrInvalidFilePath = errors.New("readerstore: invalid file path")
)

type HomeManifest struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
}

type FileStore struct {
	dataRoot string
	root     string
}

func (f FileStore) WriteFile(data []byte, perm os.FileMode, segments ...string) error {
	relative, err := filePath(segments)
	if err != nil {
		return err
	}
	root, err := f.openRoot()
	if err != nil {
		return fmt.Errorf("readerstore: open files root: %w", err)
	}
	defer root.Close()
	if err := root.WriteFile(relative, data, perm); err != nil {
		return fmt.Errorf("readerstore: write reader file: %w", err)
	}
	return nil
}

func (f FileStore) ReadFile(segments ...string) ([]byte, error) {
	relative, err := filePath(segments)
	if err != nil {
		return nil, err
	}
	root, err := f.openRoot()
	if err != nil {
		return nil, fmt.Errorf("readerstore: open files root: %w", err)
	}
	defer root.Close()
	contents, err := root.ReadFile(relative)
	if err != nil {
		return nil, fmt.Errorf("readerstore: read reader file: %w", err)
	}
	return contents, nil
}

func (f FileStore) Remove(segments ...string) error {
	relative, err := filePath(segments)
	if err != nil {
		return err
	}
	root, err := f.openRoot()
	if err != nil {
		return fmt.Errorf("readerstore: open files root: %w", err)
	}
	defer root.Close()
	if err := root.Remove(relative); err != nil {
		return fmt.Errorf("readerstore: remove reader file: %w", err)
	}
	return nil
}

func (f FileStore) openRoot() (*os.Root, error) {
	inside, err := ContainsPath(f.dataRoot, f.root)
	if err != nil {
		return nil, fmt.Errorf("readerstore: validate files root: %w", err)
	}
	if !inside {
		return nil, ErrInvalidFilePath
	}
	info, err := os.Lstat(f.root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidFilePath
	}
	return os.OpenRoot(f.root)
}

func filePath(segments []string) (string, error) {
	if len(segments) == 0 {
		return "", ErrInvalidFilePath
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || filepath.IsAbs(segment) || filepath.Base(segment) != segment {
			return "", ErrInvalidFilePath
		}
	}
	return filepath.Join(segments...), nil
}

func recoverStagedHome(stagingPath, homePath string, schemas []ReaderSchema) error {
	info, err := os.Lstat(stagingPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: unsafe staged reader home", ErrInvalidHome)
	}
	if err := validateHome(stagingPath, schemas); err == nil {
		if err := os.Rename(stagingPath, homePath); err != nil {
			return fmt.Errorf("readerstore: publish staged reader home: %w", err)
		}
		return nil
	}
	if err := os.RemoveAll(stagingPath); err != nil {
		return fmt.Errorf("readerstore: remove incomplete staged reader home: %w", err)
	}
	if err := createStagedHome(stagingPath, schemas); err != nil {
		_ = os.RemoveAll(stagingPath)
		return err
	}
	if err := os.Rename(stagingPath, homePath); err != nil {
		_ = os.RemoveAll(stagingPath)
		return fmt.Errorf("readerstore: publish recovered reader home: %w", err)
	}
	return nil
}

func createStagedHome(path string, schemas []ReaderSchema) error {
	if err := os.Mkdir(path, 0o700); err != nil {
		return fmt.Errorf("readerstore: create staged reader home: %w", err)
	}
	for _, relative := range requiredHomeDirectories() {
		if err := os.MkdirAll(filepath.Join(path, relative), 0o700); err != nil {
			return fmt.Errorf("readerstore: create reader files: %w", err)
		}
	}
	if err := writeHomeManifest(path); err != nil {
		return err
	}
	if err := initializeReaderDatabase(filepath.Join(path, ReaderDatabaseName), schemas); err != nil {
		return fmt.Errorf("readerstore: initialize %s: %w", ReaderDatabaseName, err)
	}
	if err := initializeCredentialsDatabase(filepath.Join(path, CredentialsDatabaseName)); err != nil {
		return fmt.Errorf("readerstore: initialize %s: %w", CredentialsDatabaseName, err)
	}
	return validateHome(path, schemas)
}

func validateHome(path string, schemas []ReaderSchema) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrHomeNotFound
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidHome
	}
	manifestPath := filepath.Join(path, HomeManifestName)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil || !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidHome
	}
	manifest, err := readHomeManifest(manifestPath)
	if err != nil || manifest.Format != HomeFormat || manifest.Version != CurrentHomeVersion {
		return ErrInvalidHome
	}
	for _, relative := range append([]string{FilesDirectory}, requiredHomeDirectories()...) {
		fileInfo, err := os.Lstat(filepath.Join(path, relative))
		if err != nil || !fileInfo.IsDir() || fileInfo.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidHome
		}
	}
	readerDatabasePath := filepath.Join(path, ReaderDatabaseName)
	readerDatabaseInfo, err := os.Lstat(readerDatabasePath)
	if err != nil || !readerDatabaseInfo.Mode().IsRegular() || readerDatabaseInfo.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidHome
	}
	if err := validateHomeDatabase(readerDatabasePath, CurrentReaderSchemaVersion, schemas); err != nil {
		return err
	}
	credentialsPath := filepath.Join(path, CredentialsDatabaseName)
	credentialsInfo, err := os.Lstat(credentialsPath)
	if err != nil || !credentialsInfo.Mode().IsRegular() || credentialsInfo.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidHome
	}
	if err := validateCredentialsDatabase(credentialsPath); err != nil {
		return ErrInvalidHome
	}
	return nil
}

func requiredHomeDirectories() []string {
	return []string{
		filepath.Join(FilesDirectory, FontsDirectory),
		filepath.Join(FilesDirectory, CoversDirectory),
		filepath.Join(FilesDirectory, ChapterAssetsDirectory),
	}
}

func writeHomeManifest(path string) error {
	manifest := HomeManifest{Format: HomeFormat, Version: CurrentHomeVersion}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("readerstore: encode reader manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(path, HomeManifestName), append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("readerstore: write reader manifest: %w", err)
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
	return manifest, nil
}
