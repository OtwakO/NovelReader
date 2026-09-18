package epubstore

import (
	"context"
	"errors"
	"os"
	"path"

	"github.com/otwako/novelreader/internal/epub"
)

var errIncompletePreparation = errors.New("epubstore: incomplete prepared output")

// Check crash-recovery output, not untrusted portable semantic correctness.
// All records are read individually; images are statted, never decoded again.
func (s *Store) checkInstalledPreparation(ctx context.Context, root *os.Root, a PreparationAttempt) (err error) {
	metadata, _, size, err := s.preparationMetadata(ctx, a.ReceiptID, a.Generation, PreparationFinalizing)
	if err != nil {
		return err
	}
	directory := preparationPath(a.ReceiptID, a.Generation)
	f, err := root.Open(path.Join(directory, sectionStreamFile))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != size || len(metadata.Sections) == 0 {
		return errInvalidSectionSpan
	}
	resources, err := s.preparedResources(ctx, a.ReceiptID, a.Generation)
	if err != nil {
		return err
	}
	byPath := map[string]epub.PreparedImage{}
	for _, r := range resources {
		byPath[r.Image.Reference.Path] = r.Image
		if r.Image.DerivativeID != "" {
			if err = validateID(r.Image.DerivativeID); err != nil {
				return err
			}
			info, err := root.Lstat(path.Join(directory, "images", r.Image.DerivativeID+".webp"))
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || info.Size() != r.DerivativeBytes {
				return errIncompletePreparation
			}
		}
	}
	if metadata.Cover != nil {
		image, found := byPath[metadata.Cover.Reference.Path]
		if !found || image != *metadata.Cover {
			return errIncompletePreparation
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ordinal,offset,length FROM epub_sections WHERE file_id=? AND generation=? ORDER BY ordinal`, a.ReceiptID, a.Generation)
	if err != nil {
		return err
	}
	ordinal := 0
	var end int64
	for rows.Next() {
		var storedOrdinal int
		var span SectionSpan
		if err = rows.Scan(&storedOrdinal, &span.Offset, &span.Length); err != nil {
			break
		}
		if storedOrdinal != ordinal || span.Offset != end {
			err = errInvalidSectionSpan
			break
		}
		var section epub.PreparedSection
		section, err = readSection(ctx, f, size, ordinal, span)
		if err != nil {
			break
		}
		for _, image := range section.Images {
			stored, found := byPath[image.Reference.Path]
			if !found || stored != image {
				err = errIncompletePreparation
				break
			}
		}
		if err != nil {
			break
		}
		end += span.Length
		ordinal++
	}
	err = errors.Join(err, rows.Err(), rows.Close())
	if err == nil && (end != size || ordinal != len(metadata.Sections)) {
		err = errInvalidSectionSpan
	}
	return err
}
