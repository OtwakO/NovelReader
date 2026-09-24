package txtstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/txt"
)

// List queries the receipt projection without loading unbounded state. The caller
// bounds limit; an ID cursor is stable but does not imply chronological ordering.
func (s *Store) List(ctx context.Context, after string, state State, limit int) ([]Receipt, error) {
	query := `SELECT ` + receiptColumns + ` FROM txt_receipts WHERE id>?`
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
	return s.review(ctx, id, version, start, limit, false, reviewSampleBytes)
}

// ReviewReparse samples only the named completed candidate of a live publication.
func (s *Store) ReviewReparse(ctx context.Context, id string, generation int64, start, limit int) (Review, error) {
	return s.review(ctx, id, generation, start, limit, true, reviewSampleBytes)
}

// ReviewSection uses the same saved-interpretation guards as review, but returns
// a complete single section. Decoding remains bounded by txt.ReadSection.
func (s *Store) ReviewSection(ctx context.Context, id string, version int64, index int) (Review, error) {
	return s.review(ctx, id, version, index, 1, false, 0)
}

func (s *Store) ReviewReparseSection(ctx context.Context, id string, generation int64, index int) (Review, error) {
	return s.review(ctx, id, generation, index, 1, true, 0)
}

// sampleBytes == 0 requests the whole indexed section rather than a summary.
func (s *Store) review(ctx context.Context, id string, version int64, start, limit int, reparse bool, sampleBytes int) (Review, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()
	value, err := scanReceipt(tx.QueryRowContext(ctx, `SELECT `+receiptColumns+` FROM txt_receipts WHERE id=?`, id))
	if err != nil {
		return Review{}, err
	}
	role := "candidate"
	if reparse {
		if value.State != Published {
			return Review{}, ErrStateChanged
		}
	} else {
		if value.AnalysisVersion != version || !hasInterpretation(value.State) {
			return Review{}, ErrStateChanged
		}
		if value.State == Published {
			role = "active"
		}
	}
	var result Review
	var reasons string
	err = tx.QueryRowContext(ctx, `SELECT generation,encoding,preset,parser_version,review_reasons,(SELECT count(*) FROM txt_sections WHERE file_id=? AND generation=?) FROM txt_interpretations WHERE file_id=? AND generation=? AND role=? AND state IN ('ready','needs_review')`, id, version, id, version, role).Scan(&result.Version, &result.Encoding, &result.Preset, &result.ParserVersion, &reasons, &result.TotalSections)
	if errors.Is(err, sql.ErrNoRows) {
		return Review{}, ErrStateChanged
	}
	if err != nil {
		return Review{}, err
	}
	if err := json.Unmarshal([]byte(reasons), &result.ReviewReasons); err != nil {
		return Review{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT idx,title,generated,start_byte,end_byte FROM txt_sections WHERE file_id=? AND generation=? AND idx>=? ORDER BY idx LIMIT ?`, id, version, start, limit+1)
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
		end := len(text)
		if sampleBytes > 0 {
			end = min(end, sampleBytes)
		}
		for !utf8.ValidString(text[:end]) {
			end--
		}
		result.Sample, result.SampleTruncated = text[:end], end < len(text)
	}
	// Apply/discard must invalidate a sample even when the generation still exists
	// under another role. Do not use the published receipt's active-version alias.
	var current bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM txt_interpretations i JOIN txt_files f ON f.id=i.file_id WHERE i.file_id=? AND i.generation=? AND i.role=? AND i.state IN ('ready','needs_review') AND f.state='acquired' AND COALESCE(f.library_id,'')=?)`, id, version, role, value.LibraryID).Scan(&current)
	if err != nil {
		return Review{}, err
	}
	if !current {
		return Review{}, ErrStateChanged
	}
	return result, nil
}

func hasInterpretation(state State) bool {
	return state == Ready || state == NeedsReview || state == Published
}
