package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
)

// Candidate describes preparation, never the currently readable interpretation.
type Candidate struct {
	Generation          int64
	State               State
	Options             txt.Options
	Encoding            txt.Encoding
	BaseContentRevision int64
	Error               string
}

type ReparseStatus struct {
	ActiveGeneration int64
	ContentRevision  int64
	StateVersion     int64
	ActiveOptions    txt.Options
	ActiveEncoding   txt.Encoding
	Candidate        *Candidate
}

func (s *Store) ReparseStatus(ctx context.Context, id string) (ReparseStatus, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReparseStatus{}, err
	}
	defer tx.Rollback()
	status, _, err := reparseStatusTx(ctx, tx, id)
	return status, err
}

func reparseStatusTx(ctx context.Context, tx *sql.Tx, id string) (ReparseStatus, *library.Item, error) {
	var status ReparseStatus
	item, err := library.GetTx(ctx, tx, id)
	if err != nil {
		return status, nil, err
	}
	if item == nil || item.Provider != library.TXT {
		return status, nil, ErrNotFound
	}
	status.ContentRevision, status.StateVersion = item.ContentRevision, item.StateVersion
	err = tx.QueryRowContext(ctx, `SELECT i.generation,i.requested_encoding,i.requested_preset,i.requested_pattern,i.encoding FROM txt_interpretations i JOIN txt_files f ON f.id=i.file_id WHERE f.id=? AND f.library_id=? AND f.state='acquired' AND i.role='active'`, id, id).Scan(&status.ActiveGeneration, &status.ActiveOptions.Encoding, &status.ActiveOptions.Preset, &status.ActiveOptions.Pattern, &status.ActiveEncoding)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil, ErrStateChanged
	}
	if err != nil {
		return status, nil, err
	}
	var candidate Candidate
	err = tx.QueryRowContext(ctx, `SELECT generation,state,requested_encoding,requested_preset,requested_pattern,encoding,base_content_revision,error FROM txt_interpretations WHERE file_id=? AND role='candidate'`, id).Scan(&candidate.Generation, &candidate.State, &candidate.Options.Encoding, &candidate.Options.Preset, &candidate.Options.Pattern, &candidate.Encoding, &candidate.BaseContentRevision, &candidate.Error)
	if errors.Is(err, sql.ErrNoRows) {
		return status, item, nil
	}
	if err != nil {
		return status, nil, err
	}
	status.Candidate = &candidate
	return status, item, nil
}

// Reserve the writer through TXT's own row before reading shared library state.
// This avoids read-to-write upgrades without depending on library table layouts.
func lockPublicationTx(ctx context.Context, tx *sql.Tx, id string) error {
	return requireChange(tx.ExecContext(ctx, `UPDATE txt_files SET generation=generation WHERE id=? AND library_id=? AND state='acquired'`, id, id))
}

func candidateGeneration(status ReparseStatus) int64 {
	if status.Candidate == nil {
		return 0
	}
	return status.Candidate.Generation
}

// QueueReparse replaces only the expected candidate; zero means observed absence.
// Reading-state changes do not invalidate preparation. Notify the existing pool
// after commit. A lost response is resolved with ReparseStatus, not a blind retry.
func (s *Store) QueueReparse(ctx context.Context, id string, contentRevision, expectedCandidate int64, options txt.Options) (int64, error) {
	if err := options.Validate(); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := lockPublicationTx(ctx, tx, id); err != nil {
		return 0, err
	}
	status, _, err := reparseStatusTx(ctx, tx, id)
	if err != nil {
		return 0, err
	}
	if status.ContentRevision != contentRevision || candidateGeneration(status) != expectedCandidate {
		return 0, ErrStateChanged
	}
	var generation int64
	err = tx.QueryRowContext(ctx, `UPDATE txt_files SET generation=generation+1,updated_at=? WHERE id=? RETURNING generation`, time.Now().UnixMilli(), id).Scan(&generation)
	if err != nil {
		return 0, err
	}
	if err := replaceCandidateTx(ctx, tx, id, contentRevision, options); err != nil {
		return 0, err
	}
	return generation, tx.Commit()
}

// DiscardReparse revokes one generation. It never deletes bytes or library state.
func (s *Store) DiscardReparse(ctx context.Context, id string, contentRevision, generation int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockPublicationTx(ctx, tx, id); err != nil {
		return err
	}
	status, _, err := reparseStatusTx(ctx, tx, id)
	if err != nil {
		return err
	}
	if status.ContentRevision != contentRevision || generation <= 0 || candidateGeneration(status) != generation {
		return ErrStateChanged
	}
	if err := requireChange(tx.ExecContext(ctx, `DELETE FROM txt_interpretations WHERE file_id=? AND generation=? AND role='candidate'`, id, generation)); err != nil {
		return err
	}
	return tx.Commit()
}
