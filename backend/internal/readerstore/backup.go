package readerstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	backupStagingSuffix  = ".restore-staging"
	backupRollbackSuffix = ".restore-rollback"
)

// SnapshotHome writes a complete, account-neutral Reader home to destination.
// The caller owns destination and may archive it after this method returns.
func (m *Manager) SnapshotHome(ctx context.Context, userID UserID, destination string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		return fmt.Errorf("readerstore: create snapshot home: %w", err)
	}
	cleanup := func(err error) error {
		_ = os.RemoveAll(destination)
		return err
	}
	home, err := m.Open(ctx, userID)
	if err != nil {
		return cleanup(err)
	}
	defer home.Close()
	if err := writeHomeManifest(destination); err != nil {
		return cleanup(err)
	}
	if err := backupDatabase(ctx, home.DB(), filepath.Join(destination, ReaderDatabaseName)); err != nil {
		return cleanup(fmt.Errorf("readerstore: snapshot reader database: %w", err))
	}
	if err := initializeCredentialsDatabase(filepath.Join(destination, CredentialsDatabaseName)); err != nil {
		return cleanup(fmt.Errorf("readerstore: initialize snapshot credentials: %w", err))
	}
	if err := copyDurableFiles(home.Files().root, filepath.Join(destination, FilesDirectory)); err != nil {
		return cleanup(err)
	}
	if err := validateHome(destination, m.schemas); err != nil {
		return cleanup(err)
	}
	return nil
}

// PrepareReplacement creates an account-neutral staged reader home from a validated
// database snapshot and durable files. It does not alter the published reader home.
func (m *Manager) PrepareReplacement(ctx context.Context, userID UserID, readerDatabase string, filesRoot string) (string, error) {
	if err := validateUserID(userID); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", ErrManagerClosed
	}
	if m.deleting[userID] {
		return "", ErrHomeDeleting
	}
	if err := m.validateRootPath(); err != nil {
		return "", err
	}
	homePath, err := m.homePath(userID)
	if err != nil {
		return "", err
	}
	stagingPath := homePath + backupStagingSuffix
	if err := safeRemoveDirectory(stagingPath); err != nil {
		return "", fmt.Errorf("readerstore: clear replacement staging: %w", err)
	}
	if err := os.Mkdir(stagingPath, 0o700); err != nil {
		return "", fmt.Errorf("readerstore: create replacement staging: %w", err)
	}
	cleanup := func(err error) (string, error) {
		_ = os.RemoveAll(stagingPath)
		return "", err
	}
	if err := writeHomeManifest(stagingPath); err != nil {
		return cleanup(err)
	}
	if err := copyRegularFile(readerDatabase, filepath.Join(stagingPath, ReaderDatabaseName), 0o600); err != nil {
		return cleanup(fmt.Errorf("readerstore: stage reader database: %w", err))
	}
	if err := initializeCredentialsDatabase(filepath.Join(stagingPath, CredentialsDatabaseName)); err != nil {
		return cleanup(fmt.Errorf("readerstore: stage credentials database: %w", err))
	}
	if err := copyDurableFiles(filesRoot, filepath.Join(stagingPath, FilesDirectory)); err != nil {
		return cleanup(err)
	}
	if err := validateHome(stagingPath, m.schemas); err != nil {
		return cleanup(err)
	}
	return stagingPath, nil
}

// PublishReplacement atomically replaces one reader home and restores the previous
// home if the replacement cannot be validated or opened. The caller must first drain
// feature-level runtimes so no lease remains against the old home.
func (m *Manager) PublishReplacement(ctx context.Context, userID UserID, stagingPath string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	if m.deleting[userID] {
		return ErrHomeDeleting
	}
	if err := m.validateRootPath(); err != nil {
		return err
	}
	homePath, err := m.homePath(userID)
	if err != nil {
		return err
	}
	if stagingPath != homePath+backupStagingSuffix {
		return ErrInvalidHome
	}
	if entry := m.entries[userID]; entry != nil {
		if entry.references > 0 {
			return fmt.Errorf("readerstore: reader home is still in use")
		}
		delete(m.entries, userID)
		if err := errors.Join(entry.readerDB.Close(), entry.credentialsDB.Close()); err != nil {
			return fmt.Errorf("readerstore: close reader home before replacement: %w", err)
		}
	}
	if err := validateHome(stagingPath, m.schemas); err != nil {
		return err
	}
	rollbackPath := homePath + backupRollbackSuffix
	if err := safeRemoveDirectory(rollbackPath); err != nil {
		return fmt.Errorf("readerstore: clear replacement rollback: %w", err)
	}
	if err := os.Rename(homePath, rollbackPath); err != nil {
		return fmt.Errorf("readerstore: preserve current reader home: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(homePath)
			_ = os.Rename(rollbackPath, homePath)
		}
	}()
	if err := os.Rename(stagingPath, homePath); err != nil {
		return fmt.Errorf("readerstore: publish replacement reader home: %w", err)
	}
	if err := validateHome(homePath, m.schemas); err != nil {
		return fmt.Errorf("readerstore: validate replacement reader home: %w", err)
	}
	entry, err := m.openEntry(userID)
	if err != nil {
		return fmt.Errorf("readerstore: open replacement reader home: %w", err)
	}
	entry.references = 0
	m.entries[userID] = entry
	published = true
	if err := os.RemoveAll(rollbackPath); err != nil {
		return fmt.Errorf("readerstore: remove replacement rollback: %w", err)
	}
	m.signalLocked()
	return nil
}

func backupDatabase(ctx context.Context, source *sql.DB, destination string) error {
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err := source.ExecContext(ctx, `VACUUM INTO ?`, destination)
	return err
}

func copyDurableFiles(source, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return fmt.Errorf("readerstore: create replacement files: %w", err)
	}
	for _, relative := range requiredHomeDirectories() {
		target := filepath.Join(destination, relative[len(FilesDirectory)+1:])
		if err := os.MkdirAll(target, 0o700); err != nil {
			return fmt.Errorf("readerstore: create replacement file directory: %w", err)
		}
	}
	if source == "" {
		return nil
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalidFilePath
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return ErrInvalidFilePath
		}
		return copyRegularFile(path, target, 0o600)
	})
}

func copyRegularFile(source, destination string, perm os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ErrInvalidFilePath
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	return errors.Join(copyErr, closeErr)
}

func safeRemoveDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidHome
	}
	return os.RemoveAll(path)
}
