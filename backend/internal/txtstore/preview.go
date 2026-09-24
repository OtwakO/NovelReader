package txtstore

import (
	"context"
	"database/sql"
	"encoding/json"

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
	receipt, err := scanReceipt(tx.QueryRowContext(ctx, `SELECT `+receiptColumns+` FROM txt_receipts WHERE id=?`, id))
	if err != nil {
		return Interpretation{}, err
	}
	if !hasInterpretation(receipt.State) {
		return Interpretation{}, ErrStateChanged
	}
	value.Version, value.Options = receipt.AnalysisVersion, receipt.Options
	var reasons string
	err = tx.QueryRowContext(ctx, `SELECT encoding,preset,parser_version,review_reasons FROM txt_interpretations WHERE file_id=? AND generation=?`, id, value.Version).Scan(&value.Analysis.Encoding, &value.Analysis.Preset, &value.Analysis.ParserVersion, &reasons)
	if err != nil {
		return Interpretation{}, err
	}
	if err := json.Unmarshal([]byte(reasons), &value.Analysis.ReviewReasons); err != nil {
		return Interpretation{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT title,start_byte,end_byte,generated FROM txt_sections WHERE file_id=? AND generation=? ORDER BY idx`, id, value.Version)
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
