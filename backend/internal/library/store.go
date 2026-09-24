package library

import (
	"context"
	"database/sql"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

const itemColumns = `id, provider, name, author, cover_url, intro, kind, last_chapter, update_time, word_count,
	dur_chapter_index, dur_chapter_pos, total_chapter_num, current_chapter_title, content_revision, state_version, created_at, updated_at, last_read_at`

type scanner interface{ Scan(...any) error }

func scanItem(row scanner) (*Item, error) {
	var item Item
	err := row.Scan(&item.ID, &item.Provider, &item.Name, &item.Author, &item.CoverURL, &item.Intro, &item.Kind,
		&item.LastChapter, &item.UpdateTime, &item.WordCount, &item.DurChapterIndex, &item.DurChapterPos,
		&item.TotalChapterNum, &item.CurrentChapterTitle, &item.ContentRevision, &item.StateVersion, &item.CreatedAt, &item.UpdatedAt, &item.LastReadAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) Get(ctx context.Context, id string) (*Item, error) {
	return scanItem(s.db.QueryRowContext(ctx, `SELECT `+itemColumns+` FROM library_items WHERE id = ?`, id))
}

// GetTx composes with provider operations in the caller's transaction.
func GetTx(ctx context.Context, tx *sql.Tx, id string) (*Item, error) {
	return scanItem(tx.QueryRowContext(ctx, `SELECT `+itemColumns+` FROM library_items WHERE id = ?`, id))
}

func (s *Store) List(ctx context.Context) ([]Item, error) {
	return listItems(ctx, s.db)
}

func ListTx(ctx context.Context, tx *sql.Tx) ([]Item, error) {
	return listItems(ctx, tx)
}

type itemQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func listItems(ctx context.Context, query itemQuery) ([]Item, error) {
	rows, err := query.QueryContext(ctx, `SELECT `+itemColumns+` FROM library_items ORDER BY updated_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// InsertTx never merges identity or commits; the admission use case owns both.
func InsertTx(ctx context.Context, tx *sql.Tx, item Item) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO library_items (`+itemColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID, item.Provider, item.Name, item.Author, item.CoverURL, item.Intro, item.Kind, item.LastChapter, item.UpdateTime, item.WordCount,
		item.DurChapterIndex, item.DurChapterPos, item.TotalChapterNum, item.CurrentChapterTitle, item.ContentRevision, item.StateVersion, item.CreatedAt, item.UpdatedAt, item.LastReadAt)
	return err
}

func UpdateMetadataTx(ctx context.Context, tx *sql.Tx, item Item) error {
	_, err := tx.ExecContext(ctx, `UPDATE library_items SET name=?, author=?, cover_url=?, intro=?, kind=?, last_chapter=?, update_time=?, word_count=?, updated_at=? WHERE id=?`,
		item.Name, item.Author, item.CoverURL, item.Intro, item.Kind, item.LastChapter, item.UpdateTime, item.WordCount, item.UpdatedAt, item.ID)
	return err
}

func DeleteTx(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM library_items WHERE id=?`, id)
	return err
}

// PutTx preserves the explicit replace-by-ID behavior of internal admission
// callers. It does not merge display identity; ordinary admission uses InsertTx.
// Replacing an existing item preserves its reading-activity timestamp.
func PutTx(ctx context.Context, tx *sql.Tx, item Item) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO library_items (`+itemColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, name=excluded.name, author=excluded.author,
		cover_url=excluded.cover_url, intro=excluded.intro, kind=excluded.kind, last_chapter=excluded.last_chapter,
		update_time=excluded.update_time, word_count=excluded.word_count, dur_chapter_index=excluded.dur_chapter_index,
		dur_chapter_pos=excluded.dur_chapter_pos, total_chapter_num=excluded.total_chapter_num,
		current_chapter_title=excluded.current_chapter_title, content_revision=excluded.content_revision,
		state_version=excluded.state_version, updated_at=excluded.updated_at`,
		item.ID, item.Provider, item.Name, item.Author, item.CoverURL, item.Intro, item.Kind, item.LastChapter, item.UpdateTime, item.WordCount,
		item.DurChapterIndex, item.DurChapterPos, item.TotalChapterNum, item.CurrentChapterTitle, item.ContentRevision, item.StateVersion, item.CreatedAt, item.UpdatedAt, item.LastReadAt)
	return err
}
