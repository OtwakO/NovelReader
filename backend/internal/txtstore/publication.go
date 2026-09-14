package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
)

// Accept explicitly approves this exact preview. Identical display names do not
// merge publications. Retrying an accepted version returns its existing item.
func (s *Store) Accept(ctx context.Context, id string, version int64, name, author string) (library.Item, error) {
	if strings.TrimSpace(name) == "" {
		return library.Item{}, fmt.Errorf("txtstore: a publication name is required")
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return library.Item{}, err
	}
	defer unlock()
	value, err := s.Get(ctx, id)
	if err != nil {
		return library.Item{}, err
	}
	if value.AnalysisVersion != version {
		return library.Item{}, ErrStateChanged
	}
	if value.State == Published {
		item, err := library.NewStore(s.db).Get(ctx, value.LibraryID)
		if err != nil {
			return library.Item{}, err
		}
		if item == nil {
			return library.Item{}, ErrStateChanged
		}
		return *item, nil
	}
	if value.State != Ready && value.State != NeedsReview {
		return library.Item{}, ErrStateChanged
	}
	file, err := s.openOriginal(value)
	if err != nil {
		return library.Item{}, err
	}
	if err := file.Close(); err != nil {
		return library.Item{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return library.Item{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE txt_interpretations SET role='active',updated_at=? WHERE file_id=? AND generation=? AND role='candidate' AND state IN (?,?) AND EXISTS(SELECT 1 FROM txt_files WHERE id=? AND state='acquired' AND library_id IS NULL)`, time.Now().UnixMilli(), id, version, Ready, NeedsReview, id)
	if err != nil {
		return library.Item{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return library.Item{}, err
	}
	if count != 1 {
		return library.Item{}, ErrStateChanged
	}
	item := library.Item{ID: id, Provider: library.TXT, Name: strings.TrimSpace(name), Author: strings.TrimSpace(author), ContentRevision: 1, CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli()}
	if err := tx.QueryRowContext(ctx, `SELECT count(*),COALESCE((SELECT title FROM txt_sections WHERE file_id=? AND generation=? AND idx=0),'') FROM txt_sections WHERE file_id=? AND generation=?`, id, version, id, version).Scan(&item.TotalChapterNum, &item.CurrentChapterTitle); err != nil {
		return library.Item{}, err
	}
	if item.TotalChapterNum == 0 {
		return library.Item{}, fmt.Errorf("txtstore: analysis has no sections")
	}
	if err := library.InsertTx(ctx, tx, item); err != nil {
		return library.Item{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE txt_files SET library_id=? WHERE id=?`, id, id); err != nil {
		return library.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return library.Item{}, err
	}
	return item, nil
}

type SectionContent struct {
	ContentRevision int64
	Title           string
	Text            string
}

// ReadSection loads one indexed range; it never loads the complete index or
// re-analyzes the original. Removal/revision changes invalidate a completed read.
func (s *Store) ReadSection(ctx context.Context, id string, revision int64, index int) (SectionContent, error) {
	var value Receipt
	var encoding txt.Encoding
	var section txt.Section
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SectionContent{}, err
	}
	defer tx.Rollback()
	item, err := library.GetTx(ctx, tx, id)
	if err != nil {
		return SectionContent{}, err
	}
	if item == nil || item.Provider != library.TXT {
		return SectionContent{}, ErrNotFound
	}
	if item.ContentRevision != revision {
		return SectionContent{}, library.ErrStateChanged
	}
	err = tx.QueryRowContext(ctx, `SELECT f.id,f.path,f.size,i.encoding,s.title,s.start_byte,s.end_byte
 FROM txt_files f JOIN txt_interpretations i ON i.file_id=f.id AND i.role='active'
 JOIN txt_sections s ON s.file_id=i.file_id AND s.generation=i.generation
 WHERE f.library_id=? AND f.state='acquired' AND s.idx=?`, id, index).Scan(&value.ID, &value.Path, &value.Size, &encoding, &section.Title, &section.Start, &section.End)
	if errors.Is(err, sql.ErrNoRows) {
		return SectionContent{}, ErrNotFound
	}
	if err != nil {
		return SectionContent{}, err
	}
	if err := tx.Rollback(); err != nil {
		return SectionContent{}, err
	}
	file, err := s.openOriginal(value)
	if err != nil {
		return SectionContent{}, err
	}
	text, err := txt.ReadSection(ctx, file, encoding, section)
	err = errors.Join(err, file.Close())
	if err != nil {
		return SectionContent{}, err
	}
	current, err := library.NewStore(s.db).Get(ctx, id)
	if err != nil {
		return SectionContent{}, err
	}
	if current == nil || current.Provider != library.TXT || current.ContentRevision != revision {
		return SectionContent{}, library.ErrStateChanged
	}
	return SectionContent{ContentRevision: revision, Title: section.Title, Text: text}, nil
}
