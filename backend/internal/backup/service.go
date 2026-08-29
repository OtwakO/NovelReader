package backup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const restoreLifetime = 30 * time.Minute

var (
	ErrRestoreNotFound = errors.New("backup: restore operation not found")
	ErrRestoreConflict = errors.New("backup: restore operation already exists")
)

type ExportInfo struct {
	Filename  string
	CreatedAt time.Time
}

type PreparedRestore struct {
	ID                   string `json:"operationId"`
	CreatedAt            string `json:"createdAt"`
	ExportedFromUsername string `json:"exportedFromUsername"`
	ReaderSchemaVersion  int    `json:"readerSchemaVersion"`
	CurrentSchemaVersion int    `json:"currentSchemaVersion"`
	Compatibility        string `json:"compatibility"`
	ExpiresAt            string `json:"expiresAt"`
}

type RestoreResult struct {
	Restored bool `json:"restored"`
}

type operation struct {
	owner     readerstore.UserID
	staging   string
	expiresAt time.Time
	summary   PreparedRestore
}

// Service owns archive staging and delegates Reader-home lifecycle to readerstore.
type Service struct {
	readers  *readerstore.Manager
	root     string
	quiesce  func(context.Context, readerstore.UserID) error
	resume   func(readerstore.UserID)
	now      func() time.Time
	mu       sync.Mutex
	byID     map[string]operation
	byReader map[readerstore.UserID]string
}

func NewService(readers *readerstore.Manager, dataRoot string, quiesce func(context.Context, readerstore.UserID) error, resume func(readerstore.UserID)) *Service {
	return &Service{
		readers: readers, root: dataRoot, quiesce: quiesce, resume: resume, now: time.Now,
		byID: make(map[string]operation), byReader: make(map[readerstore.UserID]string),
	}
}

func (s *Service) Export(ctx context.Context, userID readerstore.UserID, username string, createdAt time.Time, output io.Writer) (ExportInfo, error) {
	createdAt = createdAt.In(time.Local)
	temporary, err := os.MkdirTemp(s.root, ".backup-export-")
	if err != nil {
		return ExportInfo{}, fmt.Errorf("backup: create export staging: %w", err)
	}
	defer os.RemoveAll(temporary)
	homePath := filepath.Join(temporary, PayloadRoot)
	if err := s.readers.SnapshotHome(ctx, userID, homePath); err != nil {
		return ExportInfo{}, err
	}
	manifest := NewManifest(username, createdAt)
	if err := writeArchive(ctx, output, homePath, manifest, createdAt); err != nil {
		return ExportInfo{}, fmt.Errorf("backup: write archive: %w", err)
	}
	return ExportInfo{Filename: Filename(username, createdAt), CreatedAt: createdAt}, nil
}

func (s *Service) PrepareRestore(ctx context.Context, userID readerstore.UserID, input io.Reader) (PreparedRestore, error) {
	now := s.now()
	operationID, err := newOperationID()
	if err != nil {
		return PreparedRestore{}, err
	}
	s.mu.Lock()
	if existingID := s.byReader[userID]; existingID != "" {
		existing, ok := s.byID[existingID]
		if !ok || now.Before(existing.expiresAt) {
			s.mu.Unlock()
			return PreparedRestore{}, ErrRestoreConflict
		}
		delete(s.byID, existingID)
		delete(s.byReader, userID)
		_ = os.RemoveAll(existing.staging)
	}
	s.byReader[userID] = operationID
	s.mu.Unlock()
	reserved := true
	defer func() {
		if !reserved {
			return
		}
		s.mu.Lock()
		if s.byReader[userID] == operationID {
			delete(s.byReader, userID)
		}
		s.mu.Unlock()
	}()
	temporary, err := os.MkdirTemp(s.root, ".backup-restore-")
	if err != nil {
		return PreparedRestore{}, fmt.Errorf("backup: create restore staging: %w", err)
	}
	manifest, payload, err := extractArchive(ctx, input, temporary)
	if err != nil {
		_ = os.RemoveAll(temporary)
		return PreparedRestore{}, err
	}
	if manifest.ReaderSchemaVersion != readerstore.CurrentReaderSchemaVersion {
		_ = os.RemoveAll(temporary)
		return PreparedRestore{}, fmt.Errorf("backup: Reader schema %d is incompatible with current schema %d", manifest.ReaderSchemaVersion, readerstore.CurrentReaderSchemaVersion)
	}
	staging, err := s.readers.PrepareReplacement(ctx, userID, filepath.Join(payload, readerstore.ReaderDatabaseName), filepath.Join(payload, readerstore.FilesDirectory))
	_ = os.RemoveAll(temporary)
	if err != nil {
		return PreparedRestore{}, err
	}
	expiresAt := now.Add(restoreLifetime)
	summary := PreparedRestore{
		ID: operationID, CreatedAt: manifest.CreatedAt, ExportedFromUsername: manifest.ExportedFromUsername,
		ReaderSchemaVersion: manifest.ReaderSchemaVersion, CurrentSchemaVersion: readerstore.CurrentReaderSchemaVersion,
		Compatibility: "compatible", ExpiresAt: expiresAt.Format(time.RFC3339),
	}
	s.mu.Lock()
	s.byID[operationID] = operation{owner: userID, staging: staging, expiresAt: expiresAt, summary: summary}
	s.mu.Unlock()
	reserved = false
	return summary, nil
}

func (s *Service) GetRestore(userID readerstore.UserID, operationID string) (PreparedRestore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.byID[operationID]
	if !ok || operation.owner != userID || !s.now().Before(operation.expiresAt) {
		if ok && operation.owner == userID {
			delete(s.byID, operationID)
			delete(s.byReader, userID)
			_ = os.RemoveAll(operation.staging)
		}
		return PreparedRestore{}, ErrRestoreNotFound
	}
	return operation.summary, nil
}

func (s *Service) CancelRestore(userID readerstore.UserID, operationID string) error {
	operation, err := s.takeOperation(userID, operationID)
	if err != nil {
		return err
	}
	return os.RemoveAll(operation.staging)
}

func (s *Service) CommitRestore(ctx context.Context, userID readerstore.UserID, operationID string) (RestoreResult, error) {
	operation, err := s.takeOperation(userID, operationID)
	if err != nil {
		return RestoreResult{}, err
	}
	defer os.RemoveAll(operation.staging)
	if err := s.quiesce(ctx, userID); err != nil {
		s.resume(userID)
		return RestoreResult{}, err
	}
	defer s.resume(userID)
	if err := s.readers.PublishReplacement(ctx, userID, operation.staging); err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{Restored: true}, nil
}

// Close removes every uncommitted restore staged by this process.
func (s *Service) Close() error {
	s.mu.Lock()
	operations := make([]operation, 0, len(s.byID))
	for _, pending := range s.byID {
		operations = append(operations, pending)
	}
	s.byID = make(map[string]operation)
	s.byReader = make(map[readerstore.UserID]string)
	s.mu.Unlock()
	var cleanupErr error
	for _, pending := range operations {
		cleanupErr = errors.Join(cleanupErr, os.RemoveAll(pending.staging))
	}
	return cleanupErr
}

func (s *Service) takeOperation(userID readerstore.UserID, operationID string) (operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.byID[operationID]
	if !ok || operation.owner != userID || !s.now().Before(operation.expiresAt) {
		if ok && operation.owner == userID {
			delete(s.byID, operationID)
			delete(s.byReader, userID)
			_ = os.RemoveAll(operation.staging)
		}
		return operation, ErrRestoreNotFound
	}
	delete(s.byID, operationID)
	delete(s.byReader, userID)
	return operation, nil
}

func newOperationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("backup: generate restore operation ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}
