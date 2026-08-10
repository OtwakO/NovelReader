package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const defaultSetupClaimTTL = 15 * time.Minute

var (
	ErrSetupUnavailable = errors.New("auth: initial administrator setup is unavailable")
	ErrSetupClosed      = errors.New("auth: initial administrator setup is closed")
	ErrSetupInProgress  = errors.New("auth: initial administrator setup is already claimed")
)

type setupClaim struct {
	userID             readerstore.UserID
	username           string
	usernameNormalized string
	passwordHash       string
	claimedAt          int64
	claimExpiresAt     int64
}

// SetupService owns the one-time cross-store initial Administrator workflow.
type SetupService struct {
	store          *Store
	readers        *readerstore.Manager
	bootstrapToken string
	passwords      *PasswordHasher
	claimTTL       time.Duration
	randomUserID   func() (readerstore.UserID, error)
}

func NewSetupService(store *Store, readers *readerstore.Manager, bootstrapToken string) *SetupService {
	return &SetupService{
		store:          store,
		readers:        readers,
		bootstrapToken: bootstrapToken,
		passwords:      NewPasswordHasher(),
		claimTTL:       defaultSetupClaimTTL,
		randomUserID:   randomUserID,
	}
}

// SetupStatus reports the durable one-time setup state without exposing claim details.
func (s *SetupService) SetupStatus(ctx context.Context) (string, error) {
	if s == nil || s.store == nil {
		return "", ErrSetupUnavailable
	}
	return s.setupStatus(ctx)
}

// AuthenticateInitialAdministrator validates credentials only for the Administrator that closed setup.
func (s *SetupService) AuthenticateInitialAdministrator(ctx context.Context, rawUsername, password string) (Account, error) {
	if s == nil || s.store == nil {
		return Account{}, ErrSetupUnavailable
	}
	var status, initialUserID string
	if err := s.store.db.QueryRowContext(ctx, `
		SELECT status, COALESCE(proposed_user_id, '') FROM setup_state WHERE id = 1
	`).Scan(&status, &initialUserID); err != nil {
		return Account{}, fmt.Errorf("auth: read completed setup identity: %w", err)
	}
	if status != "closed" {
		return Account{}, ErrSetupInProgress
	}
	account, err := NewAccountService(s.store).Authenticate(ctx, rawUsername, password)
	if err != nil {
		return Account{}, err
	}
	if account.Role != RoleAdmin || string(account.ID) != initialUserID {
		return Account{}, ErrInvalidCredentials
	}
	return account, nil
}

// CreateInitialAdministrator claims setup, publishes the claimed reader home, then atomically
// activates the Administrator and closes setup. A retry resumes the durable claim rather than
// accepting replacement credentials.
func (s *SetupService) CreateInitialAdministrator(ctx context.Context, presentedToken, rawUsername, password string, now time.Time) (Account, error) {
	if s == nil || s.store == nil || s.readers == nil || !validBootstrapToken(s.bootstrapToken, presentedToken) {
		return Account{}, ErrSetupUnavailable
	}
	if err := s.store.setupGuard.lock(ctx); err != nil {
		return Account{}, err
	}
	defer s.store.setupGuard.unlock()
	status, err := s.setupStatus(ctx)
	if err != nil {
		return Account{}, err
	}
	if status == "closed" {
		return Account{}, ErrSetupClosed
	}
	if status == "claimed" {
		return Account{}, ErrSetupInProgress
	}
	username, err := NormalizeUsername(rawUsername)
	if err != nil {
		return Account{}, err
	}
	passwordHash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return Account{}, err
	}
	userID, err := s.randomUserID()
	if err != nil {
		return Account{}, fmt.Errorf("auth: generate setup account identity: %w", err)
	}
	claim, created, err := s.claim(ctx, setupClaim{
		userID:             userID,
		username:           username.Display,
		usernameNormalized: username.Normalized,
		passwordHash:       passwordHash,
		claimedAt:          now.Unix(),
		claimExpiresAt:     now.Add(s.claimTTL).Unix(),
	})
	if err != nil {
		return Account{}, err
	}
	if !created {
		return Account{}, ErrSetupInProgress
	}
	if err := s.readers.Create(ctx, claim.userID); err != nil {
		return Account{}, fmt.Errorf("auth: create initial administrator reader home: %w", err)
	}
	return s.activate(ctx, claim, now.Unix())
}

// RecoverInitialAdministrator rolls the authoritative durable setup claim forward after interruption; claim expiry is advisory.
func (s *SetupService) RecoverInitialAdministrator(ctx context.Context, presentedToken string, now time.Time) (Account, error) {
	if s == nil || s.store == nil || s.readers == nil || !validBootstrapToken(s.bootstrapToken, presentedToken) {
		return Account{}, ErrSetupUnavailable
	}
	if err := s.store.setupGuard.lock(ctx); err != nil {
		return Account{}, err
	}
	defer s.store.setupGuard.unlock()
	claim, err := s.readClaim(ctx)
	if err != nil {
		return Account{}, err
	}
	if err := s.readers.Create(ctx, claim.userID); err != nil {
		return Account{}, fmt.Errorf("auth: recover initial administrator reader home: %w", err)
	}
	return s.activate(ctx, claim, now.Unix())
}

func (s *SetupService) setupStatus(ctx context.Context) (string, error) {
	var status string
	if err := s.store.db.QueryRowContext(ctx, `SELECT status FROM setup_state WHERE id = 1`).Scan(&status); err != nil {
		return "", fmt.Errorf("auth: read setup status: %w", err)
	}
	return status, nil
}

func (s *SetupService) readClaim(ctx context.Context) (setupClaim, error) {
	var claim setupClaim
	var rawUserID, username, normalized, passwordHash sql.NullString
	var claimedAt, expiresAt sql.NullInt64
	var status string
	err := s.store.db.QueryRowContext(ctx, `
		SELECT status, proposed_user_id, username, username_normalized, password_hash, claimed_at, claim_expires_at
		FROM setup_state WHERE id = 1
	`).Scan(&status, &rawUserID, &username, &normalized, &passwordHash, &claimedAt, &expiresAt)
	if err != nil {
		return setupClaim{}, fmt.Errorf("auth: read setup recovery claim: %w", err)
	}
	if status == "closed" {
		return setupClaim{}, ErrSetupClosed
	}
	if status != "claimed" || !rawUserID.Valid || !username.Valid || !normalized.Valid || !passwordHash.Valid || !claimedAt.Valid || !expiresAt.Valid {
		return setupClaim{}, ErrSetupInProgress
	}
	claim.userID, err = readerstore.ParseUserID(rawUserID.String)
	if err != nil {
		return setupClaim{}, ErrInvalidSystemSchema
	}
	claim.username = username.String
	claim.usernameNormalized = normalized.String
	claim.passwordHash = passwordHash.String
	claim.claimedAt = claimedAt.Int64
	claim.claimExpiresAt = expiresAt.Int64
	if err := validateSetupClaim(claim); err != nil {
		return setupClaim{}, err
	}
	return claim, nil
}

func validateSetupClaim(claim setupClaim) error {
	username, err := NormalizeUsername(claim.username)
	if err != nil || username.Display != claim.username || username.Normalized != claim.usernameNormalized {
		return ErrInvalidSystemSchema
	}
	if _, _, err := parsePasswordHash(claim.passwordHash); err != nil {
		return ErrInvalidSystemSchema
	}
	if claim.claimedAt < 0 || claim.claimExpiresAt <= claim.claimedAt {
		return ErrInvalidSystemSchema
	}
	return nil
}

func (s *SetupService) claim(ctx context.Context, proposed setupClaim) (setupClaim, bool, error) {
	if err := validateSetupClaim(proposed); err != nil {
		return setupClaim{}, false, err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return setupClaim{}, false, fmt.Errorf("auth: begin initial administrator claim: %w", err)
	}
	defer tx.Rollback()

	var accountCount int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&accountCount); err != nil {
		return setupClaim{}, false, fmt.Errorf("auth: inspect existing accounts: %w", err)
	}
	if accountCount != 0 {
		return setupClaim{}, false, ErrSetupClosed
	}

	var status string
	var stored setupClaim
	var rawUserID sql.NullString
	var username, normalized, passwordHash sql.NullString
	var claimedAt, expiresAt sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, proposed_user_id, username, username_normalized, password_hash, claimed_at, claim_expires_at
		FROM setup_state WHERE id = 1
	`).Scan(&status, &rawUserID, &username, &normalized, &passwordHash, &claimedAt, &expiresAt); err != nil {
		return setupClaim{}, false, fmt.Errorf("auth: read setup claim: %w", err)
	}
	switch status {
	case "closed":
		return setupClaim{}, false, ErrSetupClosed
	case "claimed":
		stored.userID, err = readerstore.ParseUserID(rawUserID.String)
		if err != nil || !username.Valid || !normalized.Valid || !passwordHash.Valid || !claimedAt.Valid || !expiresAt.Valid {
			return setupClaim{}, false, ErrInvalidSystemSchema
		}
		stored.username = username.String
		stored.usernameNormalized = normalized.String
		stored.passwordHash = passwordHash.String
		stored.claimedAt = claimedAt.Int64
		stored.claimExpiresAt = expiresAt.Int64
		if err := validateSetupClaim(stored); err != nil {
			return setupClaim{}, false, err
		}
		return stored, false, nil
	case "open":
		result, err := tx.ExecContext(ctx, `
			UPDATE setup_state
			SET status = 'claimed', proposed_user_id = ?, username = ?, username_normalized = ?, password_hash = ?, claimed_at = ?, claim_expires_at = ?
			WHERE id = 1 AND status = 'open'
		`, string(proposed.userID), proposed.username, proposed.usernameNormalized, proposed.passwordHash, proposed.claimedAt, proposed.claimExpiresAt)
		if err != nil {
			return setupClaim{}, false, fmt.Errorf("auth: claim initial administrator setup: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return setupClaim{}, false, ErrSetupInProgress
		}
		if err := tx.Commit(); err != nil {
			return setupClaim{}, false, fmt.Errorf("auth: commit initial administrator claim: %w", err)
		}
		return proposed, true, nil
	default:
		return setupClaim{}, false, ErrInvalidSystemSchema
	}
}

func (s *SetupService) activate(ctx context.Context, claim setupClaim, now int64) (Account, error) {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, fmt.Errorf("auth: begin initial administrator activation: %w", err)
	}
	defer tx.Rollback()

	var status, proposedUserID string
	if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(proposed_user_id, '') FROM setup_state WHERE id = 1`).Scan(&status, &proposedUserID); err != nil {
		return Account{}, fmt.Errorf("auth: verify initial administrator claim: %w", err)
	}
	if status == "closed" {
		account, err := accountByID(ctx, tx, claim.userID)
		if err == nil && account.Role == RoleAdmin {
			return account, nil
		}
		return Account{}, ErrSetupClosed
	}
	if status != "claimed" || proposedUserID != string(claim.userID) {
		return Account{}, ErrSetupInProgress
	}
	var accountCount int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&accountCount); err != nil {
		return Account{}, fmt.Errorf("auth: inspect accounts before initial administrator activation: %w", err)
	}
	if accountCount != 0 {
		return Account{}, ErrInvalidSystemSchema
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, string(claim.userID), claim.username, claim.usernameNormalized, string(RoleAdmin), claim.passwordHash, string(StatusActive), now, now)
	if err != nil {
		if sqliteConstraintCode(err) == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return Account{}, ErrUsernameUnavailable
		}
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
			return Account{}, ErrAccountAlreadyExists
		}
		return Account{}, fmt.Errorf("auth: activate initial administrator: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE setup_state
		SET status = 'closed', username = NULL, username_normalized = NULL, password_hash = NULL, closed_at = ?
		WHERE id = 1 AND status = 'claimed' AND proposed_user_id = ?
	`, now, string(claim.userID))
	if err != nil {
		return Account{}, fmt.Errorf("auth: close initial administrator setup: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return Account{}, ErrSetupInProgress
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("auth: commit initial administrator activation: %w", err)
	}
	return Account{
		ID:                 claim.userID,
		Username:           claim.username,
		UsernameNormalized: claim.usernameNormalized,
		Role:               RoleAdmin,
		Status:             StatusActive,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func validBootstrapToken(configured, presented string) bool {
	if configured == "" || presented == "" {
		return false
	}
	configuredHash := sha256.Sum256([]byte(configured))
	presentedHash := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(configuredHash[:], presentedHash[:]) == 1
}

func randomUserID() (readerstore.UserID, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return readerstore.ParseUserID(fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]))
}
