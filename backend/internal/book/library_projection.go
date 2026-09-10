package book

import (
	"context"
	"database/sql"

	"github.com/otwako/novelreader/internal/library"
)

// Book is the BookSource acquisition/reading-context projection. The library
// record alone owns persisted display metadata and reading state.
func (b *Book) libraryItem() library.Item {
	return library.Item{
		ID: b.ID, Provider: library.BookSource, Name: b.Name, Author: b.Author,
		CoverURL: b.CoverURL, Intro: b.Intro, Kind: b.Kind, LastChapter: b.LastChapter,
		UpdateTime: b.UpdateTime, WordCount: b.WordCount, DurChapterIndex: b.DurChapterIndex,
		DurChapterPos: b.DurChapterPos, TotalChapterNum: b.TotalChapterNum,
		CurrentChapterTitle: b.CurrentChapterTitle, ContentRevision: b.ContentRevision,
		StateVersion: b.StateVersion, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func (b *Book) applyLibraryItem(item library.Item) {
	b.ID, b.Provider, b.Name, b.Author = item.ID, item.Provider, item.Name, item.Author
	b.CoverURL, b.Intro, b.Kind = item.CoverURL, item.Intro, item.Kind
	b.LastChapter, b.UpdateTime, b.WordCount = item.LastChapter, item.UpdateTime, item.WordCount
	b.DurChapterIndex, b.DurChapterPos = item.DurChapterIndex, item.DurChapterPos
	b.TotalChapterNum, b.CurrentChapterTitle = item.TotalChapterNum, item.CurrentChapterTitle
	b.ContentRevision, b.StateVersion = item.ContentRevision, item.StateVersion
	b.CreatedAt, b.UpdatedAt = item.CreatedAt, item.UpdatedAt
}

func readBookTx(ctx context.Context, tx *sql.Tx, id string) (*Book, error) {
	item, err := library.GetTx(ctx, tx, id)
	if err != nil || item == nil {
		return nil, err
	}
	if item.Provider != library.BookSource {
		return nil, nil
	}
	b := &Book{}
	b.applyLibraryItem(*item)
	if err := scanBookRow(tx.QueryRowContext(ctx, `SELECT `+bookColumns+` FROM books WHERE id=?`, id), b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) getBookByIdentity(name, author string) (*Book, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM books WHERE identity_name=? AND identity_author=?`, name, author).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetBook(id)
}

func chapterTitleAt(chapters []Chapter, index int) string {
	for _, chapter := range chapters {
		if chapter.Index == index {
			return chapter.Title
		}
	}
	return ""
}

// GetCatalog captures a native catalog and its shared revision in one snapshot.
func (s *Store) GetCatalog(ctx context.Context, id string) ([]Chapter, int64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	item, err := library.GetTx(ctx, tx, id)
	if err != nil {
		return nil, 0, err
	}
	if item == nil || item.Provider != library.BookSource {
		return nil, 0, ErrBookNotFound
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+chapterColumns+` FROM chapters WHERE book_id=? ORDER BY idx`, id)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	chapters, err := scanChapters(rows)
	return chapters, item.ContentRevision, err
}
