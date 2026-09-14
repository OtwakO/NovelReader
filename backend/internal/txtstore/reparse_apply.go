package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/otwako/novelreader/internal/library"
)

var (
	ErrResumeRequired = errors.New("txtstore: choose a resume section before applying")
	ErrInvalidResume  = errors.New("txtstore: resume section is not in the candidate")
)

type ApplyReparseRequest struct {
	Generation       int64
	ActiveGeneration int64
	Expected         library.Revision
	// Nil preserves proven correspondence; an explicit choice starts at position zero.
	ResumeChapter *int
}

type ApplyReparseResult struct {
	Item           library.Item
	AlreadyApplied bool
}

// ApplyReparse changes the active index and shared reading state atomically.
// The mutation gate keeps original validation and publication removal ordered;
// SQLite's writer reservation protects the reviewed versions and correspondence.
func (s *Store) ApplyReparse(ctx context.Context, id string, request ApplyReparseRequest) (ApplyReparseResult, error) {
	var result ApplyReparseResult
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return result, err
	}
	defer unlock()
	value, err := s.Get(ctx, id)
	if err != nil {
		return result, err
	}
	if value.State != Published {
		return result, ErrStateChanged
	}
	// An already-applied retry is a status query, even if later disk trouble exists.
	if value.AnalysisVersion != request.Generation {
		file, err := s.openOriginal(value)
		if err != nil {
			return result, err
		}
		if err := file.Close(); err != nil {
			return result, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err := lockPublicationTx(ctx, tx, id); err != nil {
		return result, err
	}
	status, item, err := reparseStatusTx(ctx, tx, id)
	if err != nil {
		return result, err
	}
	if status.ActiveGeneration == request.Generation {
		return ApplyReparseResult{Item: *item, AlreadyApplied: true}, nil
	}
	if status.ActiveGeneration != request.ActiveGeneration {
		return result, ErrStateChanged
	}
	if err := requireCompletedCandidate(status, request.Generation); err != nil {
		return result, err
	}
	if item.Revision() != request.Expected {
		return result, library.ErrStateChanged
	}
	impact, marks, err := reparseImpactTx(ctx, tx, id, status, item)
	if err != nil {
		return result, err
	}
	resume := impact.Resume
	if request.ResumeChapter != nil {
		resume = &library.Location{ChapterIndex: *request.ResumeChapter}
		err := tx.QueryRowContext(ctx, `SELECT title FROM txt_sections WHERE file_id=? AND generation=? AND idx=?`, id, request.Generation, resume.ChapterIndex).Scan(&resume.ChapterTitle)
		if errors.Is(err, sql.ErrNoRows) {
			return result, ErrInvalidResume
		}
		if err != nil {
			return result, err
		}
	}
	if resume == nil {
		return result, ErrResumeRequired
	}
	if err := library.ReplaceInterpretationTx(ctx, tx, id, request.Expected, impact.TotalSections, *resume); err != nil {
		return result, err
	}
	for _, mark := range marks {
		if err := library.MapBookmarkTx(ctx, tx, mark); err != nil {
			return result, err
		}
	}
	if err := requireChange(tx.ExecContext(ctx, `DELETE FROM txt_interpretations WHERE file_id=? AND generation=? AND role='active'`, id, status.ActiveGeneration)); err != nil {
		return result, err
	}
	if err := requireChange(tx.ExecContext(ctx, `UPDATE txt_interpretations SET role='active',updated_at=? WHERE file_id=? AND generation=? AND role='candidate'`, time.Now().UnixMilli(), id, request.Generation)); err != nil {
		return result, err
	}
	item, err = library.GetTx(ctx, tx, id)
	if err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return ApplyReparseResult{Item: *item}, nil
}
