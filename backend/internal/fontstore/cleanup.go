package fontstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Cleanup retries committed file retirements. Missing files mean cleanup already
// succeeded before an interruption; the record can safely be removed.
func (s *Store) Cleanup(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Take the write lock before reading the queue; never remove a live file.
	if _, err := tx.ExecContext(ctx, `DELETE FROM font_cleanup WHERE file_name IN (SELECT file_name FROM fonts)`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT file_name FROM font_cleanup`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.removeFile(name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM font_cleanup WHERE file_name = ?`, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) removeFile(name string) error {
	err := s.files.Remove(readerstore.FontsDirectory, name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("fontstore: remove file %s: %w", name, err)
	}
	return nil
}
