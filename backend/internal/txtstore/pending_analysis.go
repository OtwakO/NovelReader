package txtstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/txt"
)

// QueueAnalysis persists revised pending options and immediately revokes the old
// preview. Acquisition already leaves new originals Received with automatic options.
// The caller notifies the worker pool after this write, while still owning its lease.
func (s *Store) QueueAnalysis(ctx context.Context, id string, version int64, options txt.Options) error {
	if err := options.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireChange(tx.ExecContext(ctx, `UPDATE txt_files SET generation=generation+1,updated_at=? WHERE id=? AND state='acquired' AND library_id IS NULL AND EXISTS(SELECT 1 FROM txt_interpretations WHERE file_id=? AND generation=? AND role='candidate')`, time.Now().UnixMilli(), id, id, version)); err != nil {
		return err
	}
	if err := replaceCandidateTx(ctx, tx, id, 0, options); err != nil {
		return err
	}
	return tx.Commit()
}

// AnalyzePending claims only the selected generation and loads its saved options.
// It reports whether it claimed work even on failure, so scheduling can advance.
// A replacement cannot inherit an older candidate's scheduling priority.
func (s *Store) AnalyzePending(ctx context.Context, pending PendingAnalysis) (bool, error) {
	value, err := s.claimAnalysis(ctx, pending.ReceiptID, pending.Generation)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.analyzeClaim(ctx, value)
	if err != nil {
		return true, fmt.Errorf("txtstore: analyze receipt %s: %w", value.ID, err)
	}
	return true, nil
}
