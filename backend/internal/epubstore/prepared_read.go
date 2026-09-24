package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"path"

	"github.com/otwako/novelreader/internal/epub"
)

// PreparedSection fetches one current ready section. It does not load the catalog,
// rerun preparation, or share seek state. Publication/revision authorization is
// the reading provider's responsibility; the caller must hold the home lease.
func (s *Store) PreparedSection(ctx context.Context, id string, generation int64, ordinal int) (section epub.PreparedSection, err error) {
	if err = validateID(id); err != nil {
		return section, err
	}
	var span SectionSpan
	var size int64
	err = s.db.QueryRowContext(ctx, `SELECT s.offset,s.length,p.stream_size FROM epub_sections s
 JOIN epub_preparations p ON p.file_id=s.file_id AND p.generation=s.generation
 JOIN epub_files f ON f.id=p.file_id AND f.preparation_generation=p.generation
 WHERE s.file_id=? AND s.generation=? AND s.ordinal=? AND p.state='ready' AND p.format_version=? AND f.state='acquired'`, id, generation, ordinal, preparationFormatVersion).Scan(&span.Offset, &span.Length, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return section, ErrNotFound
	}
	if err != nil {
		return section, err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return section, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	f, err := root.Open(path.Join(preparationPath(id, generation), sectionStreamFile))
	if err != nil {
		return section, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return section, err
	}
	if info.Size() != size {
		return section, errInvalidSectionSpan
	}
	return readSection(ctx, f, size, ordinal, span)
}
