package book

import "context"

// GetChapterWithNext returns an exact chapter and its immediate catalog successor,
// which may be a volume heading. A missing index returns nil, nil, nil.
func (s *Store) GetChapterWithNext(ctx context.Context, bookID string, index int) (*Chapter, *Chapter, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+chapterColumns+` FROM chapters WHERE book_id = ? AND idx >= ? ORDER BY idx ASC LIMIT 2`, bookID, index)
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

// HasReadableChapter validates one progress target without loading the catalog.
func (s *Store) HasReadableChapter(ctx context.Context, bookID string, index int) (bool, error) {
	var readable bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chapters WHERE book_id = ? AND idx = ? AND is_volume = 0)`, bookID, index).Scan(&readable)
	return readable, err
}
