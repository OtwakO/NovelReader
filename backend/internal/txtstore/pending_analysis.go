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
	result, err := s.db.ExecContext(ctx, `UPDATE txt_files SET state=?,analysis_version=analysis_version+1,requested_encoding=?,requested_preset=?,error='',updated_at=? WHERE id=? AND analysis_version=? AND state IN (?,?,?,?)`, Received, options.Encoding, options.Preset, time.Now().UnixMilli(), id, version, Received, Ready, NeedsReview, AnalysisFailed)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrStateChanged
	}
	return nil
}

// AnalyzeNext atomically claims at most one received original and loads its saved
// options. It returns whether it claimed work, even on a per-file failure, so the
// scheduler can advance to another file. It retains neither results nor file handles.
// Call Recover only with intake quiescent, not before individual worker attempts.
func (s *Store) AnalyzeNext(ctx context.Context) (bool, error) {
	value, err := s.claimAnalysis(ctx, `UPDATE txt_files SET state=?,analysis_version=analysis_version+1,error='',updated_at=? WHERE id=(SELECT id FROM txt_files WHERE state=? ORDER BY id LIMIT 1) AND state=? RETURNING `+receiptColumns, Analyzing, time.Now().UnixMilli(), Received, Received)
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
