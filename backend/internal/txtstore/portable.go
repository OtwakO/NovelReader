package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/otwako/novelreader/internal/library"
)

// validatePortableFiles validates ownership in both directions without decoding
// originals or rebuilding indexes. Unreferenced files are never swept.
func validatePortableFiles(ctx context.Context, tx *sql.Tx, root *os.Root) error {
	var localClaims bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM txt_inbox_claims)`).Scan(&localClaims); err != nil {
		return err
	}
	if localClaims {
		return fmt.Errorf("txtstore: portable data contains inbox cleanup authority")
	}
	items, err := library.ListTx(ctx, tx)
	if err != nil {
		return err
	}
	publications := make(map[string]library.Item)
	for _, item := range items {
		if item.Provider == library.TXT {
			publications[item.ID] = item
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,path,state,size,COALESCE(library_id,''),generation FROM txt_files ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var value Receipt
		var generation int64
		if err := rows.Scan(&value.ID, &value.Path, &value.State, &value.Size, &value.LibraryID, &generation); err != nil {
			return err
		}
		item, published := publications[value.ID]
		if value.LibraryID != "" {
			if value.State != acquired || value.LibraryID != value.ID || !published {
				return fmt.Errorf("txtstore: receipt %s has no matching TXT publication", value.ID)
			}
			delete(publications, value.ID)
		}
		if err := validatePortableInterpretations(ctx, tx, value, generation, item); err != nil {
			return fmt.Errorf("txtstore: receipt %s: %w", value.ID, err)
		}
		if err := validatePortableOriginal(root, value); err != nil {
			return fmt.Errorf("txtstore: receipt %s: %w", value.ID, err)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(publications) != 0 {
		return fmt.Errorf("txtstore: %d TXT publications lack a published receipt", len(publications))
	}
	return nil
}

func validatePortableOriginal(root *os.Root, value Receipt) error {
	if err := validateReceiptPath(value); err != nil {
		return err
	}
	optional := value.State == Receiving || value.State == Failed || value.State == Removing
	info, err := root.Lstat(value.Path)
	if optional && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("managed original is not a regular file")
	}
	// Failed/removing receipts may preserve damaged bytes for explicit cleanup.
	// A finalized receiving original, however, must match its acquisition intent.
	if value.State != Failed && value.State != Removing && info.Size() != value.Size {
		return fmt.Errorf("managed original size differs from receipt")
	}
	return nil
}

func preparePortable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM txt_inbox_claims`)
	return err
}
