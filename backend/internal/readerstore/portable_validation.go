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

// preparePortableDatabase strips local-only state after copying and on import.
func preparePortableDatabase(ctx context.Context, filename string, schemas []ReaderSchema) error {
	var prepare []func(context.Context, *sql.Tx) error
	for _, schema := range schemas {
		if schema.PreparePortable != nil {
			prepare = append(prepare, schema.PreparePortable)
		}
	}
	if len(prepare) == 0 {
		return ctx.Err()
	}
	// Reject incompatible input before feature callbacks touch its staged copy.
	if err := validateHomeDatabase(filename, CurrentReaderSchemaVersion, schemas); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", sqliteFileURI(filename)+"?mode=rw")
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, callback := range prepare {
		if err := callback(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}
