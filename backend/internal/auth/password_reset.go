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
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	passwordResetTokenBytes = 32
	defaultPasswordResetTTL = 30 * time.Minute
)

var ErrInvalidPasswordReset = errors.New("auth: invalid password reset")

type PasswordResetCredential struct {
	Token     string
	ExpiresAt int64
}

// PasswordResetService owns Administrator-issued, hash-only ordinary-reader reset tokens.
type PasswordResetService struct {
	store     *Store
	passwords *PasswordHasher
	random    io.Reader
	now       func() int64
}

func NewPasswordResetService(store *Store) *PasswordResetService {
	return &PasswordResetService{store: store, passwords: NewPasswordHasher(), random: rand.Reader, now: func() int64 { return time.Now().Unix() }}
}

// Issue invalidates prior unused tokens and returns the new plaintext credential exactly once.
func (s *PasswordResetService) Issue(ctx context.Context, userID readerstore.UserID, issuer Account, now int64) (PasswordResetCredential, error) {
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return PasswordResetCredential{}, err
	}
	if _, err := readerstore.ParseUserID(string(issuer.ID)); err != nil {
		return PasswordResetCredential{}, err
	}
	token, err := randomBase64URL(s.random, passwordResetTokenBytes)
	if err != nil {
		return PasswordResetCredential{}, fmt.Errorf("auth: generate password reset token: %w", err)
	}
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := now + int64(defaultPasswordResetTTL.Seconds())
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return PasswordResetCredential{}, fmt.Errorf("auth: begin password reset issuance: %w", err)
	}
	defer tx.Rollback()
	var issuerUsername string
	var issuerRole Role
	var issuerStatus AccountStatus
	if err := tx.QueryRowContext(ctx, `SELECT username, role, status FROM users WHERE id = ?`, string(issuer.ID)).Scan(&issuerUsername, &issuerRole, &issuerStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasswordResetCredential{}, ErrProtectedAccount
		}
		return PasswordResetCredential{}, fmt.Errorf("auth: read password reset issuer: %w", err)
	}
	if issuerRole != RoleAdmin || issuerStatus != StatusActive {
		return PasswordResetCredential{}, ErrProtectedAccount
	}
	var targetRole Role
	var targetStatus AccountStatus
	if err := tx.QueryRowContext(ctx, `SELECT role, status FROM users WHERE id = ?`, string(userID)).Scan(&targetRole, &targetStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasswordResetCredential{}, ErrAccountNotFound
		}
		return PasswordResetCredential{}, fmt.Errorf("auth: read password reset target: %w", err)
	}
	if targetRole != RoleReader {
		return PasswordResetCredential{}, ErrProtectedAccount
	}
	if targetStatus != StatusActive && targetStatus != StatusDisabled {
		return PasswordResetCredential{}, ErrAccountNotActive
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ? AND used_at IS NULL`, string(userID)); err != nil {
		return PasswordResetCredential{}, fmt.Errorf("auth: invalidate prior password resets: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (token_hash, user_id, created_by_user_id, created_by_username, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tokenHash[:], string(userID), string(issuer.ID), issuerUsername, now, expiresAt); err != nil {
		return PasswordResetCredential{}, fmt.Errorf("auth: store password reset token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return PasswordResetCredential{}, fmt.Errorf("auth: commit password reset issuance: %w", err)
	}
	return PasswordResetCredential{Token: token, ExpiresAt: expiresAt}, nil
}

// Complete atomically consumes a valid token, replaces the password, and revokes all sessions.
func (s *PasswordResetService) Complete(ctx context.Context, token, newPassword string) error {
	tokenHash, tokenWellFormed := passwordResetTokenHash(token)
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	var exists int
	lookupErr := sql.ErrNoRows
	if tokenWellFormed {
		lookupErr = s.store.db.QueryRowContext(ctx, `
		SELECT 1 FROM password_reset_tokens
		JOIN users ON users.id = password_reset_tokens.user_id
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		  AND users.role = 'reader' AND users.status IN ('active', 'disabled')
		`, tokenHash[:], s.now()).Scan(&exists)
	}
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return fmt.Errorf("auth: inspect password reset token: %w", lookupErr)
	}
	passwordHash, err := s.passwords.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	if errors.Is(lookupErr, sql.ErrNoRows) {
		return ErrInvalidPasswordReset
	}
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return err
	}
	defer s.store.sessionGuard.unlock()
	completionTime := s.now()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: begin password reset completion: %w", err)
	}
	defer tx.Rollback()
	var userID string
	var authVersion int64
	if err := tx.QueryRowContext(ctx, `
		SELECT users.id, users.auth_version
		FROM password_reset_tokens
		JOIN users ON users.id = password_reset_tokens.user_id
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		  AND users.role = 'reader' AND users.status IN ('active', 'disabled')
	`, tokenHash[:], completionTime).Scan(&userID, &authVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidPasswordReset
		}
		return fmt.Errorf("auth: lock password reset token: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE password_reset_tokens SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`, completionTime, tokenHash[:], completionTime)
	if err != nil {
		return fmt.Errorf("auth: consume password reset token: %w", err)
	}
	if changed, changedErr := result.RowsAffected(); changedErr != nil || changed != 1 {
		return ErrInvalidPasswordReset
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ?, auth_version = auth_version + 1
		WHERE id = ? AND role = 'reader' AND status IN ('active', 'disabled') AND auth_version = ?
	`, passwordHash, completionTime, userID, authVersion)
	if err != nil {
		return fmt.Errorf("auth: replace reset password: %w", err)
	}
	if changed, changedErr := result.RowsAffected(); changedErr != nil || changed != 1 {
		return ErrInvalidPasswordReset
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("auth: revoke sessions after password reset: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth: commit password reset completion: %w", err)
	}
	return nil
}

func passwordResetTokenHash(token string) ([sha256.Size]byte, bool) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(decoded) != passwordResetTokenBytes {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256([]byte(token)), true
}
