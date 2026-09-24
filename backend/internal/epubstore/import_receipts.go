package epubstore

import (
	"context"
	"database/sql"
	"errors"
)

// ImportReceipt is a lightweight coherent status projection. Polling/history do
// not decode metadata or section trees; acquisition and preparation stay distinct.
type ImportReceipt struct {
	Receipt
	PreparationState PreparationState
	PreparationError string
}

const importReceiptColumns = `f.id,f.original_name,f.state,f.size,f.preparation_generation,f.image_mode,f.error,f.created_at,MAX(f.updated_at,COALESCE(p.updated_at,0)),COALESCE(f.library_id,''),COALESCE(p.state,''),COALESCE(p.error,'')`
const importReceiptFrom = ` FROM epub_files f LEFT JOIN epub_preparations p ON p.file_id=f.id AND p.generation=f.preparation_generation`
const importReceiptQuery = `SELECT ` + importReceiptColumns + importReceiptFrom

func scanImportReceipt(row interface{ Scan(...any) error }, extra ...any) (ImportReceipt, error) {
	var r ImportReceipt
	columns := []any{&r.ID, &r.OriginalName, &r.State, &r.Size, &r.PreparationGeneration, &r.ImageMode, &r.Error, &r.CreatedAt, &r.UpdatedAt, &r.LibraryID, &r.PreparationState, &r.PreparationError}
	err := row.Scan(append(columns, extra...)...)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err == nil {
		err = validateID(r.ID)
		r.Path = originalPath(r.ID)
	}
	return r, err
}

func (s *Store) GetImport(ctx context.Context, id string) (ImportReceipt, error) {
	return scanImportReceipt(s.db.QueryRowContext(ctx, importReceiptQuery+` WHERE f.id=?`, id))
}

// ListImports uses a stable ID cursor, not a chronological ordering. HTTP bounds
// the page size; metadata and private resource mappings are never loaded here.
func (s *Store) ListImports(ctx context.Context, after string, limit int) ([]ImportReceipt, error) {
	rows, err := s.db.QueryContext(ctx, importReceiptQuery+` WHERE f.id>? ORDER BY f.id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ImportReceipt, 0)
	for rows.Next() {
		r, err := scanImportReceipt(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
