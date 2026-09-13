package txtstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/otwako/novelreader/internal/txt"
)

// Preview returns a coherent saved interpretation, never a half-written index.
// It is a review/catalog operation, not part of opening an individual section.
func (s *Store) Preview(ctx context.Context, id string) (Interpretation, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Interpretation{}, err
	}
	defer tx.Rollback()
	var value Interpretation
	var state State
	var reasons string
	err = tx.QueryRowContext(ctx, `SELECT state,analysis_version,requested_encoding,requested_preset,encoding,preset,parser_version,review_reasons FROM txt_files WHERE id=?`, id).Scan(&state, &value.Version, &value.Options.Encoding, &value.Options.Preset, &value.Analysis.Encoding, &value.Analysis.Preset, &value.Analysis.ParserVersion, &reasons)
	if errors.Is(err, sql.ErrNoRows) {
		return Interpretation{}, ErrNotFound
	}
	if err != nil {
		return Interpretation{}, err
	}
	if !hasInterpretation(state) {
		return Interpretation{}, ErrStateChanged
	}
	if err := json.Unmarshal([]byte(reasons), &value.Analysis.ReviewReasons); err != nil {
		return Interpretation{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT title,start_byte,end_byte,generated FROM txt_sections WHERE receipt_id=? ORDER BY idx`, id)
	if err != nil {
		return Interpretation{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var section txt.Section
		if err := rows.Scan(&section.Title, &section.Start, &section.End, &section.Generated); err != nil {
			return Interpretation{}, err
		}
		value.Analysis.Sections = append(value.Analysis.Sections, section)
	}
	return value, rows.Err()
}
