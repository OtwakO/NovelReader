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
	UserID  UserID `json:"userId"`
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

func recoverStagedHome(stagingPath, homePath string, userID UserID, migrations []ReaderMigration) error {
	info, err := os.Lstat(stagingPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: unsafe staged reader home", ErrInvalidHome)
	}
	if err := validateHome(stagingPath, userID, migrations); err == nil {
		if err := os.Rename(stagingPath, homePath); err != nil {
			return fmt.Errorf("readerstore: publish staged reader home: %w", err)
		}
		return nil
	}
	if err := os.RemoveAll(stagingPath); err != nil {
		return fmt.Errorf("readerstore: remove incomplete staged reader home: %w", err)
	}
	if err := createStagedHome(stagingPath, userID, migrations); err != nil {
		_ = os.RemoveAll(stagingPath)
		return err
	}
	if err := os.Rename(stagingPath, homePath); err != nil {
		_ = os.RemoveAll(stagingPath)
		return fmt.Errorf("readerstore: publish recovered reader home: %w", err)
	}
	return nil
}

func createStagedHome(path string, userID UserID, migrations []ReaderMigration) error {
	if err := os.Mkdir(path, 0o700); err != nil {
		return fmt.Errorf("readerstore: create staged reader home: %w", err)
	}
	for _, relative := range requiredHomeDirectories() {
		if err := os.MkdirAll(filepath.Join(path, relative), 0o700); err != nil {
			return fmt.Errorf("readerstore: create reader files: %w", err)
		}
	}
	manifest := HomeManifest{Format: HomeFormat, Version: CurrentHomeVersion, UserID: userID}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("readerstore: encode reader manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(path, HomeManifestName), append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("readerstore: write reader manifest: %w", err)
	}
	for _, databaseName := range []string{ReaderDatabaseName, CredentialsDatabaseName} {
		if err := initializeHomeDatabase(filepath.Join(path, databaseName)); err != nil {
			return fmt.Errorf("readerstore: initialize %s: %w", databaseName, err)
		}
	}
	readerDB, err := openHomeDatabase(filepath.Join(path, ReaderDatabaseName))
	if err != nil {
		return err
	}
	migrationErr := applyReaderMigrations(readerDB, migrations)
	closeErr := readerDB.Close()
	if migrationErr != nil {
		return migrationErr
	}
	if closeErr != nil {
		return fmt.Errorf("readerstore: close initialized reader database: %w", closeErr)
	}
	return validateHome(path, userID, migrations)
}

func validateHome(path string, userID UserID, migrations []ReaderMigration) error {
	return validateHomeWithReaderVersion(path, userID, InitialDatabaseVersion+len(migrations), len(migrations))
}

func validateHomeForOpen(path string, userID UserID, migrations []ReaderMigration) error {
	return validateHomeWithReaderVersion(path, userID, 0, len(migrations))
}

func validateHomeWithReaderVersion(path string, userID UserID, requiredReaderVersion, migrationCount int) error {
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
	if err != nil || manifest.Format != HomeFormat || manifest.Version != CurrentHomeVersion || manifest.UserID != userID {
		return ErrInvalidHome
	}
	for _, relative := range append([]string{FilesDirectory}, requiredHomeDirectories()...) {
		fileInfo, err := os.Lstat(filepath.Join(path, relative))
		if err != nil || !fileInfo.IsDir() || fileInfo.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidHome
		}
	}
	for _, databaseName := range []string{ReaderDatabaseName, CredentialsDatabaseName} {
		databasePath := filepath.Join(path, databaseName)
		databaseInfo, err := os.Lstat(databasePath)
		if err != nil || !databaseInfo.Mode().IsRegular() || databaseInfo.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidHome
		}
		minimumVersion, maximumVersion := InitialDatabaseVersion, InitialDatabaseVersion
		if databaseName == ReaderDatabaseName {
			maximumVersion = InitialDatabaseVersion + migrationCount
			if requiredReaderVersion > 0 {
				minimumVersion = requiredReaderVersion
			}
		}
		if err := validateHomeDatabase(databasePath, minimumVersion, maximumVersion); err != nil {
			if errors.Is(err, ErrNewerDatabaseSchema) || errors.Is(err, ErrMigrationOrder) {
				return err
			}
			return ErrInvalidHome
		}
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
