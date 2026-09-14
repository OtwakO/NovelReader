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
	value, err := s.Get(ctx, id)
	if err != nil {
		return Interpretation{}, err
	}
	if err := s.QueueAnalysis(ctx, id, value.AnalysisVersion, options); err != nil {
		return Interpretation{}, err
	}
	claim, err := s.claimAnalysis(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Interpretation{}, ErrStateChanged
	}
	if err != nil {
		return Interpretation{}, err
	}
	return s.analyzeClaim(ctx, claim)
}

func (s *Store) analyzeClaim(ctx context.Context, value Receipt) (Interpretation, error) {
	file, err := s.openOriginal(value)
	failureCode := AnalysisStorageError
	var result txt.Analysis
	if err == nil {
		input := &candidateReader{ctx: ctx, store: s, fileID: value.ID, generation: value.AnalysisVersion, input: file}
		if err = input.check(); err == nil {
			result, err = txt.Analyze(ctx, input, value.Options)
			if err != nil {
				failureCode = analysisFailureCode(err)
			}
		}
		if closeErr := file.Close(); closeErr != nil {
			if err == nil {
				failureCode = AnalysisStorageError
			}
			err = errors.Join(err, closeErr)
		}
	}
	if err == nil {
		err = s.saveInterpretation(ctx, value.ID, value.AnalysisVersion, result)
		failureCode = AnalysisStorageError
	}
	if err != nil {
		failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
		defer cancel()
		state := AnalysisFailed
		if ctx.Err() != nil {
			// Cancellation stops this attempt, not the durable request. A worker
			// paused for shutdown/restore can resume it without losing options.
			state = queued
			failureCode = ""
		}
		_, updateErr := s.db.ExecContext(failureCtx, `UPDATE txt_interpretations SET state=?,error=?,updated_at=? WHERE file_id=? AND generation=? AND role='candidate' AND state=?`, state, failureCode, time.Now().UnixMilli(), value.ID, value.AnalysisVersion, Analyzing)
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
	updated, err := tx.ExecContext(ctx, `UPDATE txt_interpretations SET state=?,encoding=?,preset=?,parser_version=?,review_reasons=?,updated_at=? WHERE file_id=? AND generation=? AND role='candidate' AND state=?`, state, result.Encoding, result.Preset, result.ParserVersion, string(reasons), time.Now().UnixMilli(), id, version, Analyzing)
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
	insert, err := tx.PrepareContext(ctx, `INSERT INTO txt_sections(file_id,generation,idx,title,start_byte,end_byte,generated) VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for index, section := range result.Sections {
		if _, err := insert.ExecContext(ctx, id, version, index, section.Title, section.Start, section.End, section.Generated); err != nil {
			return err
		}
	}
	return tx.Commit()
}
