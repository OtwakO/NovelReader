package epubstore

import (
	"context"
	"database/sql"
)

// preparePortable runs only against a copied database. Temporary output is not
// portable, but an installed finalizing generation can still be recovered.
// Registration awaits the full portable validation boundary.
func preparePortable(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `UPDATE epub_preparations SET stage_name='' WHERE stage_name<>''`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE epub_preparations SET state='failed',error='preparation interrupted by portable copy; retry required' WHERE state='preparing'`)
	return err
}
