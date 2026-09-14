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
	if _, err := tx.ExecContext(ctx, `DELETE FROM txt_interpretations WHERE file_id=? AND role='candidate'`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO txt_interpretations(file_id,generation,role,state,requested_encoding,requested_preset,requested_pattern,queued_at,updated_at) SELECT id,generation,'candidate','queued',?,?,?,updated_at,updated_at FROM txt_files WHERE id=?`, options.Encoding, options.Preset, options.Pattern, id); err != nil {
		return err
	}
	return tx.Commit()
}

// AnalyzeNext atomically claims at most one queued candidate and loads its saved
// options. It returns whether it claimed work, even on a per-file failure, so the
// scheduler can advance to another file. It retains neither results nor file handles.
// Call Recover only with intake quiescent, not before individual worker attempts.
func (s *Store) AnalyzeNext(ctx context.Context) (bool, error) {
	value, err := s.claimAnalysis(ctx, "")
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
