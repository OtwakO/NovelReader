package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/otwako/novelreader/internal/epub"
)

// validatePortableOwnership follows readerstore's declared-schema validation and
// portable cleanup, checking copied records rather than repairing live state. It is only
// the ownership portion of portable validation; prepared content needs separate
// reference/semantic checks before the combined boundary can be registered.
func validatePortableOwnership(ctx context.Context, tx *sql.Tx, root *os.Root) error {
	var orphan bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epub_preparations p
 LEFT JOIN epub_files f ON f.id=p.file_id WHERE f.id IS NULL)`).Scan(&orphan)
	if err != nil {
		return err
	}
	if orphan {
		return fmt.Errorf("epubstore: preparation lacks a receipt")
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,original_name,state,size,preparation_generation,image_mode FROM epub_files ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var r Receipt
		if err := rows.Scan(&r.ID, &r.OriginalName, &r.State, &r.Size, &r.PreparationGeneration, &r.ImageMode); err != nil {
			return err
		}
		if err := validatePortableReceipt(root, r); err != nil {
			return fmt.Errorf("epubstore: receipt %s: %w", r.ID, err)
		}
		if err := validatePortablePreparationOwnership(ctx, tx, r); err != nil {
			return fmt.Errorf("epubstore: receipt %s: %w", r.ID, err)
		}
	}
	return rows.Err()
}

func validatePortableReceipt(root *os.Root, r Receipt) error {
	if err := validateID(r.ID); err != nil {
		return err
	}
	if err := ValidateFilename(r.OriginalName); err != nil {
		return err
	}
	if r.ImageMode != epub.OriginalImages && r.ImageMode != epub.OptimizedImages {
		return epub.ErrImagePolicy
	}
	if r.Size < 0 || r.PreparationGeneration < 0 {
		return fmt.Errorf("invalid receipt size or generation")
	}
	switch r.State {
	case Acquired, Finalizing:
		if r.Size > epub.MaxInputBytes {
			return fmt.Errorf("original exceeds acquisition limit")
		}
	case Receiving, Failed, Removing:
	default:
		return fmt.Errorf("invalid acquisition state")
	}
	if r.State != Acquired && r.State != Removing && r.PreparationGeneration != 0 {
		return fmt.Errorf("unfinished acquisition owns preparation generations")
	}
	info, err := root.Lstat(originalPath(r.ID))
	// Finalizing may still refer to a transfer in excluded .work storage. Recovery
	// fails that receipt if no installed original survived; it must remain portable.
	if errors.Is(err, os.ErrNotExist) && r.State != Acquired {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("original is not a regular file")
	}
	// Failed/removing receipts may retain damaged bytes for explicit cleanup.
	if r.State != Failed && r.State != Removing && info.Size() != r.Size {
		return fmt.Errorf("original size differs from receipt")
	}
	return nil
}

func validatePortablePreparationOwnership(ctx context.Context, tx *sql.Tx, r Receipt) error {
	rows, err := tx.QueryContext(ctx, `SELECT generation,state,image_mode,stage_name FROM epub_preparations WHERE file_id=? ORDER BY generation`, r.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	current := r.PreparationGeneration == 0
	for rows.Next() {
		var generation int64
		var state PreparationState
		var mode epub.ImageMode
		var stage string
		if err := rows.Scan(&generation, &state, &mode, &stage); err != nil {
			return err
		}
		if generation <= 0 || generation > r.PreparationGeneration {
			return fmt.Errorf("invalid preparation generation")
		}
		if mode != r.ImageMode {
			return fmt.Errorf("preparation image policy differs from receipt")
		}
		if stage != "" {
			return fmt.Errorf("portable preparation retains local staging authority")
		}
		switch state {
		case PreparationQueued, PreparationFinalizing, PreparationReady, PreparationFailed:
		default:
			return fmt.Errorf("invalid portable preparation state")
		}
		if generation == r.PreparationGeneration {
			current = true
		} else if state != PreparationFailed {
			return fmt.Errorf("superseded preparation is not failed")
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !current {
		return fmt.Errorf("receipt lacks its current preparation generation")
	}
	return nil
}
