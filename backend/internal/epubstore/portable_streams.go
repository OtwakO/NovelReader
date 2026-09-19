package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/otwako/novelreader/internal/epub"
)

// validatePortableStreams follows ownership and index validation. It verifies
// bounded records, semantics, summary/resource agreement and target existence.
// Failed attempts and receipts being removed need not retain disposable output.
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
	metadata, err := portableMetadata(ctx, tx, id, generation)
	if err != nil {
		return err
	}
	if metadata == nil {
		return epub.ErrPreparedMetadata
	}
	publication := epub.NewPreparedPublicationCheck()
	resources, err := tx.PrepareContext(ctx, `SELECT derivative_id,media_type,width,height FROM epub_resources WHERE file_id=? AND generation=? AND source_path=?`)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, resources.Close()) }()
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
		section, readErr := readPortableSection(ctx, file, size, ordinal, span)
		if readErr != nil {
			return readErr
		}
		if err = publication.Add(ctx, section); err != nil {
			return err
		}
		for _, image := range section.Images {
			stored := epub.PreparedImage{Reference: epub.Reference{Path: image.Reference.Path}}
			err = resources.QueryRowContext(ctx, id, generation, image.Reference.Path).Scan(&stored.DerivativeID, &stored.Info.MediaType, &stored.Info.Width, &stored.Info.Height)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: image not registered", epub.ErrPreparedSection)
			}
			if err != nil {
				return err
			}
			if stored != image {
				return fmt.Errorf("%w: image registry mismatch", epub.ErrPreparedSection)
			}
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return publication.Finish(ctx, *metadata)
}
