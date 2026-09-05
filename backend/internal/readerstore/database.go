package readerstore

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
)

const (
	// CurrentReaderSchemaVersion is one epoch for the complete current reader schema.
	// Versions 1-4 belonged to the removed development migration history.
	CurrentReaderSchemaVersion      = 8
	CurrentCredentialsSchemaVersion = 2
)

var ErrReaderSchemaMismatch = errors.New("readerstore: reader database schema mismatch")

// ReaderSchema contributes one feature's authoritative current DDL.
type ReaderSchema struct {
	Initialize            func(*sql.Tx) error
	InitializeCredentials func(*sql.Tx) error
}

func initializeCredentialsDatabase(path string, schemas []ReaderSchema) (err error) {
	db, err := sql.Open("sqlite", sqliteFileURI(path))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("readerstore: close initialized credentials database: %w", closeErr)
		}
	}()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("readerstore: begin credentials schema initialization: %w", err)
	}
	for _, schema := range schemas {
		if schema.InitializeCredentials != nil {
			if err := schema.InitializeCredentials(tx); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("readerstore: initialize credentials schema: %w", err)
			}
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, CurrentCredentialsSchemaVersion)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("readerstore: stamp credentials schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("readerstore: commit credentials schema: %w", err)
	}
	return nil
}

func initializeReaderDatabase(path string, schemas []ReaderSchema) (err error) {
	db, err := sql.Open("sqlite", sqliteFileURI(path))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("readerstore: close initialized reader database: %w", closeErr)
		}
	}()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("readerstore: begin reader schema initialization: %w", err)
	}
	for _, schema := range schemas {
		if err := schema.Initialize(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("readerstore: initialize reader schema: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, CurrentReaderSchemaVersion)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("readerstore: stamp reader schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("readerstore: commit reader schema: %w", err)
	}
	return nil
}

func openHomeDatabase(path string) (*sql.DB, error) {
	query := url.Values{}
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "cache_size(-8000)")
	dsn := sqliteFileURI(path) + "?" + query.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("readerstore: open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("readerstore: ping database: %w", err)
	}
	return db, nil
}

func validateCredentialsDatabase(path string, schemas []ReaderSchema) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version != CurrentCredentialsSchemaVersion {
		return fmt.Errorf("readerstore: credentials database schema version mismatch: found %d, expected %d", version, CurrentCredentialsSchemaVersion)
	}
	reference, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}
	defer reference.Close()
	tx, err := reference.Begin()
	if err != nil {
		return err
	}
	for _, schema := range schemas {
		if schema.InitializeCredentials != nil {
			if err := schema.InitializeCredentials(tx); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	expected, err := declaredReaderSchema(reference)
	if err != nil {
		return err
	}
	actual, err := declaredReaderSchema(db)
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("readerstore: credentials database schema mismatch")
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("readerstore: credentials database schema mismatch")
		}
	}
	return nil
}

func validateHomeDatabase(path string, expectedVersion int, schemas []ReaderSchema) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version != expectedVersion {
		return schemaMismatchError("version found %d, expected %d", version, expectedVersion)
	}
	reference, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return schemaMismatchError("open schema reference: %v", err)
	}
	defer reference.Close()
	tx, err := reference.Begin()
	if err != nil {
		return schemaMismatchError("begin schema reference: %v", err)
	}
	for _, schema := range schemas {
		if err := schema.Initialize(tx); err != nil {
			_ = tx.Rollback()
			return schemaMismatchError("build schema reference: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return schemaMismatchError("commit schema reference: %v", err)
	}
	expected, err := declaredReaderSchema(reference)
	if err != nil {
		return schemaMismatchError("read schema reference: %v", err)
	}
	actual, err := declaredReaderSchema(db)
	if err != nil {
		return schemaMismatchError("read reader schema: %v", err)
	}
	if len(actual) != len(expected) {
		return schemaMismatchError("declared object count is %d, expected %d", len(actual), len(expected))
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return schemaMismatchError("declared object %q does not match the current schema", actual[index].name)
		}
	}
	return nil
}

type readerSchemaObject struct {
	kind, name, tableName, sql string
}

func declaredReaderSchema(db *sql.DB) ([]readerSchemaObject, error) {
	rows, err := db.Query(`SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master ORDER BY type, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var objects []readerSchemaObject
	for rows.Next() {
		var object readerSchemaObject
		if err := rows.Scan(&object.kind, &object.name, &object.tableName, &object.sql); err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

func schemaMismatchError(format string, args ...any) error {
	detail := fmt.Sprintf(format, args...)
	return fmt.Errorf("%w: %s; stop NovelReader, remove or rename this disposable reader home or DATA_DIR, restart, and re-import test BookSources", ErrReaderSchemaMismatch, detail)
}

func sqliteFileURI(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && len(slashPath) >= 2 && slashPath[1] == ':' {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}
