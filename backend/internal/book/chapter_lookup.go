package book

import (
	"context"
	"database/sql"
)

// GetChapterWithNext returns an exact chapter and its immediate catalog successor,
// which may be a volume heading. A missing index returns nil, nil, nil.
func (s *Store) GetChapterWithNext(ctx context.Context, bookID string, index int) (*Chapter, *Chapter, error) {
	return getChapterWithNext(ctx, s.db, bookID, index)
}

func getChapterWithNext(ctx context.Context, query interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, bookID string, index int) (*Chapter, *Chapter, error) {
	rows, err := query.QueryContext(ctx, `SELECT `+chapterColumns+` FROM chapters WHERE book_id = ? AND idx >= ? ORDER BY idx ASC LIMIT 2`, bookID, index)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	chapters, err := scanChapters(rows)
	if err != nil {
		return nil, nil, err
	}
	if len(chapters) == 0 || chapters[0].Index != index {
		return nil, nil, nil
	}
	if len(chapters) == 1 {
		return &chapters[0], nil, nil
	}
	return &chapters[0], &chapters[1], nil
}

// GetChapterSnapshot reads the binding, interpretation revision and chapter pair
// from one database snapshot. Network work must happen after this transaction ends.
func (s *Store) GetChapterSnapshot(ctx context.Context, bookID string, index int) (*Book, *Chapter, *Chapter, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, nil, err
	}
	defer tx.Rollback()
	stored, err := readBookTx(ctx, tx, bookID)
	if err != nil || stored == nil {
		return stored, nil, nil, err
	}
	chapter, next, err := getChapterWithNext(ctx, tx, bookID, index)
	return stored, chapter, next, err
}

// IsChapterSnapshotCurrent excludes source switches, catalog replacement and removal,
// but deliberately does not reject a fetch because progress or bookmarks changed.
func (s *Store) IsChapterSnapshotCurrent(ctx context.Context, snapshot *Book) (bool, error) {
	var current bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM books b JOIN library_items l ON l.id=b.id WHERE b.id=? AND b.source_id=? AND l.content_revision=?)`, snapshot.ID, snapshot.SourceID, snapshot.ContentRevision).Scan(&current)
	return current, err
}
