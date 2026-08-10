package readerstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

var ErrManagerClosed = errors.New("readerstore: manager is closed")

// Manager owns bounded reader database handles and all reader-home paths.
type Manager struct {
	root     string
	capacity int
	notify   chan struct{}

	mu      sync.Mutex
	closed  bool
	entries map[UserID]*homeEntry
}

type homeEntry struct {
	id            UserID
	path          string
	readerDB      *sql.DB
	credentialsDB *sql.DB
	references    int
}

type Home struct {
	manager *Manager
	entry   *homeEntry
	once    sync.Once
}

func NewManager(root string, capacity int) (*Manager, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("readerstore: capacity must be positive")
	}
	state, err := PrepareRoot(root)
	if err != nil {
		return nil, err
	}
	if state != RootCurrent {
		return nil, fmt.Errorf("readerstore: root state %q", state)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("readerstore: resolve data root: %w", err)
	}
	return &Manager{
		root:     absoluteRoot,
		capacity: capacity,
		notify:   make(chan struct{}),
		entries:  make(map[UserID]*homeEntry),
	}, nil
}

// Exists reports whether a published or staged reader home already exists for the identity.
func (m *Manager) Exists(ctx context.Context, userID UserID) (bool, error) {
	if err := validateUserID(userID); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false, ErrManagerClosed
	}
	homePath, err := m.homePath(userID)
	if err != nil {
		return false, err
	}
	for _, path := range []string{homePath, homePath + homeStagingSuffix} {
		if _, err := os.Lstat(path); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("readerstore: inspect reader home: %w", err)
		}
	}
	return false, nil
}

func (m *Manager) Create(ctx context.Context, userID UserID) error {
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

	homePath, err := m.homePath(userID)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(homePath); err == nil {
		return validateHome(homePath, userID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("readerstore: inspect reader home: %w", err)
	}

	stagingPath := homePath + homeStagingSuffix
	if _, err := os.Lstat(stagingPath); err == nil {
		if err := recoverStagedHome(stagingPath, homePath, userID); err != nil {
			return err
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("readerstore: inspect staged reader home: %w", err)
	}
	if err := createStagedHome(stagingPath, userID); err != nil {
		_ = os.RemoveAll(stagingPath)
		return err
	}
	if err := os.Rename(stagingPath, homePath); err != nil {
		_ = os.RemoveAll(stagingPath)
		if validateErr := validateHome(homePath, userID); validateErr == nil {
			return nil
		}
		return fmt.Errorf("readerstore: publish reader home: %w", err)
	}
	return nil
}

func (m *Manager) Open(ctx context.Context, userID UserID) (*Home, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return nil, ErrManagerClosed
		}
		if entry := m.entries[userID]; entry != nil {
			entry.references++
			m.mu.Unlock()
			return &Home{manager: m, entry: entry}, nil
		}
		if len(m.entries) >= m.capacity {
			if idle := m.removeIdleEntry(); idle != nil {
				_ = errors.Join(idle.readerDB.Close(), idle.credentialsDB.Close())
			} else {
				notify := m.notify
				m.mu.Unlock()
				select {
				case <-notify:
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
		entry, err := m.openEntry(userID)
		if err == nil {
			m.entries[userID] = entry
		}
		m.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return &Home{manager: m, entry: entry}, nil
	}
}

func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	entries := make([]*homeEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		entries = append(entries, entry)
	}
	m.entries = make(map[UserID]*homeEntry)
	m.signalLocked()
	m.mu.Unlock()

	var closeErr error
	for _, entry := range entries {
		closeErr = errors.Join(closeErr, entry.readerDB.Close(), entry.credentialsDB.Close())
	}
	return closeErr
}

func (h *Home) ID() UserID  { return h.entry.id }
func (h *Home) DB() *sql.DB { return h.entry.readerDB }
func (h *Home) Files() FileStore {
	return FileStore{
		dataRoot: h.manager.root,
		root:     filepath.Join(h.entry.path, FilesDirectory),
	}
}
func (h *Home) Close() error {
	var closeErr error
	h.once.Do(func() { closeErr = h.manager.release(h.entry) })
	return closeErr
}

func (m *Manager) openEntry(userID UserID) (*homeEntry, error) {
	homePath, err := m.homePath(userID)
	if err != nil {
		return nil, err
	}
	if err := validateHome(homePath, userID); err != nil {
		return nil, err
	}
	readerDB, err := openHomeDatabase(filepath.Join(homePath, ReaderDatabaseName))
	if err != nil {
		return nil, err
	}
	credentialsDB, err := openHomeDatabase(filepath.Join(homePath, CredentialsDatabaseName))
	if err != nil {
		_ = readerDB.Close()
		return nil, err
	}
	return &homeEntry{id: userID, path: homePath, readerDB: readerDB, credentialsDB: credentialsDB, references: 1}, nil
}

func (m *Manager) release(entry *homeEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.entries[entry.id] != entry {
		return nil
	}
	entry.references--
	if entry.references == 0 {
		m.signalLocked()
	}
	return nil
}

func (m *Manager) removeIdleEntry() *homeEntry {
	for userID, entry := range m.entries {
		if entry.references == 0 {
			delete(m.entries, userID)
			return entry
		}
	}
	return nil
}

func (m *Manager) signalLocked() {
	close(m.notify)
	m.notify = make(chan struct{})
}

func (m *Manager) homePath(userID UserID) (string, error) {
	usersRoot := filepath.Join(m.root, UsersDirectory)
	usersInfo, err := os.Lstat(usersRoot)
	if err != nil || !usersInfo.IsDir() || usersInfo.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalidRoot
	}
	path := filepath.Join(usersRoot, string(userID))
	inside, err := ContainsPath(m.root, path)
	if err != nil {
		return "", fmt.Errorf("readerstore: resolve reader home: %w", err)
	}
	if !inside {
		return "", ErrInvalidUserID
	}
	return path, nil
}
