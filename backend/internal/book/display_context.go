package book

import (
	"context"
	"database/sql"
)

// DisplayContext is the native input to source labels and cover cache revisions.
// Shelf metadata and reading state remain owned by library.
type DisplayContext struct {
	SourceID    string
	SourceURL   string
	BookURL     string
	VariableMap string
	Origin      string
}

// ReadDisplayContextsTx reads all contexts for a shelf, or one for a detail view.
// The caller combines these with library records in the same read transaction.
func ReadDisplayContextsTx(ctx context.Context, tx *sql.Tx, id string) (map[string]DisplayContext, error) {
	query := `SELECT id, source_id, source_url, book_url, variable_map, origin FROM books`
	var args []any
	if id != "" {
		query += ` WHERE id = ?`
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contexts := make(map[string]DisplayContext)
	for rows.Next() {
		var id string
		var value DisplayContext
		if err := rows.Scan(&id, &value.SourceID, &value.SourceURL, &value.BookURL, &value.VariableMap, &value.Origin); err != nil {
			return nil, err
		}
		contexts[id] = value
	}
	return contexts, rows.Err()
}
