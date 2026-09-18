package epubstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

// RecoverPreparations runs after acquisition recovery with all intake/workers
// quiescent. It never reparses originals or automatically publishes a book.
func (s *Store) RecoverPreparations(ctx context.Context) (err error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	root, err := s.files.OpenRoot()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	rows, err := s.db.QueryContext(ctx, `SELECT file_id,generation,state FROM epub_preparations WHERE state IN ('preparing','finalizing')`)
	if err != nil {
		return err
	}
	var attempts []PreparationAttempt
	for rows.Next() {
		var a PreparationAttempt
		if err = rows.Scan(&a.ReceiptID, &a.Generation, &a.State); err != nil {
			return errors.Join(err, rows.Close())
		}
		attempts = append(attempts, a)
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	var failures []error
	for _, a := range attempts {
		if err = ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err = validateID(a.ReceiptID); err == nil {
			if a.State == PreparationRunning {
				err = s.failInterruptedPreparation(ctx, a, "preparation interrupted; retry required")
			} else {
				err = s.recoverFinalization(ctx, root, a)
			}
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("epubstore: recover preparation %s/%d: %w", a.ReceiptID, a.Generation, err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	// This private work subtree contains only disposable EPUB attempts. Do not
	// sweep unknown managed files or another provider's work directory.
	return root.RemoveAll(preparationWorkPath())
}

func (s *Store) recoverFinalization(ctx context.Context, root *os.Root, a PreparationAttempt) error {
	_, stage, _, err := s.preparationMetadata(ctx, a.ReceiptID, a.Generation, PreparationFinalizing)
	if err != nil {
		return err
	}
	if stage != "" {
		if !strings.HasPrefix(stage, "epub-") {
			return errors.New("epubstore: invalid staging identity")
		}
		if err = validateID(strings.TrimPrefix(stage, "epub-")); err != nil {
			return err
		}
	}
	destination := preparationPath(a.ReceiptID, a.Generation)
	if _, err = root.Lstat(destination); errors.Is(err, os.ErrNotExist) {
		// Portable copies have no authority to adopt installation-local work.
		if stage == "" {
			return s.failInterruptedPreparation(ctx, a, "prepared output missing; retry required")
		}
		source := path.Join(receiptPreparationWorkPath(a.ReceiptID), stage)
		if _, err = root.Lstat(source); errors.Is(err, os.ErrNotExist) {
			return s.failInterruptedPreparation(ctx, a, "prepared output missing; retry required")
		}
		if err != nil {
			return err
		}
		if err = root.MkdirAll(path.Dir(destination), 0700); err != nil {
			return err
		}
		if err = root.Rename(source, destination); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err = s.checkInstalledPreparation(ctx, root, a); err != nil {
		// Cancellation/transient I/O must not destroy otherwise recoverable output.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var syntax *json.SyntaxError
		var shape *json.UnmarshalTypeError
		incomplete := errors.Is(err, errInvalidSectionSpan) || errors.Is(err, errIncompletePreparation) || errors.Is(err, os.ErrNotExist) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &syntax) || errors.As(err, &shape)
		if !incomplete {
			return err
		}
		if cleanupErr := root.RemoveAll(destination); cleanupErr != nil {
			return errors.Join(err, cleanupErr)
		}
		return s.failInterruptedPreparation(ctx, a, "prepared output incomplete; retry required: "+err.Error())
	}
	return s.markPreparationReady(ctx, a)
}

func (s *Store) failInterruptedPreparation(ctx context.Context, a PreparationAttempt, message string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE epub_preparations SET state='failed',error=?,stage_name='',updated_at=? WHERE file_id=? AND generation=? AND state=? AND EXISTS(
 SELECT 1 FROM epub_files WHERE id=? AND preparation_generation=? AND state='acquired')`, message, time.Now().UnixMilli(), a.ReceiptID, a.Generation, a.State, a.ReceiptID, a.Generation)
	if err != nil {
		return err
	}
	return changedOne(result)
}
