package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// The file lifecycle and interpretation lifecycle are separate. Public receipt
// states remain a projection for the existing import workflow.
const (
	acquired = "acquired"
	queued   = "queued"
)

func (s *Store) finalizeAcquisition(ctx context.Context, id string, size int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireChange(tx.ExecContext(ctx, `UPDATE txt_files SET state='acquired',size=?,error='',generation=generation+1,updated_at=? WHERE id=? AND state='receiving'`, size, time.Now().UnixMilli(), id)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO txt_interpretations(file_id,generation,role,state,queued_at,updated_at) SELECT id,generation,'candidate','queued',updated_at,updated_at FROM txt_files WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Every guarded interpretation write must affect its exact current row.
func requireChange(result sql.Result, err error) error {
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

// Wait for a connection cancellably, then finish the short claim independently of
// cancellation. The worker must receive its committed claim so it can release it.
// No generation is allocated here: retries continue the same immutable request.
func (s *Store) claimAnalysis(ctx context.Context, id string) (Receipt, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer conn.Close()
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	claimCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	tx, err := conn.BeginTx(claimCtx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback()
	var claim Receipt
	query := `UPDATE txt_interpretations SET state='analyzing',error='',updated_at=? WHERE role='candidate' AND state='queued' AND file_id=`
	args := []any{time.Now().UnixMilli()}
	if id == "" {
		query += `(SELECT file_id FROM txt_interpretations WHERE role='candidate' AND state='queued' ORDER BY queued_at,file_id LIMIT 1)`
	} else {
		query += `?`
		args = append(args, id)
	}
	err = tx.QueryRowContext(claimCtx, query+` RETURNING file_id,generation,requested_encoding,requested_preset,requested_pattern`, args...).Scan(&claim.ID, &claim.AnalysisVersion, &claim.Options.Encoding, &claim.Options.Preset, &claim.Options.Pattern)
	if errors.Is(err, sql.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	value, err := scanReceipt(tx.QueryRowContext(claimCtx, `SELECT `+receiptColumns+` FROM txt_receipts WHERE id=?`, claim.ID))
	if err != nil {
		return Receipt{}, err
	}
	// Published receipt metadata describes the active interpretation, not this job.
	value.AnalysisVersion, value.Options = claim.AnalysisVersion, claim.Options
	if err := tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return value, nil
}
