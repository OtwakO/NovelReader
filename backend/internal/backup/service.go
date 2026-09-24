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
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	restoreLifetime        = 30 * time.Minute
	restoreCleanupInterval = time.Minute
	exportWorkspacePrefix  = ".backup-export-"
	restoreWorkspacePrefix = ".backup-restore-"
)

var (
	ErrRestoreNotFound = errors.New("backup: restore operation not found")
	ErrRestoreConflict = errors.New("backup: restore operation already exists")
)

type ExportInfo struct {
	Filename  string
	CreatedAt time.Time
}

type PreparedRestore struct {
	ID                   string         `json:"operationId"`
	CreatedAt            string         `json:"createdAt"`
	ExportedFromUsername string         `json:"exportedFromUsername"`
	ReaderSchemaVersion  int            `json:"readerSchemaVersion"`
	CurrentSchemaVersion int            `json:"currentSchemaVersion"`
	Compatibility        string         `json:"compatibility"`
	ExpiresAt            string         `json:"expiresAt"`
	State                string         `json:"state"`
	Result               *RestoreResult `json:"result,omitempty"`
}

type RestoreResult struct {
	Restored bool     `json:"restored"`
	Warnings []string `json:"warnings,omitempty"`
}

type operation struct {
	owner     readerstore.UserID
	staging   string
	expiresAt time.Time
	summary   PreparedRestore
}

// Service owns archive staging and delegates Reader-home lifecycle to readerstore.
type Service struct {
	readers *readerstore.Manager
	root    string
	quiesce func(context.Context, readerstore.UserID) error
	resume  func(readerstore.UserID)
	// Runs after publication while quiescent; returns safe warning codes, not raw errors.
	afterRestore func(context.Context, readerstore.UserID) []string
	now          func() time.Time
	mu           sync.Mutex
	byID         map[string]operation
	byReader     map[readerstore.UserID]string
	janitorTicks <-chan time.Time
	janitorTimer *time.Ticker
	janitorStop  chan struct{}
	janitorDone  chan struct{}
	closed       bool
	commits      sync.WaitGroup
}

func NewService(readers *readerstore.Manager, dataRoot string, quiesce func(context.Context, readerstore.UserID) error, resume func(readerstore.UserID), afterRestore func(context.Context, readerstore.UserID) []string) (*Service, error) {
	timer := time.NewTicker(restoreCleanupInterval)
	service, err := newService(readers, dataRoot, quiesce, resume, afterRestore, timer.C)
	if err != nil {
		timer.Stop()
		return nil, err
	}
	service.janitorTimer = timer
	return service, nil
}

func newService(readers *readerstore.Manager, dataRoot string, quiesce func(context.Context, readerstore.UserID) error, resume func(readerstore.UserID), afterRestore func(context.Context, readerstore.UserID) []string, ticks <-chan time.Time) (*Service, error) {
	if err := removeAbandonedWorkspaces(dataRoot); err != nil {
		return nil, err
	}
	service := &Service{
		readers: readers, root: dataRoot, quiesce: quiesce, resume: resume, afterRestore: afterRestore, now: time.Now,
		byID: make(map[string]operation), byReader: make(map[readerstore.UserID]string), janitorTicks: ticks,
		janitorStop: make(chan struct{}), janitorDone: make(chan struct{}),
	}
	go service.runJanitor()
	return service, nil
}

func (s *Service) Export(ctx context.Context, userID readerstore.UserID, username string, createdAt time.Time, output io.Writer) (ExportInfo, error) {
	temporary, err := os.MkdirTemp(s.root, exportWorkspacePrefix)
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
	if s.closed {
		s.mu.Unlock()
		return PreparedRestore{}, ErrRestoreConflict
	}
	if existingID := s.byReader[userID]; existingID != "" {
		existing, ok := s.byID[existingID]
		if !ok || existing.summary.State == "committing" || (existing.summary.State == "prepared" && now.Before(existing.expiresAt)) {
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
	temporary, err := os.MkdirTemp(s.root, restoreWorkspacePrefix)
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
		Compatibility: "compatible", ExpiresAt: expiresAt.Format(time.RFC3339), State: "prepared",
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = os.RemoveAll(staging)
		return PreparedRestore{}, ErrRestoreConflict
	}
	s.byID[operationID] = operation{owner: userID, staging: staging, expiresAt: expiresAt, summary: summary}
	s.mu.Unlock()
	reserved = false
	return summary, nil
}

// Close removes every uncommitted restore staged by this process.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.janitorTimer != nil {
		s.janitorTimer.Stop()
	}
	close(s.janitorStop)
	s.mu.Unlock()
	<-s.janitorDone
	s.commits.Wait() // Never remove staging or lifecycle ownership from a live commit.
	operations := s.takeExpiredOperations(time.Time{})
	var cleanupErr error
	for _, pending := range operations {
		cleanupErr = errors.Join(cleanupErr, os.RemoveAll(pending.staging))
	}
	return cleanupErr
}

func (s *Service) runJanitor() {
	defer close(s.janitorDone)
	for {
		select {
		case <-s.janitorTicks:
			for _, expired := range s.takeExpiredOperations(s.now()) {
				_ = os.RemoveAll(expired.staging)
			}
		case <-s.janitorStop:
			return
		}
	}
}

// takeExpiredOperations removes expired operations from the indexes. A zero cutoff
// removes every registered operation and is used after the janitor has stopped.
func (s *Service) takeExpiredOperations(cutoff time.Time) []operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	operations := make([]operation, 0)
	for id, pending := range s.byID {
		if pending.summary.State == "committing" || (!cutoff.IsZero() && cutoff.Before(pending.expiresAt)) {
			continue
		}
		operations = append(operations, pending)
		delete(s.byID, id)
		if s.byReader[pending.owner] == id {
			delete(s.byReader, pending.owner)
		}
	}
	return operations
}

func removeAbandonedWorkspaces(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("backup: inspect abandoned workspaces: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), exportWorkspacePrefix) && !strings.HasPrefix(entry.Name(), restoreWorkspacePrefix) {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup: unsafe abandoned workspace %q", entry.Name())
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return fmt.Errorf("backup: remove abandoned workspace %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func newOperationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("backup: generate restore operation ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}
