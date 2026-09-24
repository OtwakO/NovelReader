package library

import (
	"context"
	"database/sql"
	"math"
	"time"
)

// Location is validated against the provider's readable catalog by the use case.
// The final revision check prevents that validation surviving a catalog change.
type Location struct {
	ChapterIndex int
	Position     float64
	ChapterTitle string
}

func validLocation(location Location) bool {
	return location.ChapterIndex >= 0 && !math.IsNaN(location.Position) && !math.IsInf(location.Position, 0) && location.Position >= 0 && location.Position <= 1
}

func (s *Store) UpdateProgress(ctx context.Context, id string, expected Revision, location Location) (int64, error) {
	if !validLocation(location) {
		return 0, ErrInvalidProgress
	}
	now := time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, `UPDATE library_items SET dur_chapter_index=?, dur_chapter_pos=?, current_chapter_title=?,
		state_version=state_version+1, updated_at=?, last_read_at=? WHERE id=? AND content_revision=? AND state_version=?`,
		location.ChapterIndex, location.Position, location.ChapterTitle, now, now, id, expected.Content, expected.State)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		item, err := s.Get(ctx, id)
		if err != nil {
			return 0, err
		}
		if item == nil {
			return 0, ErrNotFound
		}
		return 0, ErrStateChanged
	}
	return expected.State + 1, nil
}

// PublishCatalogTx changes interpretation without consuming a reading-state
// version. A progress write during a crawl must not invalidate that crawl.
func PublishCatalogTx(ctx context.Context, tx *sql.Tx, id string, expectedContent int64, count int, currentTitle string) error {
	result, err := tx.ExecContext(ctx, `UPDATE library_items SET total_chapter_num=?, current_chapter_title=?,
		content_revision=content_revision+1, updated_at=? WHERE id=? AND content_revision=?`,
		count, currentTitle, time.Now().UnixMilli(), id, expectedContent)
	return requireMutation(result, err)
}

// ReplaceInterpretationTx also relocates reading state, so it compares both
// versions. Provider catalog and bookmark mappings belong in the same tx.
func ReplaceInterpretationTx(ctx context.Context, tx *sql.Tx, id string, expected Revision, count int, location Location) error {
	if !validLocation(location) {
		return ErrInvalidProgress
	}
	result, err := tx.ExecContext(ctx, `UPDATE library_items SET total_chapter_num=?, dur_chapter_index=?, dur_chapter_pos=?, current_chapter_title=?,
		content_revision=content_revision+1, state_version=state_version+1, updated_at=? WHERE id=? AND content_revision=? AND state_version=?`,
		count, location.ChapterIndex, location.Position, location.ChapterTitle, time.Now().UnixMilli(), id, expected.Content, expected.State)
	return requireMutation(result, err)
}

func requireMutation(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrStateChanged
	}
	return nil
}

func TouchTx(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `UPDATE library_items SET updated_at=? WHERE id=?`, time.Now().UnixMilli(), id)
	return err
}

func UpdateTotalTx(ctx context.Context, tx *sql.Tx, id string, total int) error {
	_, err := tx.ExecContext(ctx, `UPDATE library_items SET total_chapter_num=?, updated_at=? WHERE id=?`, total, time.Now().UnixMilli(), id)
	return err
}
