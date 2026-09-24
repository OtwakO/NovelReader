package txtstore

import (
	"context"

	"github.com/otwako/novelreader/internal/importhistory"
)

type HistoryReceipt struct {
	Receipt
	Status string
}

func (s *Store) History(ctx context.Context, query importhistory.Query) ([]HistoryReceipt, error) {
	where, args := query.Where("txt")
	rows, err := s.db.QueryContext(ctx, `WITH history AS (
 SELECT `+receiptColumns+`, CASE
 WHEN state IN ('receiving','received','analyzing') THEN 'processing'
 WHEN state IN ('failed','analysis_failed') THEN 'failed'
 WHEN state='published' THEN 'added'
 ELSE state END AS history_status FROM txt_receipts)
 SELECT * FROM history WHERE `+where+` ORDER BY created_at DESC,id DESC LIMIT ?`, append(args, query.Limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]HistoryReceipt, 0)
	for rows.Next() {
		var value HistoryReceipt
		value.Receipt, err = scanReceipt(rows, &value.Status)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
