package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/otwako/novelreader/internal/readerstore"
)

var ErrUsernameConfirmation = errors.New("auth: username confirmation does not match")

type DeletionJob struct {
	ID          string
	UserID      readerstore.UserID
	Status      string
	LastError   string
	CreatedAt   int64
	UpdatedAt   int64
	CompletedAt *int64
}

// DeletionService owns durable, roll-forward ordinary-reader removal.
type DeletionService struct {
	store    *Store
	readers  *readerstore.Manager
	quiesce  func(context.Context, readerstore.UserID) error
	randomID func() (string, error)
	mutex    sync.Mutex
}

func NewDeletionService(store *Store, readers *readerstore.Manager, quiesce func(context.Context, readerstore.UserID) error) *DeletionService {
	return &DeletionService{store: store, readers: readers, quiesce: quiesce, randomID: randomDeletionID}
}

// Delete starts or resumes deletion and returns only after the durable job is complete.
func (s *DeletionService) Delete(ctx context.Context, userID readerstore.UserID, confirmedUsername string, issuer Account, now int64) (DeletionJob, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.readers == nil || s.quiesce == nil {
		return DeletionJob{}, errors.New("auth: account deletion unavailable")
	}
	if _, err := readerstore.ParseUserID(string(userID)); err != nil {
		return DeletionJob{}, err
	}
	job, err := s.ensureJob(ctx, userID, confirmedUsername, issuer, now)
	if err != nil {
		return DeletionJob{}, err
	}
	if job.Status == "complete" {
		return job, nil
	}
	if err := s.advanceJob(ctx, job, now); err != nil {
		_ = s.recordFailure(context.Background(), job.ID, "deletion step failed; retry required", now)
		return DeletionJob{}, err
	}
	return s.jobByUserID(ctx, userID)
}

func (s *DeletionService) ensureJob(ctx context.Context, userID readerstore.UserID, confirmedUsername string, issuer Account, now int64) (DeletionJob, error) {
	if err := s.store.setupGuard.readLock(ctx); err != nil {
		return DeletionJob{}, err
	}
	defer s.store.setupGuard.readUnlock()
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return DeletionJob{}, err
	}
	defer s.store.sessionGuard.unlock()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return DeletionJob{}, fmt.Errorf("auth: begin account deletion: %w", err)
	}
	defer tx.Rollback()
	var issuerRole Role
	var issuerStatus AccountStatus
	if err := tx.QueryRowContext(ctx, `SELECT role, status FROM users WHERE id = ?`, string(issuer.ID)).Scan(&issuerRole, &issuerStatus); err != nil || issuerRole != RoleAdmin || issuerStatus != StatusActive {
		return DeletionJob{}, ErrProtectedAccount
	}
	if job, err := deletionJobByUserID(ctx, tx, userID); err == nil {
		if err := tx.Commit(); err != nil {
			return DeletionJob{}, fmt.Errorf("auth: commit existing deletion job: %w", err)
		}
		return job, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return DeletionJob{}, err
	}
	var username string
	var role Role
	var status AccountStatus
	if err := tx.QueryRowContext(ctx, `SELECT username, role, status FROM users WHERE id = ?`, string(userID)).Scan(&username, &role, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeletionJob{}, ErrAccountNotFound
		}
		return DeletionJob{}, fmt.Errorf("auth: read deletion target: %w", err)
	}
	if role != RoleReader {
		return DeletionJob{}, ErrProtectedAccount
	}
	if confirmedUsername != username {
		return DeletionJob{}, ErrUsernameConfirmation
	}
	if status != StatusActive && status != StatusDisabled {
		return DeletionJob{}, ErrInvalidStatusTransition
	}
	jobID, err := s.randomID()
	if err != nil {
		return DeletionJob{}, fmt.Errorf("auth: generate deletion job ID: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users SET status = 'deleting', updated_at = ?, auth_version = auth_version + 1
		WHERE id = ? AND role = 'reader' AND status = ?
	`, now, string(userID), string(status))
	if err != nil {
		return DeletionJob{}, fmt.Errorf("auth: mark account deleting: %w", err)
	}
	if changed, changedErr := result.RowsAffected(); changedErr != nil || changed != 1 {
		return DeletionJob{}, fmt.Errorf("auth: deletion target changed concurrently")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ?`, string(userID)); err != nil {
		return DeletionJob{}, fmt.Errorf("auth: revoke sessions for deletion: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_deletions (id, user_id, status, created_at, updated_at)
		VALUES (?, ?, 'pending', ?, ?)
	`, jobID, string(userID), now, now); err != nil {
		return DeletionJob{}, fmt.Errorf("auth: create deletion job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DeletionJob{}, fmt.Errorf("auth: commit account deletion: %w", err)
	}
	return DeletionJob{ID: jobID, UserID: userID, Status: "pending", CreatedAt: now, UpdatedAt: now}, nil
}

func (s *DeletionService) advanceJob(ctx context.Context, job DeletionJob, now int64) error {
	if err := s.updateJob(ctx, job.ID, "removing_data", "", now); err != nil {
		return err
	}
	if err := s.quiesce(ctx, job.UserID); err != nil {
		return fmt.Errorf("auth: quiesce reader runtime: %w", err)
	}
	if err := s.readers.Remove(ctx, job.UserID); err != nil {
		return fmt.Errorf("auth: remove reader home: %w", err)
	}
	if err := s.updateJob(ctx, job.ID, "removing_account", "", now); err != nil {
		return err
	}
	return s.removeAccountAndComplete(ctx, job, now)
}

func (s *DeletionService) removeAccountAndComplete(ctx context.Context, job DeletionJob, now int64) error {
	if err := s.store.sessionGuard.lock(ctx); err != nil {
		return err
	}
	defer s.store.sessionGuard.unlock()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: begin deletion completion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND role = 'reader' AND status = 'deleting'`, string(job.UserID))
	if err != nil {
		return fmt.Errorf("auth: remove deleted account: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: inspect deleted account removal: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("auth: deleting reader account is unavailable")
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE account_deletions SET status = 'complete', last_error = NULL, updated_at = ?, completed_at = ?
		WHERE id = ? AND user_id = ? AND status IN ('removing_account', 'complete')
	`, now, now, job.ID, string(job.UserID))
	if err != nil {
		return fmt.Errorf("auth: complete deletion job: %w", err)
	}
	if changed, changedErr := result.RowsAffected(); changedErr != nil || changed != 1 {
		return fmt.Errorf("auth: deletion job changed concurrently")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth: commit deletion completion: %w", err)
	}
	return nil
}

func (s *DeletionService) updateJob(ctx context.Context, jobID, status, lastError string, now int64) error {
	var nullableError any
	if lastError != "" {
		nullableError = lastError
	}
	result, err := s.store.db.ExecContext(ctx, `UPDATE account_deletions SET status = ?, last_error = ?, updated_at = ? WHERE id = ? AND status != 'complete'`, status, nullableError, now, jobID)
	if err != nil {
		return fmt.Errorf("auth: update deletion job: %w", err)
	}
	if changed, changedErr := result.RowsAffected(); changedErr != nil || changed != 1 {
		return fmt.Errorf("auth: deletion job unavailable")
	}
	return nil
}

func (s *DeletionService) recordFailure(ctx context.Context, jobID, message string, now int64) error {
	return s.updateJob(ctx, jobID, "failed", message, now)
}

func (s *DeletionService) jobByUserID(ctx context.Context, userID readerstore.UserID) (DeletionJob, error) {
	return deletionJobByUserID(ctx, s.store.db, userID)
}

func deletionJobByUserID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID readerstore.UserID) (DeletionJob, error) {
	var job DeletionJob
	var rawUserID string
	var lastError sql.NullString
	var completedAt sql.NullInt64
	err := queryer.QueryRowContext(ctx, `SELECT id, user_id, status, last_error, created_at, updated_at, completed_at FROM account_deletions WHERE user_id = ?`, string(userID)).Scan(&job.ID, &rawUserID, &job.Status, &lastError, &job.CreatedAt, &job.UpdatedAt, &completedAt)
	if err != nil {
		return DeletionJob{}, err
	}
	job.UserID, err = readerstore.ParseUserID(rawUserID)
	if err != nil {
		return DeletionJob{}, fmt.Errorf("auth: invalid deletion user ID: %w", err)
	}
	job.LastError = lastError.String
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Int64
	}
	return job, nil
}

func randomDeletionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
