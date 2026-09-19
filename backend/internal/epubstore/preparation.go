package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/otwako/novelreader/internal/epub"
)

type PreparationState string

const (
	PreparationQueued     PreparationState = "queued"
	PreparationRunning    PreparationState = "preparing"
	PreparationFailed     PreparationState = "failed"
	PreparationFinalizing PreparationState = "finalizing"
	PreparationReady      PreparationState = "ready" // complete output, not review approval
)

// PreparationAttempt identifies ownership of one preparation, not publication.
// A worker must carry Generation through every state/output commit, even after
// cancellation: a replacement attempt must never accept an older completion.
type PreparationAttempt struct {
	ReceiptID            string
	Generation           int64
	State                PreparationState
	ImageMode            epub.ImageMode
	Error                string
	CreatedAt, UpdatedAt int64
}

// QueuePreparation creates a new generation. Scheduling/cancellation belongs to
// the caller. Superseded attempt evidence is retained; the caller remains
// responsible for staged work. This method neither removes files nor joins workers.
func (s *Store) QueuePreparation(ctx context.Context, id string) (PreparationAttempt, error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return PreparationAttempt{}, err
	}
	defer unlock()
	r, err := s.Get(ctx, id)
	if err != nil {
		return PreparationAttempt{}, err
	}
	if r.State != Acquired {
		return PreparationAttempt{}, ErrStateChanged
	}
	if r.PreparationGeneration > 0 {
		current, err := s.GetPreparation(ctx, id, r.PreparationGeneration)
		if err != nil {
			return PreparationAttempt{}, err
		}
		if current.State == PreparationFinalizing || current.State == PreparationReady {
			return PreparationAttempt{}, ErrStateChanged
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PreparationAttempt{}, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	a := PreparationAttempt{ReceiptID: id, Generation: r.PreparationGeneration + 1, State: PreparationQueued, ImageMode: r.ImageMode, CreatedAt: now, UpdatedAt: now}
	// The receipt counter survives old-attempt cleanup, so a token is never reused.
	result, err := tx.ExecContext(ctx, `UPDATE epub_files SET preparation_generation=?,updated_at=? WHERE id=? AND state='acquired' AND preparation_generation=?`, a.Generation, now, id, r.PreparationGeneration)
	if err != nil {
		return PreparationAttempt{}, err
	}
	if err = changedOne(result); err != nil {
		return PreparationAttempt{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE epub_preparations SET state='failed',error='superseded by retry',updated_at=? WHERE file_id=? AND generation=? AND state IN ('queued','preparing')`, now, id, r.PreparationGeneration)
	if err != nil {
		return PreparationAttempt{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO epub_preparations(file_id,generation,state,image_mode,created_at,updated_at) VALUES(?,?,?,?,?,?)`, id, a.Generation, a.State, a.ImageMode, now, now)
	if err != nil {
		return PreparationAttempt{}, err
	}
	if err = tx.Commit(); err != nil {
		return PreparationAttempt{}, err
	}
	return a, nil
}

func (s *Store) GetPreparation(ctx context.Context, id string, generation int64) (PreparationAttempt, error) {
	var a PreparationAttempt
	err := s.db.QueryRowContext(ctx, `SELECT file_id,generation,state,image_mode,error,created_at,updated_at FROM epub_preparations WHERE file_id=? AND generation=?`, id, generation).Scan(&a.ReceiptID, &a.Generation, &a.State, &a.ImageMode, &a.Error, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PreparationAttempt{}, ErrNotFound
	}
	return a, err
}

func (s *Store) ClaimPreparation(ctx context.Context, id string, generation int64) (PreparationAttempt, error) {
	return s.transitionPreparation(ctx, id, generation, PreparationQueued, PreparationRunning, "")
}

// FailPreparation must use a live cleanup context if the worker's operation
// context was cancelled. A stale failure returns ErrStateChanged, not success.
func (s *Store) FailPreparation(ctx context.Context, id string, generation int64, cause error) error {
	_, err := s.transitionPreparation(ctx, id, generation, PreparationRunning, PreparationFailed, cause.Error())
	return err
}

func (s *Store) transitionPreparation(ctx context.Context, id string, generation int64, from, to PreparationState, message string) (PreparationAttempt, error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return PreparationAttempt{}, err
	}
	defer unlock()
	if err := ctx.Err(); err != nil {
		return PreparationAttempt{}, err
	}
	// Return a committed claim even when shutdown cancels the worker during this
	// short transaction. The caller then owns settling that claim and its cleanup.
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(commitCtx, nil)
	if err != nil {
		return PreparationAttempt{}, err
	}
	defer tx.Rollback()
	var a PreparationAttempt
	err = tx.QueryRowContext(commitCtx, `UPDATE epub_preparations SET state=?,error=?,updated_at=?
 WHERE file_id=? AND generation=? AND state=? AND EXISTS(
 SELECT 1 FROM epub_files WHERE id=? AND state='acquired' AND preparation_generation=?)
 RETURNING file_id,generation,state,image_mode,error,created_at,updated_at`, to, message, time.Now().UnixMilli(), id, generation, from, id, generation).Scan(&a.ReceiptID, &a.Generation, &a.State, &a.ImageMode, &a.Error, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrStateChanged
	}
	if err != nil {
		return PreparationAttempt{}, err
	}
	if err = tx.Commit(); err != nil {
		return PreparationAttempt{}, err
	}
	return a, nil
}
