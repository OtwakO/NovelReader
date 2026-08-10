package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/otwako/novelreader/internal/readerstore"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrUsernameUnavailable  = errors.New("auth: username unavailable")
	ErrAccountAlreadyExists = errors.New("auth: account already exists")
	ErrInvalidCredentials   = errors.New("auth: invalid credentials")
	ErrAccountNotActive     = errors.New("auth: account is not active")
	ErrInvalidAccountRecord = errors.New("auth: invalid stored account record")
)

type Account struct {
	ID                 readerstore.UserID
	Username           string
	UsernameNormalized string
	Role               Role
	Status             AccountStatus
	CreatedAt          int64
	UpdatedAt          int64
	AuthVersion        int64
}

// AccountService owns account credentials and persistence in system.db.
type AccountService struct {
	store     *Store
	passwords *PasswordHasher
}

func NewAccountService(store *Store) *AccountService {
	return &AccountService{store: store, passwords: NewPasswordHasher()}
}

// CreateReaderAccount creates only an ordinary reader account. Administrator creation belongs to setup/recovery workflows.
func (s *AccountService) CreateReaderAccount(ctx context.Context, userID readerstore.UserID, rawUsername, password string, now int64) (Account, error) {
	return s.createAccount(ctx, userID, rawUsername, password, RoleReader, StatusActive, now, now)
}

func (s *AccountService) createAccount(ctx context.Context, userID readerstore.UserID, rawUsername, password string, role Role, status AccountStatus, createdAt, updatedAt int64) (Account, error) {
	if err := s.store.setupGuard.readLock(ctx); err != nil {
		return Account{}, err
	}
	defer s.store.setupGuard.readUnlock()
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return Account{}, err
	}
	username, err := NormalizeUsername(rawUsername)
	if err != nil {
		return Account{}, err
	}
	var setupStatus string
	if err := s.store.db.QueryRowContext(ctx, `SELECT status FROM setup_state WHERE id = 1`).Scan(&setupStatus); err != nil {
		return Account{}, fmt.Errorf("auth: inspect setup before account creation: %w", err)
	}
	if setupStatus != "closed" {
		return Account{}, ErrSetupInProgress
	}
	passwordHash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return Account{}, err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, fmt.Errorf("auth: begin account creation: %w", err)
	}
	defer tx.Rollback()
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = ?`, string(userID)).Scan(&existing); err == nil {
		return Account{}, ErrAccountAlreadyExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Account{}, fmt.Errorf("auth: check account identity: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, string(userID), username.Display, username.Normalized, string(role), passwordHash, string(status), createdAt, updatedAt)
	if err != nil {
		if sqliteConstraintCode(err) == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return Account{}, ErrUsernameUnavailable
		}
		return Account{}, fmt.Errorf("auth: create account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("auth: commit account creation: %w", err)
	}
	return Account{
		ID:                 userID,
		Username:           username.Display,
		UsernameNormalized: username.Normalized,
		Role:               role,
		Status:             status,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
		AuthVersion:        1,
	}, nil
}

func (s *AccountService) Authenticate(ctx context.Context, rawUsername, password string) (Account, error) {
	username, err := NormalizeUsername(rawUsername)
	if err != nil {
		if dummyErr := s.passwords.DummyVerify(ctx, password); dummyErr != nil {
			return Account{}, dummyErr
		}
		return Account{}, ErrInvalidCredentials
	}

	account, passwordHash, err := s.accountByNormalizedUsername(ctx, username.Normalized)
	if errors.Is(err, ErrAccountNotFound) {
		if dummyErr := s.passwords.DummyVerify(ctx, password); dummyErr != nil {
			return Account{}, dummyErr
		}
		return Account{}, ErrInvalidCredentials
	}
	if err != nil {
		return Account{}, err
	}
	valid, err := s.passwords.Verify(ctx, password, passwordHash)
	if err != nil {
		return Account{}, err
	}
	if !valid || account.Status != StatusActive {
		return Account{}, ErrInvalidCredentials
	}
	return account, nil
}

func (s *AccountService) ReplacePassword(ctx context.Context, userID readerstore.UserID, password string, now int64) error {
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return err
	}
	if err := s.store.setupGuard.readLock(ctx); err != nil {
		return err
	}
	defer s.store.setupGuard.readUnlock()
	passwordHash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return err
	}
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return err
	}
	defer s.store.sessionGuard.unlock()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: begin password replacement: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ?, auth_version = auth_version + 1 WHERE id = ? AND status = ?
	`, passwordHash, now, string(userID), string(StatusActive))
	if err != nil {
		return fmt.Errorf("auth: replace password: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: inspect password replacement: %w", err)
	}
	if changed != 1 {
		var status AccountStatus
		if err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, string(userID)).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrAccountNotFound
			}
			return fmt.Errorf("auth: inspect account for password replacement: %w", err)
		}
		return ErrAccountNotActive
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ?`, string(userID)); err != nil {
		return fmt.Errorf("auth: revoke sessions after password replacement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth: commit password replacement: %w", err)
	}
	return nil
}

func (s *AccountService) accountByNormalizedUsername(ctx context.Context, normalized string) (Account, string, error) {
	var account Account
	var id string
	var passwordHash string
	err := s.store.db.QueryRowContext(ctx, `
		SELECT id, username, username_normalized, role, password_hash, status, created_at, updated_at, auth_version
		FROM users WHERE username_normalized = ?
	`, normalized).Scan(
		&id,
		&account.Username,
		&account.UsernameNormalized,
		&account.Role,
		&passwordHash,
		&account.Status,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.AuthVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, "", ErrAccountNotFound
	}
	if err != nil {
		return Account{}, "", fmt.Errorf("auth: read account for authentication: %w", err)
	}
	account.ID, err = readerstore.ParseUserID(id)
	if err != nil {
		return Account{}, "", fmt.Errorf("%w: account ID: %v", ErrInvalidAccountRecord, err)
	}
	if err := validateStoredAccount(account); err != nil {
		return Account{}, "", err
	}
	return account, passwordHash, nil
}

func validateStoredAccount(account Account) error {
	username, err := NormalizeUsername(account.Username)
	if err != nil || username.Display != account.Username || username.Normalized != account.UsernameNormalized {
		return fmt.Errorf("%w: username fields", ErrInvalidAccountRecord)
	}
	if account.Role != RoleReader && account.Role != RoleAdmin {
		return fmt.Errorf("%w: role %q", ErrInvalidAccountRecord, account.Role)
	}
	if account.Status != StatusActive && account.Status != StatusDisabled && account.Status != StatusDeleting {
		return fmt.Errorf("%w: status %q", ErrInvalidAccountRecord, account.Status)
	}
	if account.AuthVersion < 1 {
		return fmt.Errorf("%w: authentication version", ErrInvalidAccountRecord)
	}
	return nil
}

func accountByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID readerstore.UserID) (Account, error) {
	var account Account
	var id string
	err := queryer.QueryRowContext(ctx, `
		SELECT id, username, username_normalized, role, status, created_at, updated_at, auth_version
		FROM users WHERE id = ?
	`, string(userID)).Scan(
		&id,
		&account.Username,
		&account.UsernameNormalized,
		&account.Role,
		&account.Status,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.AuthVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: read account: %w", err)
	}
	account.ID, err = readerstore.ParseUserID(id)
	if err != nil {
		return Account{}, fmt.Errorf("%w: account ID: %v", ErrInvalidAccountRecord, err)
	}
	if err := validateStoredAccount(account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func sqliteConstraintCode(err error) int {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return 0
	}
	return sqliteErr.Code()
}
