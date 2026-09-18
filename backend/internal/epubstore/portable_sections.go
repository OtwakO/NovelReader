package epubstore

import (
	"context"
	"database/sql"
	"fmt"
)

// validatePortableSectionIndexes checks persisted format and range ownership.
// It does not read stream bodies: semantic/file validation remains a separate
// required step before this package can register a complete portable validator.
func validatePortableSectionIndexes(ctx context.Context, tx *sql.Tx) error {
	var orphan bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epub_sections s
 LEFT JOIN epub_preparations p ON p.file_id=s.file_id AND p.generation=s.generation
 WHERE p.file_id IS NULL)`).Scan(&orphan); err != nil {
		return err
	}
	if orphan {
		return fmt.Errorf("epubstore: section index lacks a preparation")
	}
	rows, err := tx.QueryContext(ctx, `SELECT file_id,generation,state,format_version,stream_size,metadata_json IS NOT NULL FROM epub_preparations ORDER BY file_id,generation`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var generation, size int64
		var state PreparationState
		var version int
		var hasMetadata bool
		if err = rows.Scan(&id, &generation, &state, &version, &size, &hasMetadata); err != nil {
			return err
		}
		if version == 0 {
			// Queued work and attempts failed before intent have no durable output.
			if (state != PreparationQueued && state != PreparationFailed) || size != 0 || hasMetadata {
				return fmt.Errorf("epubstore: preparation %s/%d has output without a format", id, generation)
			}
		} else if version != preparationFormatVersion || !hasMetadata || size <= 0 || size > maxSectionTotalBytes ||
			(state != PreparationFinalizing && state != PreparationReady && state != PreparationFailed) {
			return fmt.Errorf("epubstore: preparation %s/%d has an invalid output header", id, generation)
		}
		if err = validatePortableSectionRanges(ctx, tx, id, generation, size); err != nil {
			return fmt.Errorf("epubstore: preparation %s/%d: %w", id, generation, err)
		}
	}
	return rows.Err()
}

func validatePortableSectionRanges(ctx context.Context, tx *sql.Tx, id string, generation, size int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT ordinal,offset,length FROM epub_sections WHERE file_id=? AND generation=? ORDER BY ordinal`, id, generation)
	if err != nil {
		return err
	}
	defer rows.Close()
	ordinal := 0
	var end int64
	for rows.Next() {
		var storedOrdinal int
		var span SectionSpan
		if err = rows.Scan(&storedOrdinal, &span.Offset, &span.Length); err != nil {
			return err
		}
		if storedOrdinal != ordinal || span.Offset != end || !span.validFor(size) {
			return errInvalidSectionSpan
		}
		// validFor establishes end <= size before this addition, without overflow.
		end += span.Length
		ordinal++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if end != size {
		return errInvalidSectionSpan
	}
	return nil
}
