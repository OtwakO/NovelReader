package epubstore

import (
	"context"
	"database/sql"
	"os"
)

// preparePortable runs only against a copied database. Temporary output is not
// portable, but an installed finalizing generation can still be recovered.
func preparePortable(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `UPDATE epub_preparations SET stage_name='' WHERE stage_name<>''`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE epub_preparations SET state='failed',error='preparation interrupted by portable copy; retry required' WHERE state='preparing'`)
	if err != nil {
		return err
	}
	return preparePortableInbox(ctx, tx)
}

// validatePortableFiles is the storage-owned portable boundary. The order matters:
// establish ownership and index bounds before reading records or resolving images.
// It is read-only and runs against the private copy, not the live reader home.
func validatePortableFiles(ctx context.Context, tx *sql.Tx, root *os.Root) error {
	if err := validatePortableInbox(ctx, tx); err != nil {
		return err
	}
	if err := validatePortableOwnership(ctx, tx, root); err != nil {
		return err
	}
	if err := validatePortableSectionIndexes(ctx, tx); err != nil {
		return err
	}
	if err := validatePortableStreams(ctx, tx, root); err != nil {
		return err
	}
	if err := validatePortableResources(ctx, tx, root); err != nil {
		return err
	}
	return validatePortablePublications(ctx, tx)
}
