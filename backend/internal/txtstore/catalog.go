package txtstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
)

type IndexedSection struct {
	Index int
	txt.Section
}

const publicationSections = ` FROM library_items l
 JOIN txt_files f ON f.library_id=l.id AND f.state='published'
 JOIN txt_sections s ON s.receipt_id=f.id WHERE l.id=? AND l.provider='txt'`

// GetCatalog reads the published index and its revision in one database snapshot.
// It does not decode original bytes or expose pending analysis previews.
func (s *Store) GetCatalog(ctx context.Context, id string) ([]IndexedSection, int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.content_revision,s.idx,s.title,s.start_byte,s.end_byte,s.generated`+publicationSections+` ORDER BY s.idx`, id)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var revision int64
	sections := make([]IndexedSection, 0)
	for rows.Next() {
		var section IndexedSection
		if err := rows.Scan(&revision, &section.Index, &section.Title, &section.Start, &section.End, &section.Generated); err != nil {
			return nil, 0, err
		}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(sections) == 0 {
		return nil, 0, ErrNotFound
	}
	return sections, revision, nil
}

// GetSection validates a location using only its indexed row, without file I/O.
func (s *Store) GetSection(ctx context.Context, id string, revision int64, index int) (IndexedSection, error) {
	var section IndexedSection
	var current int64
	err := s.db.QueryRowContext(ctx, `SELECT l.content_revision,s.idx,s.title,s.start_byte,s.end_byte,s.generated`+publicationSections+` AND s.idx=?`, id, index).Scan(&current, &section.Index, &section.Title, &section.Start, &section.End, &section.Generated)
	if errors.Is(err, sql.ErrNoRows) {
		return section, ErrNotFound
	}
	if err != nil {
		return section, err
	}
	if current != revision {
		return section, library.ErrStateChanged
	}
	return section, nil
}
