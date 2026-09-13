package txtstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/otwako/novelreader/internal/txt"
)

type Interpretation struct {
	Version  int64
	Options  txt.Options
	Analysis txt.Analysis
}

// Analyze claims a pending version before decoding outside the file gate. Only
// that version may publish the result. Published interpretations cannot be edited.
func (s *Store) Analyze(ctx context.Context, id string, options txt.Options) (Interpretation, error) {
	value, err := s.claimAnalysis(ctx, `UPDATE txt_files SET state=?, analysis_version=analysis_version+1, requested_encoding=?, requested_preset=?, error='', updated_at=? WHERE id=? AND state IN (?,?,?,?) RETURNING `+receiptColumns, Analyzing, options.Encoding, options.Preset, time.Now().UnixMilli(), id, Received, Ready, NeedsReview, AnalysisFailed)
	if errors.Is(err, ErrNotFound) {
		if _, err := s.Get(ctx, id); err != nil {
			return Interpretation{}, err
		}
		return Interpretation{}, ErrStateChanged
	}
	if err != nil {
		return Interpretation{}, err
	}
	return s.analyzeClaim(ctx, value)
}

// Wait for a connection cancellably, then finish the short claim independently of
// request cancellation. Otherwise UPDATE could commit just as Scan is cancelled,
// leaving a claimed version the worker never received and cannot release.
func (s *Store) claimAnalysis(ctx context.Context, query string, args ...any) (Receipt, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer conn.Close()
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	claimCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	return scanReceipt(conn.QueryRowContext(claimCtx, query, args...))
}

func (s *Store) analyzeClaim(ctx context.Context, value Receipt) (Interpretation, error) {
	file, err := s.openOriginal(value)
	var result txt.Analysis
	if err == nil {
		result, err = txt.Analyze(ctx, file, value.Options)
		err = errors.Join(err, file.Close())
	}
	if err == nil {
		err = s.saveInterpretation(ctx, value.ID, value.AnalysisVersion, result)
	}
	if err != nil {
		failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
		defer cancel()
		state := AnalysisFailed
		if ctx.Err() != nil {
			// Cancellation stops this attempt, not the durable request. A worker
			// paused for shutdown/restore can resume it without losing options.
			state = Received
		}
		_, updateErr := s.db.ExecContext(failureCtx, `UPDATE txt_files SET state=?,error=?,updated_at=? WHERE id=? AND state=? AND analysis_version=?`, state, err.Error(), time.Now().UnixMilli(), value.ID, Analyzing, value.AnalysisVersion)
		return Interpretation{}, errors.Join(err, updateErr)
	}
	return Interpretation{Version: value.AnalysisVersion, Options: value.Options, Analysis: result}, nil
}

func (s *Store) openOriginal(value Receipt) (*os.File, error) {
	if err := validateReceiptPath(value); err != nil {
		return nil, err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(value.Path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Size() != value.Size) {
		err = fmt.Errorf("txtstore: managed original changed; re-import the file")
	}
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func (s *Store) saveInterpretation(ctx context.Context, id string, version int64, result txt.Analysis) error {
	reasons, err := json.Marshal(result.ReviewReasons)
	if err != nil {
		return err
	}
	state := Ready
	if len(result.ReviewReasons) > 0 {
		state = NeedsReview
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updated, err := tx.ExecContext(ctx, `UPDATE txt_files SET state=?,encoding=?,preset=?,parser_version=?,review_reasons=?,updated_at=? WHERE id=? AND state=? AND analysis_version=?`, state, result.Encoding, result.Preset, result.ParserVersion, string(reasons), time.Now().UnixMilli(), id, Analyzing, version)
	if err != nil {
		return err
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrStateChanged
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM txt_sections WHERE receipt_id=?`, id); err != nil {
		return err
	}
	insert, err := tx.PrepareContext(ctx, `INSERT INTO txt_sections(receipt_id,idx,title,start_byte,end_byte,generated) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for index, section := range result.Sections {
		if _, err := insert.ExecContext(ctx, id, index, section.Title, section.Start, section.End, section.Generated); err != nil {
			return err
		}
	}
	return tx.Commit()
}
