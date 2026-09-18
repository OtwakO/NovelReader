package epubstore

import "database/sql"

// EPUB storage schema fragment, not registered with live reader homes.
// Portable validation and application lifecycle integration must precede activation.
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
 state TEXT NOT NULL CHECK(state IN ('queued','preparing','finalizing','ready','failed')),
 image_mode TEXT NOT NULL CHECK(image_mode IN ('original','optimized')),
 error TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 format_version INTEGER NOT NULL DEFAULT 0,
 metadata_json BLOB,
 stage_name TEXT NOT NULL DEFAULT '',
 stream_size INTEGER NOT NULL DEFAULT 0 CHECK(stream_size >= 0),
 CHECK(state NOT IN ('finalizing','ready') OR (format_version > 0 AND metadata_json IS NOT NULL AND stream_size > 0)),
 CHECK((state = 'finalizing') = (stage_name <> '')),
 PRIMARY KEY(file_id,generation)
 );
 CREATE TABLE epub_sections (
 file_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 ordinal INTEGER NOT NULL CHECK(ordinal >= 0),
 offset INTEGER NOT NULL CHECK(offset >= 0),
 length INTEGER NOT NULL CHECK(length > 0),
 PRIMARY KEY(file_id,generation,ordinal),
 FOREIGN KEY(file_id,generation) REFERENCES epub_preparations(file_id,generation) ON DELETE CASCADE
 );
 CREATE TABLE epub_resources (
 file_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 id TEXT NOT NULL,
 source_path TEXT NOT NULL,
 derivative_id TEXT NOT NULL,
 derivative_bytes INTEGER NOT NULL CHECK(derivative_bytes >= 0),
 media_type TEXT NOT NULL,
 width INTEGER NOT NULL CHECK(width > 0),
 height INTEGER NOT NULL CHECK(height > 0),
 PRIMARY KEY(file_id,generation,id),
 UNIQUE(file_id,generation,source_path),
 FOREIGN KEY(file_id,generation) REFERENCES epub_preparations(file_id,generation) ON DELETE CASCADE
 )`)
	return err
}
