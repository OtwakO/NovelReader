package epubstore

import (
	"context"

	"github.com/otwako/novelreader/internal/importhistory"
)

type HistoryReceipt struct {
	ImportReceipt
	Status string
}

// History inspects only the saved diagnostics array, not section/resource files.
// Diagnostics is the same review gate as Review; ImageProcessing notices are not
// content warnings. CAST preserves compatibility with JSON stored as a BLOB.
func (s *Store) History(ctx context.Context, query importhistory.Query) ([]HistoryReceipt, error) {
	where, args := query.Where("epub")
	rows, err := s.db.QueryContext(ctx, `WITH history AS (
 SELECT `+importReceiptColumns+`, CASE
 WHEN f.state='removing' THEN 'removing'
 WHEN f.state='failed' THEN 'failed'
 WHEN f.library_id IS NOT NULL THEN 'added'
 WHEN f.state='acquired' AND p.state='failed' THEN 'failed'
 WHEN f.state='acquired' AND p.state='ready' THEN
  CASE WHEN json_array_length(CAST(p.metadata_json AS TEXT),'$.Diagnostics') > 0 THEN 'needs_review' ELSE 'ready' END
 ELSE 'processing' END AS history_status`+importReceiptFrom+`)
 SELECT * FROM history WHERE `+where+` ORDER BY created_at DESC,id DESC LIMIT ?`, append(args, query.Limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]HistoryReceipt, 0)
	for rows.Next() {
		var value HistoryReceipt
		value.ImportReceipt, err = scanImportReceipt(rows, &value.Status)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
