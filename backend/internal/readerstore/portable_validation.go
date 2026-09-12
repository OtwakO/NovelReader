package readerstore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// validatePortableHome checks the completed copy, not the changing live database.
func validatePortableHome(ctx context.Context, homePath string, schemas []ReaderSchema) error {
	if err := validateHome(homePath, schemas); err != nil {
		return err
	}
	var validators []func(context.Context, *sql.Tx, *os.Root) error
	for _, schema := range schemas {
		if schema.ValidatePortableFiles != nil {
			validators = append(validators, schema.ValidatePortableFiles)
		}
	}
	if len(validators) == 0 {
		return ctx.Err()
	}
	db, err := sql.Open("sqlite", sqliteFileURI(filepath.Join(homePath, ReaderDatabaseName))+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	root, err := os.OpenRoot(filepath.Join(homePath, FilesDirectory))
	if err != nil {
		return err
	}
	defer root.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, validate := range validators {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validate(ctx, tx, root); err != nil {
			return fmt.Errorf("readerstore: validate portable files: %w", err)
		}
	}
	return nil
}
