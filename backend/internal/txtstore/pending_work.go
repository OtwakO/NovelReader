package txtstore

import (
	"context"
	"database/sql"
	"errors"
)

// PendingAnalysis is a snapshot for scheduling, not authority to change a newer
// candidate. QueuedAt is the immutable request time, not its last update time.
type PendingAnalysis struct {
	ReceiptID  string
	Generation int64
	QueuedAt   int64
}

// NextAnalysis uses the caller's read transaction so cross-format selection sees
// one coherent queue snapshot. No claim is acquired until that transaction ends.
func NextAnalysis(ctx context.Context, tx *sql.Tx) (PendingAnalysis, error) {
	var pending PendingAnalysis
	err := tx.QueryRowContext(ctx, `SELECT i.file_id,i.generation,i.queued_at
 FROM txt_interpretations i JOIN txt_files f ON f.id=i.file_id
 WHERE i.role='candidate' AND i.state='queued' AND f.state='acquired' AND f.generation=i.generation
 ORDER BY i.queued_at,i.file_id LIMIT 1`).Scan(&pending.ReceiptID, &pending.Generation, &pending.QueuedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return pending, err
}
