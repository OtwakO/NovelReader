package api

import (
	"database/sql"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
)

func initializeAPITestSchema(t *testing.T, db *sql.DB, schemas ...readerstore.ReaderSchema) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range schemas {
		if err := schema.Initialize(tx); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func initializeBookAPITestSchema(t *testing.T, db *sql.DB) {
	initializeAPITestSchema(t, db, book.ReaderSchema())
}

func initializeBookAndSourceAPITestSchema(t *testing.T, db *sql.DB) {
	initializeAPITestSchema(t, db, booksource.ReaderSchema(), book.ReaderSchema())
}
