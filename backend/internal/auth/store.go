// Package auth owns deployment identity and authentication control storage.
package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	SystemDatabaseName         = "system.db"
	CurrentSystemSchemaVersion = 1
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
)

// Store owns system.db and deployment-level identity/control records.
type Store struct {
	db   *sql.DB
	path string
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
	path := filepath.Join(absoluteRoot, SystemDatabaseName)
	if err := ensureSystemDatabase(filepath.Join(absoluteRoot, systemDatabaseStagingName), path); err != nil {
		return nil, err
	}

	db, err := openSystemDatabase(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// TransitionAccountStatus applies only the legal active/disabled/deleting state changes.
func (s *Store) TransitionAccountStatus(userID readerstore.UserID, next AccountStatus, updatedAt int64) error {
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return err
	}
	if next != StatusActive && next != StatusDisabled && next != StatusDeleting {
		return ErrInvalidStatusTransition
	}

	var current AccountStatus
	if err := s.db.QueryRow(`SELECT status FROM users WHERE id = ?`, string(userID)).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccountNotFound
		}
		return fmt.Errorf("auth: read account status: %w", err)
	}
	if !legalStatusTransition(current, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStatusTransition, current, next)
	}
	result, err := s.db.Exec(`UPDATE users SET status = ?, updated_at = ? WHERE id = ? AND status = ?`, string(next), updatedAt, string(userID), string(current))
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
	return nil
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
