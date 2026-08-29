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
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	backupTokenBytes  = 32
	backupTokenPrefix = "nr_backup_"
)

type BackupScope string

const (
	BackupExport  BackupScope = "backup:export"
	BackupRestore BackupScope = "backup:restore"
)

var (
	ErrInvalidBackupToken = errors.New("auth: invalid backup token")
	ErrInvalidBackupScope = errors.New("auth: invalid backup token scope")
)

type BackupToken struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CanExport  bool   `json:"canExport"`
	CanRestore bool   `json:"canRestore"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  *int64 `json:"expiresAt,omitempty"`
	LastUsedAt *int64 `json:"lastUsedAt,omitempty"`
}

type BackupTokenCredential struct {
	BackupToken
	Token string `json:"token"`
}

type CreateBackupToken struct {
	Name       string
	CanExport  bool
	CanRestore bool
	ExpiresAt  *int64
}

// BackupTokenService owns reader-scoped hash-only automation credentials.
type BackupTokenService struct {
	store  *Store
	random io.Reader
	now    func() time.Time
}

func NewBackupTokenService(store *Store) *BackupTokenService {
	return &BackupTokenService{store: store, random: rand.Reader, now: time.Now}
}

func (s *BackupTokenService) Create(ctx context.Context, userID readerstore.UserID, input CreateBackupToken) (BackupTokenCredential, error) {
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return BackupTokenCredential{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 80 || !input.CanExport && !input.CanRestore {
		return BackupTokenCredential{}, ErrInvalidBackupToken
	}
	now := s.now().Unix()
	if input.ExpiresAt != nil && *input.ExpiresAt <= now {
		return BackupTokenCredential{}, ErrInvalidBackupToken
	}
	id, err := randomBase64URL(s.random, sessionIDBytes)
	if err != nil {
		return BackupTokenCredential{}, fmt.Errorf("auth: generate backup token ID: %w", err)
	}
	secret, err := randomBase64URL(s.random, backupTokenBytes)
	if err != nil {
		return BackupTokenCredential{}, fmt.Errorf("auth: generate backup token: %w", err)
	}
	token := backupTokenPrefix + secret
	hash := sha256.Sum256([]byte(token))
	if _, err := s.store.db.ExecContext(ctx, `
		INSERT INTO backup_tokens (id, user_id, name, token_hash, can_export, can_restore, created_at, expires_at)
		SELECT ?, id, ?, ?, ?, ?, ?, ? FROM users WHERE id = ? AND status = ?
	`, id, name, hash[:], input.CanExport, input.CanRestore, now, input.ExpiresAt, string(userID), string(StatusActive)); err != nil {
		return BackupTokenCredential{}, fmt.Errorf("auth: create backup token: %w", err)
	}
	var exists int
	if err := s.store.db.QueryRowContext(ctx, `SELECT 1 FROM backup_tokens WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BackupTokenCredential{}, ErrAccountNotActive
		}
		return BackupTokenCredential{}, err
	}
	return BackupTokenCredential{BackupToken: BackupToken{ID: id, Name: name, CanExport: input.CanExport, CanRestore: input.CanRestore, CreatedAt: now, ExpiresAt: input.ExpiresAt}, Token: token}, nil
}

func (s *BackupTokenService) List(ctx context.Context, userID readerstore.UserID) ([]BackupToken, error) {
	rows, err := s.store.db.QueryContext(ctx, `
		SELECT id, name, can_export, can_restore, created_at, expires_at, last_used_at
		FROM backup_tokens WHERE user_id = ? ORDER BY created_at DESC, id
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("auth: list backup tokens: %w", err)
	}
	defer rows.Close()
	var tokens []BackupToken
	for rows.Next() {
		var token BackupToken
		var expiresAt, lastUsedAt sql.NullInt64
		if err := rows.Scan(&token.ID, &token.Name, &token.CanExport, &token.CanRestore, &token.CreatedAt, &expiresAt, &lastUsedAt); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			value := expiresAt.Int64
			token.ExpiresAt = &value
		}
		if lastUsedAt.Valid {
			value := lastUsedAt.Int64
			token.LastUsedAt = &value
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (s *BackupTokenService) Revoke(ctx context.Context, userID readerstore.UserID, tokenID string) error {
	if tokenID == "" {
		return ErrInvalidBackupToken
	}
	_, err := s.store.db.ExecContext(ctx, `DELETE FROM backup_tokens WHERE id = ? AND user_id = ?`, tokenID, string(userID))
	if err != nil {
		return fmt.Errorf("auth: revoke backup token: %w", err)
	}
	return nil
}

func (s *BackupTokenService) Authenticate(ctx context.Context, rawToken string, scope BackupScope) (Account, error) {
	if scope != BackupExport && scope != BackupRestore {
		return Account{}, ErrInvalidBackupScope
	}
	if !validBackupToken(rawToken) {
		return Account{}, ErrInvalidBackupToken
	}
	hash := sha256.Sum256([]byte(rawToken))
	now := s.now().Unix()
	scopeColumn := "can_export"
	if scope == BackupRestore {
		scopeColumn = "can_restore"
	}
	var account Account
	var id string
	err := s.store.db.QueryRowContext(ctx, `
		SELECT users.id, users.username, users.username_normalized, users.role, users.status,
		       users.created_at, users.updated_at, users.auth_version
		FROM backup_tokens JOIN users ON users.id = backup_tokens.user_id
		WHERE backup_tokens.token_hash = ? AND `+scopeColumn+` = 1
		  AND (backup_tokens.expires_at IS NULL OR backup_tokens.expires_at > ?)
	`, hash[:], now).Scan(&id, &account.Username, &account.UsernameNormalized, &account.Role, &account.Status, &account.CreatedAt, &account.UpdatedAt, &account.AuthVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrInvalidBackupToken
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: authenticate backup token: %w", err)
	}
	account.ID, err = readerstore.ParseUserID(id)
	if err != nil || validateStoredAccount(account) != nil || account.Status != StatusActive {
		return Account{}, ErrInvalidBackupToken
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE backup_tokens SET last_used_at = max(COALESCE(last_used_at, 0), ?) WHERE token_hash = ?`, now, hash[:]); err != nil {
		return Account{}, fmt.Errorf("auth: update backup token activity: %w", err)
	}
	return account, nil
}

func validBackupToken(token string) bool {
	if !strings.HasPrefix(token, backupTokenPrefix) {
		return false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(token, backupTokenPrefix))
	return err == nil && len(decoded) == backupTokenBytes
}
