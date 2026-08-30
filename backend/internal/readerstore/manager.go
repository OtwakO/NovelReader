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

var (
	ErrManagerClosed = errors.New("readerstore: manager is closed")
	ErrHomeDeleting  = errors.New("readerstore: reader home is being deleted")
)

// Manager owns bounded reader database handles and all reader-home paths.
type Manager struct {
	root       string
	rootHandle *os.Root
	capacity   int
	notify     chan struct{}

	mu         sync.Mutex
	closed     bool
	entries    map[UserID]*homeEntry
	deleting   map[UserID]bool
	schemas    []ReaderSchema
	renameHome func(*os.Root, string, string) error
	removeHome func(*os.Root, string) error
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

func NewManager(root string, capacity int, schemas ...ReaderSchema) (*Manager, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("readerstore: capacity must be positive")
	}
	for _, schema := range schemas {
		if schema.Initialize == nil {
			return nil, fmt.Errorf("readerstore: reader schema initializer is required")
		}
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("readerstore: resolve data root: %w", err)
	}
	rootHandle, state, err := prepareAnchoredRoot(absoluteRoot, os.OpenRoot)
	if err != nil {
		return nil, err
	}
	if state != RootCurrent {
		_ = rootHandle.Close()
		return nil, fmt.Errorf("readerstore: root state %q", state)
	}
	if err := reconcileReplacementArtifacts(absoluteRoot, schemas); err != nil {
		_ = rootHandle.Close()
		return nil, err
	}
	return &Manager{
		root:       absoluteRoot,
		rootHandle: rootHandle,
		capacity:   capacity,
		notify:     make(chan struct{}),
		entries:    make(map[UserID]*homeEntry),
		deleting:   make(map[UserID]bool),
		schemas:    append([]ReaderSchema(nil), schemas...),
		renameHome: func(root *os.Root, oldName, newName string) error { return root.Rename(oldName, newName) },
		removeHome: func(root *os.Root, name string) error { return root.RemoveAll(name) },
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
	if err := m.validateRootPath(); err != nil {
		return false, err
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
	if _, err := os.Lstat(homePath); err == nil {
		return validateHome(homePath, m.schemas)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("readerstore: inspect reader home: %w", err)
	}

	stagingPath := homePath + homeStagingSuffix
	if _, err := os.Lstat(stagingPath); err == nil {
		if err := recoverStagedHome(stagingPath, homePath, m.schemas); err != nil {
			return err
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("readerstore: inspect staged reader home: %w", err)
	}
	if err := createStagedHome(stagingPath, m.schemas); err != nil {
		_ = os.RemoveAll(stagingPath)
		return err
	}
	if err := os.Rename(stagingPath, homePath); err != nil {
		_ = os.RemoveAll(stagingPath)
		if validateErr := validateHome(homePath, m.schemas); validateErr == nil {
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
		if m.deleting[userID] {
			m.mu.Unlock()
			return nil, ErrHomeDeleting
		}
		if entry := m.entries[userID]; entry != nil {
			entry.references++
			m.mu.Unlock()
			return &Home{manager: m, entry: entry}, nil
		}
		if err := m.validateRootPath(); err != nil {
			m.mu.Unlock()
			return nil, err
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

// Remove permanently deletes the immutable-ID reader home after all open leases drain.
// Once removal begins, new opens for that identity fail for the lifetime of this manager.
func (m *Manager) Remove(ctx context.Context, userID UserID) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return ErrManagerClosed
		}
		m.deleting[userID] = true
		entry := m.entries[userID]
		if entry != nil && entry.references > 0 {
			notify := m.notify
			m.mu.Unlock()
			select {
			case <-notify:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if entry != nil {
			delete(m.entries, userID)
		}
		m.mu.Unlock()
		if entry != nil {
			if err := errors.Join(entry.readerDB.Close(), entry.credentialsDB.Close()); err != nil {
				return fmt.Errorf("readerstore: close reader home before deletion: %w", err)
			}
		}
		usersRoot, err := m.rootHandle.OpenRoot(UsersDirectory)
		if err != nil {
			return fmt.Errorf("readerstore: anchor users root for deletion: %w", err)
		}
		defer usersRoot.Close()
		homeName := string(userID)
		quarantineName := homeName + ".deleting"
		if _, err := usersRoot.Lstat(quarantineName); errors.Is(err, os.ErrNotExist) {
			if err := m.renameHome(usersRoot, homeName, quarantineName); errors.Is(err, os.ErrNotExist) {
				return nil
			} else if err != nil {
				return fmt.Errorf("readerstore: quarantine reader home: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("readerstore: inspect quarantined reader home: %w", err)
		}
		info, err := usersRoot.Lstat(quarantineName)
		if err != nil {
			return fmt.Errorf("readerstore: inspect quarantined reader home: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidHome
		}
		if err := m.removeHome(usersRoot, quarantineName); err != nil {
			return fmt.Errorf("readerstore: remove reader home: %w", err)
		}
		if _, err := usersRoot.Lstat(quarantineName); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return fmt.Errorf("readerstore: reader home remains after deletion")
			}
			return fmt.Errorf("readerstore: verify reader home deletion: %w", err)
		}
		if _, err := usersRoot.Lstat(homeName); err == nil {
			return ErrInvalidHome
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("readerstore: verify canonical reader home absence: %w", err)
		}
		return nil
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
	if m.rootHandle != nil {
		closeErr = errors.Join(closeErr, m.rootHandle.Close())
	}
	return closeErr
}

func (h *Home) ID() UserID             { return h.entry.id }
func (h *Home) DB() *sql.DB            { return h.entry.readerDB }
func (h *Home) CredentialsDB() *sql.DB { return h.entry.credentialsDB }
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
	if err := validateHome(homePath, m.schemas); err != nil {
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

func (m *Manager) validateRootPath() error {
	pathInfo, err := os.Lstat(m.root)
	if err != nil || !pathInfo.IsDir() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidRoot
	}
	if !os.SameFile(pathInfo, pathInfo) {
		return ErrInvalidRoot
	}
	anchoredInfo, err := m.rootHandle.Stat(".")
	if err != nil || !os.SameFile(pathInfo, anchoredInfo) {
		return ErrInvalidRoot
	}
	return nil
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
