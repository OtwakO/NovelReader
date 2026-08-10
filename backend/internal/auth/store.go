// Package auth owns deployment identity and authentication control storage.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	SystemDatabaseName         = "system.db"
	CurrentSystemSchemaVersion = 3
	systemDatabaseStagingName  = ".system.db.staging"
)

type Role string

const (
	RoleReader Role = "reader"
	RoleAdmin  Role = "admin"
)

type AccountStatus string

const (
	StatusActive   AccountStatus = "active"
	StatusDisabled AccountStatus = "disabled"
	StatusDeleting AccountStatus = "deleting"
)

var (
	ErrNewerSystemSchema       = errors.New("auth: system database schema is newer than supported")
	ErrInvalidSystemSchema     = errors.New("auth: system database schema is invalid")
	ErrInvalidStatusTransition = errors.New("auth: invalid account status transition")
	ErrAccountNotFound         = errors.New("auth: account not found")

	sessionGuards = struct {
		sync.Mutex
		byPath map[string]*sharedSessionGuard
	}{byPath: make(map[string]*sharedSessionGuard)}
	setupGuards = struct {
		sync.Mutex
		byPath map[string]*sharedSessionGuard
	}{byPath: make(map[string]*sharedSessionGuard)}
)

type sharedSessionGuard struct {
	mutex          sync.Mutex
	changed        chan struct{}
	readers        int
	writer         bool
	waitingWriters int
}

func newSharedSessionGuard() *sharedSessionGuard {
	return &sharedSessionGuard{changed: make(chan struct{})}
}

func (g *sharedSessionGuard) lock(ctx context.Context) error {
	g.mutex.Lock()
	g.waitingWriters++
	for g.writer || g.readers > 0 {
		changed := g.changed
		g.mutex.Unlock()
		select {
		case <-ctx.Done():
			g.mutex.Lock()
			g.waitingWriters--
			g.notifyLocked()
			g.mutex.Unlock()
			return ctx.Err()
		case <-changed:
			g.mutex.Lock()
		}
	}
	g.waitingWriters--
	g.writer = true
	g.mutex.Unlock()
	return nil
}

func (g *sharedSessionGuard) unlock() {
	g.mutex.Lock()
	g.writer = false
	g.notifyLocked()
	g.mutex.Unlock()
}

func (g *sharedSessionGuard) readLock(ctx context.Context) error {
	g.mutex.Lock()
	for g.writer || g.waitingWriters > 0 {
		changed := g.changed
		g.mutex.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
			g.mutex.Lock()
		}
	}
	g.readers++
	g.mutex.Unlock()
	return nil
}

func (g *sharedSessionGuard) readUnlock() {
	g.mutex.Lock()
	g.readers--
	if g.readers == 0 {
		g.notifyLocked()
	}
	g.mutex.Unlock()
}

func (g *sharedSessionGuard) notifyLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

// Store owns system.db and deployment-level identity/control records.
type Store struct {
	db           *sql.DB
	path         string
	sessionGuard *sharedSessionGuard
	setupGuard   *sharedSessionGuard
	closeOnce    sync.Once
	closeErr     error
}

// OpenSystemStore initializes or validates the fixed system.db inside a prepared data root.
func OpenSystemStore(root string) (*Store, error) {
	state, err := readerstore.InspectRoot(root)
	if err != nil {
		return nil, fmt.Errorf("auth: inspect data root: %w", err)
	}
	if state != readerstore.RootCurrent {
		return nil, fmt.Errorf("auth: data root state %q", state)
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("auth: resolve data root: %w", err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("auth: canonicalize data root: %w", err)
	}
	path := filepath.Join(canonicalRoot, SystemDatabaseName)
	if err := ensureSystemDatabase(filepath.Join(canonicalRoot, systemDatabaseStagingName), path); err != nil {
		return nil, err
	}

	db, err := openSystemDatabase(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, path: path, sessionGuard: acquireSharedGuard(&sessionGuards, path), setupGuard: acquireSharedGuard(&setupGuards, path)}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.db != nil {
			s.closeErr = s.db.Close()
		}
	})
	return s.closeErr
}

// TransitionAccountStatus applies only legal state changes and revokes sessions when leaving active status.
func (s *Store) TransitionAccountStatus(userID readerstore.UserID, next AccountStatus, updatedAt int64) error {
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return err
	}
	if err := s.sessionGuard.lock(context.Background()); err != nil {
		return err
	}
	defer s.sessionGuard.unlock()
	if next != StatusActive && next != StatusDisabled && next != StatusDeleting {
		return ErrInvalidStatusTransition
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("auth: begin account status transition: %w", err)
	}
	defer tx.Rollback()
	var current AccountStatus
	if err := tx.QueryRow(`SELECT status FROM users WHERE id = ?`, string(userID)).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccountNotFound
		}
		return fmt.Errorf("auth: read account status: %w", err)
	}
	if !legalStatusTransition(current, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStatusTransition, current, next)
	}
	result, err := tx.Exec(`UPDATE users SET status = ?, updated_at = ? WHERE id = ? AND status = ?`, string(next), updatedAt, string(userID), string(current))
	if err != nil {
		return fmt.Errorf("auth: update account status: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: inspect account status update: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("auth: account status changed concurrently")
	}
	if current == StatusActive && next != StatusActive {
		if _, err := tx.Exec(`DELETE FROM auth_sessions WHERE user_id = ?`, string(userID)); err != nil {
			return fmt.Errorf("auth: revoke sessions after account status change: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth: commit account status transition: %w", err)
	}
	return nil
}

func acquireSharedGuard(registry *struct {
	sync.Mutex
	byPath map[string]*sharedSessionGuard
}, path string) *sharedSessionGuard {
	registry.Lock()
	defer registry.Unlock()
	guard := registry.byPath[path]
	if guard == nil {
		guard = newSharedSessionGuard()
		registry.byPath[path] = guard
	}
	return guard
}

func legalStatusTransition(current, next AccountStatus) bool {
	switch current {
	case StatusActive:
		return next == StatusDisabled || next == StatusDeleting
	case StatusDisabled:
		return next == StatusActive || next == StatusDeleting
	default:
		return false
	}
}
