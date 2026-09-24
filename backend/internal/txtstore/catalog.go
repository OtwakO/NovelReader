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

const publicationSections = ` FROM txt_files f
 JOIN txt_interpretations i ON i.file_id=f.id AND i.role='active'
 JOIN txt_sections s ON s.file_id=i.file_id AND s.generation=i.generation
 WHERE f.library_id=? AND f.state='acquired'`

// GetCatalog reads the active index and its library revision in one snapshot.
// Candidate work never becomes part of a reading query.
func (s *Store) GetCatalog(ctx context.Context, id string) ([]IndexedSection, int64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	item, err := library.GetTx(ctx, tx, id)
	if err != nil {
		return nil, 0, err
	}
	if item == nil || item.Provider != library.TXT {
		return nil, 0, ErrNotFound
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.idx,s.title,s.start_byte,s.end_byte,s.generated`+publicationSections+` ORDER BY s.idx`, id)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	sections := make([]IndexedSection, 0)
	for rows.Next() {
		var section IndexedSection
		if err := rows.Scan(&section.Index, &section.Title, &section.Start, &section.End, &section.Generated); err != nil {
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
	return sections, item.ContentRevision, nil
}

// GetSection validates a location using only its indexed row, without file I/O.
func (s *Store) GetSection(ctx context.Context, id string, revision int64, index int) (IndexedSection, error) {
	var section IndexedSection
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return section, err
	}
	defer tx.Rollback()
	item, err := library.GetTx(ctx, tx, id)
	if err != nil {
		return section, err
	}
	if item == nil || item.Provider != library.TXT {
		return section, ErrNotFound
	}
	if item.ContentRevision != revision {
		return section, library.ErrStateChanged
	}
	err = tx.QueryRowContext(ctx, `SELECT s.idx,s.title,s.start_byte,s.end_byte,s.generated`+publicationSections+` AND s.idx=?`, id, index).Scan(&section.Index, &section.Title, &section.Start, &section.End, &section.Generated)
	if errors.Is(err, sql.ErrNoRows) {
		return section, ErrNotFound
	}
	return section, err
}
