package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	sessionTokenBytes                 = 32
	sessionIDBytes                    = 16
	sessionActivityUpdateIntervalSecs = 60 * 60
)

var ErrInvalidSession = errors.New("auth: invalid session")

// SessionCredential is returned once when a persistent browser session is created.
type SessionCredential struct {
	ID    string
	Token string
}

// SessionService owns persistent hash-only browser sessions in system.db.
type SessionService struct {
	store       *Store
	random      io.Reader
	afterLookup func()
}

func NewSessionService(store *Store) *SessionService {
	return &SessionService{store: store, random: rand.Reader}
}

func (s *SessionService) Create(ctx context.Context, userID readerstore.UserID, now int64) (SessionCredential, error) {
	return s.create(ctx, userID, 0, now)
}

// CreateAuthenticated creates a session only while the account credential version still matches
// the authentication result. Password recovery can therefore revoke an in-flight old-password login.
func (s *SessionService) CreateAuthenticated(ctx context.Context, account Account, now int64) (SessionCredential, error) {
	if account.AuthVersion < 1 {
		return SessionCredential{}, ErrInvalidAccountRecord
	}
	return s.create(ctx, account.ID, account.AuthVersion, now)
}

func (s *SessionService) create(ctx context.Context, userID readerstore.UserID, expectedAuthVersion, now int64) (SessionCredential, error) {
	if err := ctx.Err(); err != nil {
		return SessionCredential{}, err
	}
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return SessionCredential{}, err
	}
	sessionID, err := randomBase64URL(s.random, sessionIDBytes)
	if err != nil {
		return SessionCredential{}, fmt.Errorf("auth: generate session ID: %w", err)
	}
	token, err := randomBase64URL(s.random, sessionTokenBytes)
	if err != nil {
		return SessionCredential{}, fmt.Errorf("auth: generate session token: %w", err)
	}
	tokenHash := sha256.Sum256([]byte(token))

	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return SessionCredential{}, err
	}
	defer s.store.sessionGuard.unlock()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionCredential{}, fmt.Errorf("auth: begin session creation: %w", err)
	}
	defer tx.Rollback()
	account, err := accountByID(ctx, tx, userID)
	if err != nil {
		return SessionCredential{}, err
	}
	if account.Status != StatusActive {
		return SessionCredential{}, ErrAccountNotActive
	}
	if expectedAuthVersion != 0 && account.AuthVersion != expectedAuthVersion {
		return SessionCredential{}, ErrInvalidCredentials
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_sessions (id, user_id, token_hash, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
	`, sessionID, string(userID), tokenHash[:], now, now); err != nil {
		return SessionCredential{}, fmt.Errorf("auth: create session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SessionCredential{}, fmt.Errorf("auth: commit session creation: %w", err)
	}
	return SessionCredential{ID: sessionID, Token: token}, nil
}

func (s *SessionService) Authenticate(ctx context.Context, token string, now int64) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}
	tokenHash, ok := sessionTokenHash(token)
	if !ok {
		return Account{}, ErrInvalidSession
	}
	if err := s.store.sessionGuard.readLock(ctx); err != nil {
		return Account{}, err
	}
	defer s.store.sessionGuard.readUnlock()

	var account Account
	var id string
	var lastSeenAt int64
	err := s.store.db.QueryRowContext(ctx, `
		SELECT users.id, users.username, users.username_normalized, users.role, users.status, users.created_at, users.updated_at,
		       users.auth_version, auth_sessions.last_seen_at
		FROM auth_sessions
		JOIN users ON users.id = auth_sessions.user_id
		WHERE auth_sessions.token_hash = ?
	`, tokenHash[:]).Scan(
		&id,
		&account.Username,
		&account.UsernameNormalized,
		&account.Role,
		&account.Status,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.AuthVersion,
		&lastSeenAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrInvalidSession
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: read session identity: %w", err)
	}
	account.ID, err = readerstore.ParseUserID(id)
	if err != nil {
		return Account{}, fmt.Errorf("%w: account ID: %v", ErrInvalidAccountRecord, err)
	}
	if err := validateStoredAccount(account); err != nil {
		return Account{}, err
	}
	if account.Status != StatusActive {
		return Account{}, ErrInvalidSession
	}
	if s.afterLookup != nil {
		s.afterLookup()
	}
	if now-lastSeenAt >= sessionActivityUpdateIntervalSecs {
		if _, err := s.store.db.ExecContext(ctx, `
			UPDATE auth_sessions SET last_seen_at = max(last_seen_at, ?) WHERE token_hash = ?
		`, now, tokenHash[:]); err != nil {
			return Account{}, fmt.Errorf("auth: update session activity: %w", err)
		}
	}
	return account, nil
}

// Logout removes only the presented browser session. Invalid or already-removed tokens are idempotent.
func (s *SessionService) Logout(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tokenHash, ok := sessionTokenHash(token)
	if !ok {
		return nil
	}
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return err
	}
	defer s.store.sessionGuard.unlock()
	if _, err := s.store.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token_hash = ?`, tokenHash[:]); err != nil {
		return fmt.Errorf("auth: logout session: %w", err)
	}
	return nil
}

func (s *SessionService) LogoutAll(ctx context.Context, userID readerstore.UserID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return err
	}
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return err
	}
	defer s.store.sessionGuard.unlock()
	if _, err := s.store.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ?`, string(userID)); err != nil {
		return fmt.Errorf("auth: logout all sessions: %w", err)
	}
	return nil
}

func randomBase64URL(random io.Reader, size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func sessionTokenHash(token string) ([sha256.Size]byte, bool) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(decoded) != sessionTokenBytes {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256([]byte(token)), true
}
