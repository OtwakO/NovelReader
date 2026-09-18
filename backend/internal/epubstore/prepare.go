package epubstore

import (
	"context"
	"errors"
	"os"
	"path"
	"time"

	"github.com/otwako/novelreader/internal/epub"
)

// Prepare owns claim, original access and staging so callers cannot accidentally
// finalize output from another receipt. Expensive interpretation stays outside
// the mutation gate. Ready output still requires explicit review/admission.
func (s *Store) Prepare(ctx context.Context, id string, generation int64) (err error) {
	attempt, err := s.ClaimPreparation(ctx, id, generation)
	if err != nil {
		return err
	}
	// Only running attempts are failed here. Finalizing intent survives errors so
	// recovery can distinguish an installed generation from a disposable attempt.
	defer func() {
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
			defer cancel()
			failErr := s.FailPreparation(cleanup, id, generation, err)
			if !errors.Is(failErr, ErrStateChanged) {
				err = errors.Join(err, failErr)
			}
		}
	}()
	receipt, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	original, err := root.Open(receipt.Path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, original.Close()) }()
	if err = root.MkdirAll(receiptPreparationWorkPath(id), 0700); err != nil {
		return err
	}
	work, err := root.OpenRoot(receiptPreparationWorkPath(id))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, work.Close()) }()
	staged, err := Stage(ctx, work, original, receipt.Size, attempt.ImageMode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, staged.Close()) }()
	return s.finalizePreparation(ctx, root, attempt, staged)
}

func (s *Store) finalizePreparation(ctx context.Context, root *os.Root, a PreparationAttempt, staged *StagedPreparation) error {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	// Like original acquisition, complete the short installation after a client
	// disconnect once we own the mutation gate. Long preparation remains cancellable.
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	if err = s.persistPreparationIntent(finalCtx, a, staged); err != nil {
		return err
	}
	destination := preparationPath(a.ReceiptID, a.Generation)
	if err = root.MkdirAll(path.Dir(destination), 0700); err != nil {
		return err
	}
	if err = root.Rename(path.Join(receiptPreparationWorkPath(a.ReceiptID), staged.Directory()), destination); err != nil {
		return err
	}
	// Files and index intent are now installed. A failed readiness write leaves
	// finalizing state, which recovery validates rather than publishing blindly.
	return s.markPreparationReady(finalCtx, a)
}

func (s *Store) markPreparationReady(ctx context.Context, a PreparationAttempt) error {
	result, err := s.db.ExecContext(ctx, `UPDATE epub_preparations SET state='ready',stage_name='',updated_at=?
 WHERE file_id=? AND generation=? AND state='finalizing' AND EXISTS(
 SELECT 1 FROM epub_files WHERE id=? AND state='acquired' AND preparation_generation=?)`, time.Now().UnixMilli(), a.ReceiptID, a.Generation, a.ReceiptID, a.Generation)
	if err != nil {
		return err
	}
	return changedOne(result)
}

// PreparedMetadata is preparation evidence, not a library or HTTP projection.
func (s *Store) PreparedMetadata(ctx context.Context, id string, generation int64) (epub.Preparation, error) {
	metadata, _, _, err := s.preparationMetadata(ctx, id, generation, PreparationReady)
	return metadata, err
}
