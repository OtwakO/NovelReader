package epubstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/library"
)

// Accept publishes exactly the reviewed ready generation. Receipt generations are
// preparation identity, not library revisions; initial content always starts at 1.
func (s *Store) Accept(ctx context.Context, id string, generation int64, name, author string) (library.Item, error) {
	if strings.TrimSpace(name) == "" {
		return library.Item{}, fmt.Errorf("epubstore: a publication name is required")
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return library.Item{}, err
	}
	defer unlock()
	r, err := s.Get(ctx, id)
	if err != nil {
		return library.Item{}, err
	}
	if r.State != Acquired || r.PreparationGeneration != generation {
		return library.Item{}, ErrStateChanged
	}
	if r.LibraryID != "" {
		item, err := library.NewStore(s.db).Get(ctx, r.LibraryID)
		if err != nil {
			return library.Item{}, err
		}
		if item == nil || item.Provider != library.EPUB {
			return library.Item{}, ErrStateChanged
		}
		return *item, nil
	}
	prepared, _, _, err := s.preparationMetadata(ctx, id, generation, PreparationReady)
	if err != nil {
		return library.Item{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return library.Item{}, err
	}
	defer tx.Rollback()
	item := library.Item{ID: id, Provider: library.EPUB, Name: strings.TrimSpace(name), Author: strings.TrimSpace(author), ContentRevision: 1, CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli(), TotalChapterNum: len(prepared.Sections)}
	first := -1
	for i, section := range prepared.Sections {
		if section.Main {
			first = i
			item.DurChapterIndex = i
			item.CurrentChapterTitle = section.Title
			break
		}
	}
	if first < 0 {
		return library.Item{}, fmt.Errorf("epubstore: publication has no main sections")
	}
	if prepared.Cover != nil {
		if err = tx.QueryRowContext(ctx, `SELECT id FROM epub_resources WHERE file_id=? AND generation=? AND source_path=?`, id, generation, prepared.Cover.Reference.Path).Scan(&item.CoverURL); err != nil {
			return library.Item{}, err
		}
	}
	if err = library.InsertTx(ctx, tx, item); err != nil {
		return library.Item{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE epub_files SET library_id=?,updated_at=? WHERE id=? AND library_id IS NULL AND state='acquired' AND preparation_generation=? AND EXISTS(SELECT 1 FROM epub_preparations WHERE file_id=? AND generation=? AND state='ready')`, id, item.UpdatedAt, id, generation, id, generation)
	if err != nil {
		return library.Item{}, err
	}
	if err = changedOne(result); err != nil {
		return library.Item{}, err
	}
	if err = tx.Commit(); err != nil {
		return library.Item{}, err
	}
	return item, nil
}

// RemovePublication hides the library item and persists cleanup intent atomically.
// Ready published generations cannot be requeued, so there is no worker to retire.
// Retry is safe after filesystem or final bookkeeping cleanup fails.
func (s *Store) RemovePublication(ctx context.Context, id string) (pending bool, err error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return false, err
	}
	defer unlock()
	r, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if r.State != Removing && r.LibraryID == "" {
		return false, ErrStateChanged
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE epub_files SET state='removing',error='',updated_at=? WHERE id=? AND COALESCE(library_id,'')=?`, time.Now().UnixMilli(), id, r.LibraryID)
	if err != nil {
		return false, err
	}
	if err = changedOne(result); err != nil {
		return false, err
	}
	if r.LibraryID != "" {
		if err = library.DeleteTx(ctx, tx, r.LibraryID); err != nil {
			return false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return true, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	err = s.finishRemoval(ctx, root, r)
	return err != nil, err
}
