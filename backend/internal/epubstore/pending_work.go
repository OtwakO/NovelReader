package epubstore

import (
	"context"
	"database/sql"
	"errors"
)

// NextPreparation returns the oldest current queued generation without claiming
// it, within the caller's coherent cross-format read transaction. Preparation
// owns the subsequent generation-guarded claim and all output.
func NextPreparation(ctx context.Context, tx *sql.Tx) (PreparationAttempt, error) {
	var a PreparationAttempt
	err := tx.QueryRowContext(ctx, `SELECT p.file_id,p.generation,p.state,p.image_mode,p.error,p.created_at,p.updated_at
 FROM epub_preparations p JOIN epub_files f ON f.id=p.file_id
 WHERE p.state='queued' AND f.state='acquired' AND f.preparation_generation=p.generation
 ORDER BY p.created_at,p.file_id LIMIT 1`).Scan(&a.ReceiptID, &a.Generation, &a.State, &a.ImageMode, &a.Error, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return a, err
}

// PreparePending reports whether a claim was acquired even if preparation fails,
// allowing the scheduler to continue without retrying a failing claim in a loop.
func (s *Store) PreparePending(ctx context.Context, pending PreparationAttempt) (bool, error) {
	attempt, err := s.ClaimPreparation(ctx, pending.ReceiptID, pending.Generation)
	if err != nil {
		return false, err
	}
	return true, s.prepareClaimed(ctx, attempt)
}
