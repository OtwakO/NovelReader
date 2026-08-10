package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type RecoveryAction string

const (
	RecoveryResetExisting     RecoveryAction = "reset_existing"
	RecoveryCreateReplacement RecoveryAction = "create_replacement"
)

var (
	ErrRecoveryUnavailable   = errors.New("auth: administrator recovery is unavailable")
	ErrInvalidRecoveryAction = errors.New("auth: invalid administrator recovery action")
	ErrRecoveryTarget        = errors.New("auth: invalid administrator recovery target")
	ErrRecoveryInProgress    = errors.New("auth: administrator recovery is already in progress")
)

type recoveryClaim struct {
	action             RecoveryAction
	generation         string
	userID             readerstore.UserID
	username           string
	usernameNormalized string
	passwordHash       string
	homeProvisioning   bool
	claimedAt          int64
}

// RecoveryService owns environment-authorized Administrator password reset and replacement creation.
type RecoveryService struct {
	store            *Store
	readers          *readerstore.Manager
	recoveryToken    string
	passwords        *PasswordHasher
	randomUserID     func() (readerstore.UserID, error)
	randomGeneration func() (readerstore.UserID, error)
	afterClaim       func(recoveryClaim)
}

func NewRecoveryService(store *Store, readers *readerstore.Manager, recoveryToken string) *RecoveryService {
	return &RecoveryService{
		store:            store,
		readers:          readers,
		recoveryToken:    recoveryToken,
		passwords:        NewPasswordHasher(),
		randomUserID:     randomUserID,
		randomGeneration: randomUserID,
	}
}

// Recover resets an existing Administrator or creates a replacement Administrator with a new empty home.
func (s *RecoveryService) Recover(ctx context.Context, presentedToken string, action RecoveryAction, rawUsername, password string, now time.Time) (Account, error) {
	if s == nil || s.store == nil || s.readers == nil || !validEnvironmentToken(s.recoveryToken, presentedToken) {
		return Account{}, ErrRecoveryUnavailable
	}
	if action != RecoveryResetExisting && action != RecoveryCreateReplacement {
		return Account{}, ErrInvalidRecoveryAction
	}
	username, err := NormalizeUsername(rawUsername)
	if err != nil {
		return Account{}, err
	}
	if err := s.store.setupGuard.lock(ctx); err != nil {
		return Account{}, err
	}
	defer s.store.setupGuard.unlock()

	state, err := s.readState(ctx)
	if err != nil {
		return Account{}, err
	}
	if state.status == "claimed" {
		if state.claim.action != action || state.claim.usernameNormalized != username.Normalized {
			return Account{}, ErrRecoveryInProgress
		}
		matches, err := s.passwords.Verify(ctx, password, state.claim.passwordHash)
		if err != nil {
			return Account{}, err
		}
		if !matches {
			return Account{}, ErrRecoveryTarget
		}
		return s.completeClaim(ctx, state.claim, now.Unix())
	}
	if state.status == "completed" && state.claim.action == action {
		account, authenticateErr := NewAccountService(s.store).Authenticate(ctx, rawUsername, password)
		if authenticateErr == nil && account.Role == RoleAdmin && account.ID == state.claim.userID {
			return account, nil
		}
	}

	passwordHash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return Account{}, err
	}
	generation, err := s.randomGeneration()
	if err != nil {
		return Account{}, fmt.Errorf("auth: generate recovery claim identity: %w", err)
	}
	claim := recoveryClaim{
		action:             action,
		generation:         string(generation),
		username:           username.Display,
		usernameNormalized: username.Normalized,
		passwordHash:       passwordHash,
		claimedAt:          now.Unix(),
	}
	if action == RecoveryResetExisting {
		account, _, err := NewAccountService(s.store).accountByNormalizedUsername(ctx, username.Normalized)
		if err != nil || account.Role != RoleAdmin || account.Status == StatusDeleting {
			return Account{}, ErrRecoveryTarget
		}
		claim.userID = account.ID
	} else {
		if _, _, err := NewAccountService(s.store).accountByNormalizedUsername(ctx, username.Normalized); !errors.Is(err, ErrAccountNotFound) {
			if err == nil {
				return Account{}, ErrUsernameUnavailable
			}
			return Account{}, err
		}
		claim.userID, err = s.randomUserID()
		if err != nil {
			return Account{}, fmt.Errorf("auth: generate recovery account identity: %w", err)
		}
	}
	if err := s.writeClaim(ctx, claim); err != nil {
		return Account{}, err
	}
	if s.afterClaim != nil {
		s.afterClaim(claim)
	}
	return s.completeClaim(ctx, claim, now.Unix())
}

type recoveryState struct {
	status string
	claim  recoveryClaim
}

func (s *RecoveryService) readState(ctx context.Context) (recoveryState, error) {
	var state recoveryState
	var action, generation, rawUserID, username, normalized, passwordHash sql.NullString
	var homeProvisioning int
	var claimedAt sql.NullInt64
	if err := s.store.db.QueryRowContext(ctx, `
		SELECT status, action, generation, user_id, username, username_normalized, password_hash, home_provisioning, claimed_at
		FROM admin_recovery_state WHERE id = 1
	`).Scan(&state.status, &action, &generation, &rawUserID, &username, &normalized, &passwordHash, &homeProvisioning, &claimedAt); err != nil {
		return recoveryState{}, fmt.Errorf("auth: read administrator recovery state: %w", err)
	}
	if state.status == "idle" {
		return state, nil
	}
	if !action.Valid || !generation.Valid || !rawUserID.Valid || !claimedAt.Valid {
		return recoveryState{}, ErrInvalidSystemSchema
	}
	state.claim.action = RecoveryAction(action.String)
	if _, err := readerstore.ParseUserID(generation.String); err != nil {
		return recoveryState{}, ErrInvalidSystemSchema
	}
	state.claim.generation = generation.String
	var err error
	state.claim.userID, err = readerstore.ParseUserID(rawUserID.String)
	if err != nil {
		return recoveryState{}, ErrInvalidSystemSchema
	}
	state.claim.homeProvisioning = homeProvisioning == 1
	state.claim.claimedAt = claimedAt.Int64
	if state.status == "claimed" {
		if !username.Valid || !normalized.Valid || !passwordHash.Valid {
			return recoveryState{}, ErrInvalidSystemSchema
		}
		state.claim.username = username.String
		state.claim.usernameNormalized = normalized.String
		state.claim.passwordHash = passwordHash.String
		if err := validateRecoveryClaim(state.claim); err != nil {
			return recoveryState{}, err
		}
	}
	if state.status != "claimed" && state.status != "completed" {
		return recoveryState{}, ErrInvalidSystemSchema
	}
	return state, nil
}

func validateRecoveryClaim(claim recoveryClaim) error {
	if claim.action != RecoveryResetExisting && claim.action != RecoveryCreateReplacement {
		return ErrInvalidSystemSchema
	}
	if _, err := readerstore.ParseUserID(claim.generation); err != nil {
		return ErrInvalidSystemSchema
	}
	username, err := NormalizeUsername(claim.username)
	if err != nil || username.Display != claim.username || username.Normalized != claim.usernameNormalized {
		return ErrInvalidSystemSchema
	}
	if _, _, err := parsePasswordHash(claim.passwordHash); err != nil || claim.claimedAt < 0 {
		return ErrInvalidSystemSchema
	}
	return nil
}

func (s *RecoveryService) writeClaim(ctx context.Context, claim recoveryClaim) error {
	if err := validateRecoveryClaim(claim); err != nil {
		return err
	}
	result, err := s.store.db.ExecContext(ctx, `
		UPDATE admin_recovery_state
		SET status = 'claimed', action = ?, generation = ?, user_id = ?, username = ?, username_normalized = ?, password_hash = ?, home_provisioning = 0, claimed_at = ?, completed_at = NULL
		WHERE id = 1 AND status IN ('idle', 'completed')
	`, string(claim.action), claim.generation, string(claim.userID), claim.username, claim.usernameNormalized, claim.passwordHash, claim.claimedAt)
	if err != nil {
		return fmt.Errorf("auth: claim administrator recovery: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrRecoveryInProgress
	}
	return nil
}

func (s *RecoveryService) completeClaim(ctx context.Context, claim recoveryClaim, now int64) (Account, error) {
	if claim.action == RecoveryCreateReplacement {
		if !claim.homeProvisioning {
			exists, err := s.readers.Exists(ctx, claim.userID)
			if err != nil {
				return Account{}, err
			}
			if exists {
				return Account{}, ErrAccountAlreadyExists
			}
			result, err := s.store.db.ExecContext(ctx, `
				UPDATE admin_recovery_state SET home_provisioning = 1
				WHERE id = 1 AND status = 'claimed' AND action = 'create_replacement' AND generation = ? AND user_id = ? AND home_provisioning = 0
			`, claim.generation, string(claim.userID))
			if err != nil {
				return Account{}, fmt.Errorf("auth: authorize replacement home provisioning: %w", err)
			}
			changed, err := result.RowsAffected()
			if err != nil || changed != 1 {
				return Account{}, ErrRecoveryInProgress
			}
			claim.homeProvisioning = true
		}
		if err := s.readers.Create(ctx, claim.userID); err != nil {
			return Account{}, fmt.Errorf("auth: create replacement administrator reader home: %w", err)
		}
	}
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return Account{}, err
	}
	defer s.store.sessionGuard.unlock()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, fmt.Errorf("auth: begin administrator recovery completion: %w", err)
	}
	defer tx.Rollback()

	var status, action, generation, userID string
	if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(action, ''), COALESCE(generation, ''), COALESCE(user_id, '') FROM admin_recovery_state WHERE id = 1`).Scan(&status, &action, &generation, &userID); err != nil {
		return Account{}, fmt.Errorf("auth: verify administrator recovery claim: %w", err)
	}
	if status != "claimed" || action != string(claim.action) || generation != claim.generation || userID != string(claim.userID) {
		return Account{}, ErrRecoveryInProgress
	}
	if claim.action == RecoveryResetExisting {
		result, err := tx.ExecContext(ctx, `
			UPDATE users
			SET password_hash = ?, status = 'active', updated_at = ?, auth_version = auth_version + 1
			WHERE id = ? AND role = 'admin' AND status IN ('active', 'disabled')
		`, claim.passwordHash, now, string(claim.userID))
		if err != nil {
			return Account{}, fmt.Errorf("auth: reset administrator password: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return Account{}, ErrRecoveryTarget
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ?`, string(claim.userID)); err != nil {
			return Account{}, fmt.Errorf("auth: revoke recovered administrator sessions: %w", err)
		}
	} else {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
			VALUES (?, ?, ?, 'admin', ?, 'active', ?, ?)
		`, string(claim.userID), claim.username, claim.usernameNormalized, claim.passwordHash, now, now)
		if err != nil {
			if sqliteConstraintCode(err) == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
				return Account{}, ErrUsernameUnavailable
			}
			var sqliteErr *sqlite.Error
			if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
				return Account{}, ErrAccountAlreadyExists
			}
			return Account{}, fmt.Errorf("auth: create replacement administrator: %w", err)
		}
		if result, err := tx.ExecContext(ctx, `
			UPDATE setup_state
			SET status = 'closed', proposed_user_id = ?, username = NULL, username_normalized = NULL, password_hash = NULL,
			    claimed_at = ?, claim_expires_at = ?, closed_at = ?
			WHERE id = 1 AND status IN ('open', 'claimed')
		`, string(claim.userID), now, now+int64(defaultSetupClaimTTL.Seconds()), now); err != nil {
			return Account{}, fmt.Errorf("auth: close setup after recovery creation: %w", err)
		} else if changed, changedErr := result.RowsAffected(); changedErr != nil || changed > 1 {
			return Account{}, ErrInvalidSystemSchema
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE admin_recovery_state
		SET status = 'completed', username = NULL, username_normalized = NULL, password_hash = NULL, home_provisioning = 0, completed_at = ?
		WHERE id = 1 AND status = 'claimed' AND generation = ? AND user_id = ?
	`, now, claim.generation, string(claim.userID))
	if err != nil {
		return Account{}, fmt.Errorf("auth: complete administrator recovery: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return Account{}, ErrRecoveryInProgress
	}
	account, err := accountByID(ctx, tx, claim.userID)
	if err != nil {
		return Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("auth: commit administrator recovery: %w", err)
	}
	return account, nil
}
