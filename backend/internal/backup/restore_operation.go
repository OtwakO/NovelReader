package backup

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

// operationLocked validates ownership and expiry while s.mu is held. A running
// commit owns its staging until cleanup finishes, regardless of the prepare TTL.
func (s *Service) operationLocked(userID readerstore.UserID, id string) (operation, error) {
	value, ok := s.byID[id]
	if !ok || value.owner != userID {
		return operation{}, ErrRestoreNotFound
	}
	if value.summary.State != "committing" && !s.now().Before(value.expiresAt) {
		delete(s.byID, id)
		delete(s.byReader, userID)
		_ = os.RemoveAll(value.staging)
		return operation{}, ErrRestoreNotFound
	}
	return value, nil
}

func (s *Service) GetRestore(userID readerstore.UserID, id string) (PreparedRestore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.operationLocked(userID, id)
	return value.summary, err
}

func (s *Service) CancelRestore(userID readerstore.UserID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.operationLocked(userID, id)
	if err != nil {
		return err
	}
	if value.summary.State != "prepared" {
		return ErrRestoreConflict
	}
	delete(s.byID, id)
	delete(s.byReader, userID)
	return os.RemoveAll(value.staging)
}

func (s *Service) CommitRestore(ctx context.Context, userID readerstore.UserID, id string) (RestoreResult, error) {
	s.mu.Lock()
	value, err := s.operationLocked(userID, id)
	if err != nil {
		s.mu.Unlock()
		return RestoreResult{}, err
	}
	if s.closed || value.summary.State != "prepared" {
		s.mu.Unlock()
		return RestoreResult{}, ErrRestoreConflict
	}
	value.summary.State = "committing"
	s.byID[id] = value
	s.commits.Add(1)
	s.mu.Unlock()
	defer s.commits.Done()

	// The operation remains discoverable, but cannot be retried, canceled,
	// expired or superseded while replacement/recovery holds lifecycle ownership.
	result, commitErr := s.replaceReader(ctx, userID, value.staging)
	cleanupErr := os.RemoveAll(value.staging)
	if cleanupErr != nil {
		log.Printf("Reader Data restore staging cleanup pending: %v", cleanupErr)
		if result.Restored {
			result.Warnings = append(result.Warnings, "restore_staging_cleanup_pending")
		} else {
			commitErr = errors.Join(commitErr, cleanupErr)
		}
	}
	s.mu.Lock()
	if cleanupErr == nil {
		value.staging = ""
	}
	value.expiresAt = s.now().Add(restoreLifetime)
	value.summary.ExpiresAt = value.expiresAt.Format(time.RFC3339)
	value.summary.State = "failed"
	if result.Restored {
		value.summary.State = "committed"
		value.summary.Result = &result
	}
	s.byID[id] = value
	s.mu.Unlock()
	return result, commitErr
}

func (s *Service) replaceReader(ctx context.Context, userID readerstore.UserID, staging string) (RestoreResult, error) {
	if err := s.quiesce(ctx, userID); err != nil {
		s.resume(userID)
		return RestoreResult{}, err
	}
	defer s.resume(userID)
	err := s.readers.PublishReplacement(ctx, userID, staging)
	if err != nil && !errors.Is(err, readerstore.ErrReplacementCleanupPending) {
		return RestoreResult{}, err
	}
	result := RestoreResult{Restored: true}
	if err != nil {
		log.Printf("Reader Data replacement committed with old-home cleanup pending: %v", err)
		result.Warnings = append(result.Warnings, "reader_cleanup_pending")
	}
	if s.afterRestore != nil {
		// Publication has committed; disconnection cannot abandon reconciliation.
		recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		result.Warnings = append(result.Warnings, s.afterRestore(recoveryCtx, userID)...)
	}
	return result, nil
}
