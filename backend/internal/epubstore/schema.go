package epubstore

import "database/sql"

// Acquisition/preparation-ownership schema fragment. Not registered with live reader homes:
// Ready-state output, section/resource persistence and portable hooks must
// precede epoch activation.
func initializeSchema(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE epub_files (
 id TEXT PRIMARY KEY,
 original_name TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('receiving','finalizing','acquired','failed','removing')),
 size INTEGER NOT NULL DEFAULT 0 CHECK(size >= 0),
 preparation_generation INTEGER NOT NULL DEFAULT 0 CHECK(preparation_generation >= 0),
 image_mode TEXT NOT NULL CHECK(image_mode IN ('original','optimized')),
 error TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
 ); CREATE INDEX idx_epub_files_state ON epub_files(state,id);
 CREATE TABLE epub_preparations (
 file_id TEXT NOT NULL REFERENCES epub_files(id) ON DELETE CASCADE,
 generation INTEGER NOT NULL CHECK(generation > 0),
 state TEXT NOT NULL CHECK(state IN ('queued','preparing','failed')),
 image_mode TEXT NOT NULL CHECK(image_mode IN ('original','optimized')),
 error TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(file_id,generation)
 )`)
	return err
}
