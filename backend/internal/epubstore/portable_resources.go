package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/otwako/novelreader/internal/epub"
)

// Follows ownership and section-index validation. Resource bytes are checked once
// per registry entry, not per image occurrence in a section or cover.
func validatePortableResources(ctx context.Context, tx *sql.Tx, root *os.Root) error {
	var orphan bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epub_resources r LEFT JOIN epub_preparations p ON p.file_id=r.file_id AND p.generation=r.generation WHERE p.file_id IS NULL)`).Scan(&orphan); err != nil {
		return err
	}
	if orphan {
		return fmt.Errorf("epubstore: resource lacks a preparation")
	}
	rows, err := tx.QueryContext(ctx, `SELECT p.file_id,p.generation,p.image_mode,p.state,p.format_version,f.state FROM epub_preparations p JOIN epub_files f ON f.id=p.file_id ORDER BY p.file_id,p.generation`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a PreparationAttempt
		var version int
		var receiptState AcquisitionState
		if err = rows.Scan(&a.ReceiptID, &a.Generation, &a.ImageMode, &a.State, &version, &receiptState); err != nil {
			return err
		}
		active := receiptState == Acquired && (a.State == PreparationReady || a.State == PreparationFinalizing)
		if err = checkPortableResources(ctx, tx, root, a, version, active); err != nil {
			return fmt.Errorf("epubstore: resources %s/%d: %w", a.ReceiptID, a.Generation, err)
		}
	}
	return rows.Err()
}

func checkPortableResources(ctx context.Context, tx *sql.Tx, root *os.Root, a PreparationAttempt, version int, active bool) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,source_path,derivative_id,derivative_bytes,media_type,width,height FROM epub_resources WHERE file_id=? AND generation=? ORDER BY id`, a.ReceiptID, a.Generation)
	if err != nil {
		return err
	}
	defer rows.Close()
	var total int64
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return err
		}
		var r PreparedResource
		if err = rows.Scan(&r.ID, &r.Image.Reference.Path, &r.Image.DerivativeID, &r.DerivativeBytes, &r.Image.Info.MediaType, &r.Image.Info.Width, &r.Image.Info.Height); err != nil {
			return err
		}
		if version != preparationFormatVersion {
			return fmt.Errorf("resource without prepared output")
		}
		if err = validateID(r.ID); err != nil {
			return err
		}
		if err = epub.ValidatePreparedImage(r.Image, a.ImageMode); err != nil {
			return err
		}
		if a.ImageMode == epub.OriginalImages {
			if r.DerivativeBytes != 0 {
				return fmt.Errorf("original resource claims derivative bytes")
			}
		} else {
			if r.Image.DerivativeID != r.ID || r.DerivativeBytes <= 0 || r.DerivativeBytes > maxDerivativeBytes || r.DerivativeBytes > maxDerivativeTotalBytes-total {
				return fmt.Errorf("invalid derivative identity or size")
			}
			total += r.DerivativeBytes
		}
		if !active {
			continue
		}
		if err = checkPortableResourceBytes(ctx, root, a, r); err != nil {
			return fmt.Errorf("resource %s: %w", r.ID, err)
		}
	}
	return rows.Err()
}

func checkPortableResourceBytes(ctx context.Context, root *os.Root, a PreparationAttempt, r PreparedResource) (err error) {
	var data []byte
	if a.ImageMode == epub.OriginalImages {
		f, openErr := root.Open(originalPath(a.ReceiptID))
		if openErr != nil {
			return openErr
		}
		defer func() { err = errors.Join(err, f.Close()) }()
		info, statErr := f.Stat()
		if statErr != nil {
			return statErr
		}
		data, err = epub.ReadOriginalImage(ctx, f, info.Size(), r.Image.Reference)
		if err != nil {
			return err
		}
	} else {
		name := path.Join(preparationPath(a.ReceiptID, a.Generation), "images", r.Image.DerivativeID+".webp")
		info, statErr := root.Lstat(name)
		if statErr == nil && (!info.Mode().IsRegular() || info.Size() != r.DerivativeBytes) {
			statErr = errIncompletePreparation
		}
		// Recovery already detects missing/size-mismatched installation output.
		// Invalid encoded bytes are NOT tolerated: recovery only stats derivatives.
		if a.State == PreparationFinalizing && isIncompletePreparation(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		f, openErr := root.Open(name)
		if openErr != nil {
			return openErr
		}
		defer func() { err = errors.Join(err, f.Close()) }()
		data, err = io.ReadAll(io.NewSectionReader(contextReaderAt{ctx, f}, 0, r.DerivativeBytes))
		if err != nil {
			return err
		}
		if int64(len(data)) != r.DerivativeBytes {
			return errIncompletePreparation
		}
	}
	return epub.ValidatePreparedImageBytes(ctx, data, r.Image, a.ImageMode)
}
