package txtstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/otwako/novelreader/internal/library"
)

type ReparseImpact struct {
	Generation          int64
	ActiveGeneration    int64
	ContentRevision     int64
	StateVersion        int64
	TotalSections       int
	Resume              *library.Location
	PreservedBookmarks  int
	UnresolvedBookmarks int
}

// ReparseImpact reads both indexes and all affected reading state in one snapshot.
// No full catalogs, original bytes, or client-supplied correspondence are needed.
func (s *Store) ReparseImpact(ctx context.Context, id string, generation int64) (ReparseImpact, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReparseImpact{}, err
	}
	defer tx.Rollback()
	status, item, err := reparseStatusTx(ctx, tx, id)
	if err != nil {
		return ReparseImpact{}, err
	}
	if err := requireCompletedCandidate(status, generation); err != nil {
		return ReparseImpact{}, err
	}
	impact, _, err := reparseImpactTx(ctx, tx, id, status, item)
	return impact, err
}

func requireCompletedCandidate(status ReparseStatus, generation int64) error {
	candidate := status.Candidate
	if candidate == nil || candidate.Generation != generation || candidate.BaseContentRevision != status.ContentRevision || (candidate.State != Ready && candidate.State != NeedsReview) {
		return ErrStateChanged
	}
	return nil
}

// The returned bookmark changes are derived afresh inside Apply's transaction.
// Cached lookups last only for this snapshot and cover distinct referenced sections.
func reparseImpactTx(ctx context.Context, tx *sql.Tx, id string, status ReparseStatus, item *library.Item) (ReparseImpact, []library.Bookmark, error) {
	candidate := status.Candidate
	impact := ReparseImpact{Generation: candidate.Generation, ActiveGeneration: status.ActiveGeneration, ContentRevision: item.ContentRevision, StateVersion: item.StateVersion}
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM txt_sections WHERE file_id=? AND generation=?`, id, candidate.Generation).Scan(&impact.TotalSections)
	if err != nil {
		return impact, nil, err
	}
	matches := make(map[int]*library.Location)
	lookup := func(index int) (*library.Location, error) {
		if result, found := matches[index]; found {
			return result, nil
		}
		var result *library.Location
		if status.ActiveEncoding == candidate.Encoding {
			var location library.Location
			err := tx.QueryRowContext(ctx, `SELECT n.idx,n.title FROM txt_sections o JOIN txt_sections n ON n.file_id=o.file_id AND n.generation=? AND n.start_byte=o.start_byte AND n.end_byte=o.end_byte WHERE o.file_id=? AND o.generation=? AND o.idx=?`, candidate.Generation, id, status.ActiveGeneration, index).Scan(&location.ChapterIndex, &location.ChapterTitle)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if err == nil {
				result = &location
			}
		}
		matches[index] = result
		return result, nil
	}
	resume, err := lookup(item.DurChapterIndex)
	if err != nil {
		return impact, nil, err
	}
	if resume != nil {
		location := *resume
		location.Position = item.DurChapterPos
		impact.Resume = &location
	}
	marks, err := library.BookmarksTx(ctx, tx, id)
	if err != nil {
		return impact, nil, err
	}
	for index := range marks {
		mark := &marks[index]
		if !mark.Orphaned && mark.ContentRevision == item.ContentRevision {
			location, err := lookup(mark.ChapterIndex)
			if err != nil {
				return impact, nil, err
			}
			if location != nil {
				mark.ChapterIndex, mark.ChapterTitle = location.ChapterIndex, location.ChapterTitle
				mark.ContentRevision = item.ContentRevision + 1
				impact.PreservedBookmarks++
				continue
			}
		}
		// Do not overwrite the old revision/location or revive previously orphaned marks.
		mark.Orphaned = true
		impact.UnresolvedBookmarks++
	}
	return impact, marks, nil
}
