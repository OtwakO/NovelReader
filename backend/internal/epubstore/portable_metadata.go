package epubstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/otwako/novelreader/internal/epub"
)

// Read through the byte-limited SQL projection before bounded typed decoding.
// Both stream and resource validation use the same summary contract.
func portableMetadata(ctx context.Context, tx *sql.Tx, id string, generation int64) (*epub.Preparation, error) {
	var version, count int
	var size sql.NullInt64
	var data []byte
	var mode epub.ImageMode
	err := tx.QueryRowContext(ctx, `SELECT p.format_version,p.image_mode,
 length(CAST(p.metadata_json AS BLOB)),
 CASE WHEN length(CAST(p.metadata_json AS BLOB))<=? THEN p.metadata_json END,
 (SELECT COUNT(*) FROM epub_sections s WHERE s.file_id=p.file_id AND s.generation=p.generation)
 FROM epub_preparations p WHERE p.file_id=? AND p.generation=?`, maxPreparedJSONBytes, id, generation).Scan(&version, &mode, &size, &data, &count)
	if err != nil {
		return nil, err
	}
	if size.Valid && size.Int64 > maxPreparedJSONBytes {
		return nil, epub.ErrLimit
	}
	if version == 0 {
		return nil, nil
	} // Ownership/index checks establish no output here.
	if version != preparationFormatVersion {
		return nil, fmt.Errorf("epubstore: unsupported preparation format %d", version)
	}
	if err = checkPreparedJSON(ctx, data); err != nil {
		return nil, fmt.Errorf("%w: %w", epub.ErrPreparedMetadata, err)
	}
	var metadata epub.Preparation
	if err = json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("%w: %w", epub.ErrPreparedMetadata, err)
	}
	if err = epub.ValidatePreparedMetadata(ctx, metadata, mode, count); err != nil {
		return nil, err
	}
	return &metadata, nil
}
