package library

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrBookmarkConflict = errors.New("library: bookmark ID belongs to a different bookmark")

type Bookmark struct {
	ID              string  `json:"id"`
	BookID          string  `json:"bookId"`
	ContentRevision int64   `json:"contentRevision"`
	ChapterIndex    int     `json:"chapterIndex"`
	ChapterTitle    string  `json:"chapterTitle"`
	Position        float64 `json:"position"`
	Note            string  `json:"note"`
	Orphaned        bool    `json:"orphaned"`
	CreatedAt       int64   `json:"createdAt"`
}

const bookmarkColumns = `id, book_id, content_revision, chapter_index, chapter_title, position, note, orphaned, created_at`

func scanBookmark(row scanner) (*Bookmark, error) {
	var mark Bookmark
	err := row.Scan(&mark.ID, &mark.BookID, &mark.ContentRevision, &mark.ChapterIndex, &mark.ChapterTitle, &mark.Position, &mark.Note, &mark.Orphaned, &mark.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &mark, nil
}

func (s *Store) GetBookmarks(ctx context.Context, itemID string) ([]Bookmark, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+bookmarkColumns+` FROM bookmarks WHERE book_id=? ORDER BY created_at DESC, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readBookmarks(rows)
}

func BookmarksTx(ctx context.Context, tx *sql.Tx, itemID string) ([]Bookmark, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+bookmarkColumns+` FROM bookmarks WHERE book_id=? ORDER BY created_at DESC, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readBookmarks(rows)
}

func readBookmarks(rows *sql.Rows) ([]Bookmark, error) {
	marks := make([]Bookmark, 0)
	for rows.Next() {
		mark, err := scanBookmark(rows)
		if err != nil {
			return nil, err
		}
		marks = append(marks, *mark)
	}
	return marks, rows.Err()
}

func (s *Store) AddBookmark(ctx context.Context, mark *Bookmark, expected Revision) (int64, error) {
	if !validLocation(Location{ChapterIndex: mark.ChapterIndex, Position: mark.Position}) {
		return 0, ErrInvalidProgress
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	item, err := lockItemTx(ctx, tx, mark.BookID)
	if err != nil {
		return 0, err
	}
	if item.ContentRevision != expected.Content {
		return 0, ErrStateChanged
	}
	existing, err := scanBookmark(tx.QueryRowContext(ctx, `SELECT `+bookmarkColumns+` FROM bookmarks WHERE id=?`, mark.ID))
	if err != nil {
		return 0, err
	}
	if existing != nil {
		if existing.BookID != mark.BookID || existing.ContentRevision != expected.Content || existing.ChapterIndex != mark.ChapterIndex || existing.ChapterTitle != mark.ChapterTitle || existing.Position != mark.Position || existing.Note != mark.Note || existing.Orphaned {
			return 0, ErrBookmarkConflict
		}
		*mark = *existing
		return item.StateVersion, nil // Exact retry: no mutation or version increment.
	}
	if item.StateVersion != expected.State {
		return 0, ErrStateChanged
	}
	mark.ContentRevision, mark.CreatedAt, mark.Orphaned = expected.Content, time.Now().UnixMilli(), false
	if _, err := tx.ExecContext(ctx, `INSERT INTO bookmarks (`+bookmarkColumns+`) VALUES (?,?,?,?,?,?,?,?,?)`, mark.ID, mark.BookID, mark.ContentRevision, mark.ChapterIndex, mark.ChapterTitle, mark.Position, mark.Note, mark.Orphaned, mark.CreatedAt); err != nil {
		return 0, err
	}
	if err := advanceStateTx(ctx, tx, mark.BookID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return expected.State + 1, nil
}

func (s *Store) DeleteBookmark(ctx context.Context, itemID, bookmarkID string, expected Revision) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	item, err := lockItemTx(ctx, tx, itemID)
	if err != nil {
		return 0, err
	}
	if item.Revision() != expected {
		return 0, ErrStateChanged
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM bookmarks WHERE book_id=? AND id=?`, itemID, bookmarkID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, ErrNotFound
	}
	if err := advanceStateTx(ctx, tx, itemID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return expected.State + 1, nil
}

// Take SQLite's write reservation before reading bookmark/state snapshots.
// Otherwise a concurrent progress write can make the later read-to-write
// upgrade fail with SQLITE_BUSY instead of yielding the intended CAS conflict.
func lockItemTx(ctx context.Context, tx *sql.Tx, id string) (*Item, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE library_items SET state_version=state_version WHERE id=?`, id); err != nil {
		return nil, err
	}
	item, err := GetTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNotFound
	}
	return item, nil
}

func advanceStateTx(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `UPDATE library_items SET state_version=state_version+1, updated_at=? WHERE id=?`, time.Now().UnixMilli(), id)
	return err
}

// MapBookmarkTx applies provider-resolved correspondence in an interpretation
// transaction. For unresolved bookmarks, callers keep the old revision/location.
func MapBookmarkTx(ctx context.Context, tx *sql.Tx, mark Bookmark) error {
	_, err := tx.ExecContext(ctx, `UPDATE bookmarks SET content_revision=?, chapter_index=?, chapter_title=?, position=?, orphaned=? WHERE id=? AND book_id=?`,
		mark.ContentRevision, mark.ChapterIndex, mark.ChapterTitle, mark.Position, mark.Orphaned, mark.ID, mark.BookID)
	return err
}
