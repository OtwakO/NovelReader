package epubstore

import (
	"context"
	"database/sql"
	"fmt"
)

// Validate both association directions; SQLite foreign keys alone cannot prove
// provider identity, ready interpretation ownership or the library's inventory.
func validatePortablePublications(ctx context.Context, tx *sql.Tx) error {
	var invalid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(
 SELECT 1 FROM epub_files f LEFT JOIN library_items l ON l.id=f.library_id
 LEFT JOIN epub_preparations p ON p.file_id=f.id AND p.generation=f.preparation_generation
 WHERE f.library_id IS NOT NULL AND (f.library_id!=f.id OR l.id IS NULL OR l.provider!='epub' OR f.state!='acquired' OR p.state IS NULL OR p.state!='ready' OR l.content_revision!=1
 OR l.total_chapter_num!=(SELECT count(*) FROM epub_sections s WHERE s.file_id=f.id AND s.generation=f.preparation_generation)
 OR NOT EXISTS(SELECT 1 FROM epub_sections s WHERE s.file_id=f.id AND s.generation=f.preparation_generation AND s.ordinal=l.dur_chapter_index AND s.main=1 AND s.title=l.current_chapter_title))
 UNION ALL SELECT 1 FROM library_items l LEFT JOIN epub_files f ON f.library_id=l.id WHERE l.provider='epub' AND f.id IS NULL
 )`).Scan(&invalid)
	if err != nil {
		return err
	}
	if invalid {
		return fmt.Errorf("epubstore: invalid publication binding or inventory")
	}
	rows, err := tx.QueryContext(ctx, `SELECT f.id,f.preparation_generation,l.cover_url FROM epub_files f JOIN library_items l ON l.id=f.library_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, cover string
		var generation int64
		if err = rows.Scan(&id, &generation, &cover); err != nil {
			return err
		}
		metadata, err := portableMetadata(ctx, tx, id, generation)
		if err != nil {
			return err
		}
		if metadata == nil {
			return fmt.Errorf("epubstore: missing publication metadata")
		}
		if metadata.Cover == nil {
			if cover != "" {
				return fmt.Errorf("epubstore: unexpected publication cover")
			}
			continue
		}
		var registered string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM epub_resources WHERE file_id=? AND generation=? AND source_path=?`, id, generation, metadata.Cover.Reference.Path).Scan(&registered); err != nil {
			return err
		}
		if cover != registered {
			return fmt.Errorf("epubstore: wrong publication cover")
		}
	}
	return rows.Err()
}
