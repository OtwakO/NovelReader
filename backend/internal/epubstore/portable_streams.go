package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
)

// validatePortableStreams follows ownership and index validation. It verifies
// physical records, not semantic nodes, metadata, or resource bindings. Failed
// attempts and receipts being removed need not retain their disposable output.
func validatePortableStreams(ctx context.Context, tx *sql.Tx, root *os.Root) error {
	rows, err := tx.QueryContext(ctx, `SELECT p.file_id,p.generation,p.state,p.stream_size FROM epub_preparations p
 JOIN epub_files f ON f.id=p.file_id WHERE f.state='acquired' AND p.state IN ('ready','finalizing') ORDER BY p.file_id,p.generation`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var generation, size int64
		var state PreparationState
		if err = rows.Scan(&id, &generation, &state, &size); err != nil {
			return err
		}
		err = checkPortableStream(ctx, tx, root, id, generation, size)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Validation does not repair this copied state. Normal recovery will fail
		// incomplete finalizations for retry, never promote them to ready.
		if state == PreparationFinalizing && isIncompletePreparation(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("epubstore: stream %s/%d: %w", id, generation, err)
		}
	}
	return rows.Err()
}

func checkPortableStream(ctx context.Context, tx *sql.Tx, root *os.Root, id string, generation, size int64) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	name := path.Join(preparationPath(id, generation), sectionStreamFile)
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return errIncompletePreparation
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	rows, err := tx.QueryContext(ctx, `SELECT ordinal,offset,length FROM epub_sections WHERE file_id=? AND generation=? ORDER BY ordinal`, id, generation)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var ordinal int
		var span SectionSpan
		if err = rows.Scan(&ordinal, &span.Offset, &span.Length); err != nil {
			return err
		}
		if _, err = readPortableSection(ctx, file, size, ordinal, span); err != nil {
			return err
		}
	}
	return rows.Err()
}
