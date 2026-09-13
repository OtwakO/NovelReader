package txtstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/txt"
)

// List uses the receipt primary key (and state index when filtered). The caller
// bounds limit; an ID cursor is stable but does not imply chronological ordering.
func (s *Store) List(ctx context.Context, after string, state State, limit int) ([]Receipt, error) {
	query := `SELECT ` + receiptColumns + ` FROM txt_files WHERE id>?`
	args := []any{after}
	if state != "" {
		query += ` AND state=?`
		args = append(args, state)
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY id LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Receipt, 0)
	for rows.Next() {
		value, err := scanReceipt(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

type ReviewHeading struct {
	Index     int
	Title     string
	Generated bool
}

type Review struct {
	Version         int64
	Encoding        txt.Encoding
	Preset          txt.Preset
	ParserVersion   int
	ReviewReasons   []txt.ReviewReason
	TotalSections   int
	Headings        []ReviewHeading
	HasMore         bool
	Sample          string
	SampleTruncated bool
}

const reviewSampleBytes = 4096

// Review reads a bounded page of a saved index and a sample of its first section.
// It does not build a full catalog or reparse the original. Callers bound the page
// size and provide the exact analysis version they are reviewing.
func (s *Store) Review(ctx context.Context, id string, version int64, start, limit int) (Review, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()
	value, err := scanReceipt(tx.QueryRowContext(ctx, `SELECT `+receiptColumns+` FROM txt_files WHERE id=?`, id))
	if err != nil {
		return Review{}, err
	}
	if value.AnalysisVersion != version || !hasInterpretation(value.State) {
		return Review{}, ErrStateChanged
	}
	var result Review
	var reasons string
	err = tx.QueryRowContext(ctx, `SELECT analysis_version,encoding,preset,parser_version,review_reasons,(SELECT count(*) FROM txt_sections WHERE receipt_id=?) FROM txt_files WHERE id=?`, id, id).Scan(&result.Version, &result.Encoding, &result.Preset, &result.ParserVersion, &reasons, &result.TotalSections)
	if err != nil {
		return Review{}, err
	}
	if err := json.Unmarshal([]byte(reasons), &result.ReviewReasons); err != nil {
		return Review{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT idx,title,generated,start_byte,end_byte FROM txt_sections WHERE receipt_id=? AND idx>=? ORDER BY idx LIMIT ?`, id, start, limit+1)
	if err != nil {
		return Review{}, err
	}
	result.Headings = make([]ReviewHeading, 0)
	var first txt.Section
	for rows.Next() {
		var heading ReviewHeading
		var section txt.Section
		if err := rows.Scan(&heading.Index, &heading.Title, &heading.Generated, &section.Start, &section.End); err != nil {
			rows.Close()
			return Review{}, err
		}
		if len(result.Headings) == limit {
			result.HasMore = true
			break
		}
		if len(result.Headings) == 0 {
			first = section
		}
		result.Headings = append(result.Headings, heading)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return Review{}, err
	}
	if err := tx.Rollback(); err != nil {
		return Review{}, err
	}
	if len(result.Headings) != 0 {
		file, err := s.openOriginal(value)
		if err != nil {
			return Review{}, err
		}
		text, err := txt.ReadSection(ctx, file, result.Encoding, first)
		if err := errors.Join(err, file.Close()); err != nil {
			return Review{}, err
		}
		end := min(len(text), reviewSampleBytes)
		for !utf8.ValidString(text[:end]) {
			end--
		}
		result.Sample, result.SampleTruncated = text[:end], end < len(text)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return Review{}, err
	}
	if current.AnalysisVersion != version || !hasInterpretation(current.State) {
		return Review{}, ErrStateChanged
	}
	return result, nil
}

func hasInterpretation(state State) bool {
	return state == Ready || state == NeedsReview || state == Published
}
