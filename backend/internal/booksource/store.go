package booksource

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Store handles BookSource persistence.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ReaderSchema returns the current book-source schema module for fresh initialization and validation.
func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{Initialize: func(tx *sql.Tx) error { return initSchema(tx) }}
}

type schemaExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func initSchema(db schemaExecutor) error {
	_, err := db.Exec(ColumnDefs())
	return err
}

// ponytail: sourceColumns must stay field-synced with scanSource's scan order.
var sourceColumns = `id, book_source_url, name, group_name, source_type, book_url_pattern,
	custom_order, enabled, enabled_explore, enabled_cookie_jar,
	search_url, explore_url, explore_screen,
	rule_search, rule_book_info, rule_toc, rule_content, rule_explore, rule_review,
	js_lib, header, login_url, login_ui, login_check_js, cover_decode_js, concurrent_rate,
	comment, variable_comment, last_update_time, respond_time, weight,
	created_at, updated_at, source_json, COALESCE(collection_id, ''), collection_position`

const collectionAvailableForDiscovery = `(collection_id IS NULL OR EXISTS (
	SELECT 1 FROM book_source_collections WHERE id = book_sources.collection_id AND enabled = 1
))`

// List returns all book sources in deterministic user order.
func (s *Store) List() ([]BookSource, error) {
	rows, err := s.db.Query(`SELECT ` + sourceColumns + ` FROM book_sources ORDER BY custom_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list: %w", err)
	}
	defer rows.Close()
	return scanSources(rows)
}

// ListByCollection returns every definition owned by one collection.
func (s *Store) ListByCollection(ctx context.Context, collectionID string) ([]*BookSource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sourceColumns+` FROM book_sources WHERE collection_id = ? ORDER BY collection_position ASC, id ASC`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("booksource: list collection sources: %w", err)
	}
	defer rows.Close()
	values, err := scanSources(rows)
	if err != nil {
		return nil, err
	}
	sources := make([]*BookSource, len(values))
	for index := range values {
		sources[index] = &values[index]
	}
	return sources, nil
}

func (s *Store) ListEnabled() ([]BookSource, error) {
	rows, err := s.db.Query(`SELECT ` + sourceColumns + ` FROM book_sources
		WHERE enabled = 1 AND ` + collectionAvailableForDiscovery + `
		ORDER BY custom_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list enabled: %w", err)
	}
	defer rows.Close()
	return scanSources(rows)
}

// ListExploreEnabled returns globally enabled sources with enabled Explore definitions.
func (s *Store) ListExploreEnabled() ([]BookSource, error) {
	rows, err := s.db.Query(`SELECT ` + sourceColumns + ` FROM book_sources
		WHERE enabled = 1 AND enabled_explore = 1 AND explore_url != ''
			AND ` + collectionAvailableForDiscovery + `
		ORDER BY custom_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list explore enabled: %w", err)
	}
	defer rows.Close()
	return scanSources(rows)
}

// GetExploreEnabledByID returns a source only when its source and collection settings allow Explore.
func (s *Store) GetExploreEnabledByID(id string) (*BookSource, error) {
	row := s.db.QueryRow(`SELECT `+sourceColumns+` FROM book_sources
		WHERE id = ? AND enabled = 1 AND enabled_explore = 1 AND explore_url != ''
			AND `+collectionAvailableForDiscovery, id)
	src, err := scanSource(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("booksource: get Explore-enabled source %s: %w", id, err)
	}
	return src, nil
}

// DefinitionRevision returns one installed source's cache revision without loading its definition.
func (s *Store) DefinitionRevision(id string) (int64, error) {
	var revision int64
	if err := s.db.QueryRow(`SELECT updated_at FROM book_sources WHERE id = ?`, id).Scan(&revision); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("booksource: get definition revision %s: %w", id, err)
	}
	return revision, nil
}

// DefinitionRevisions returns the cache revision of every installed source without loading definitions.
func (s *Store) DefinitionRevisions() (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT id, updated_at FROM book_sources`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list definition revisions: %w", err)
	}
	defer rows.Close()
	revisions := make(map[string]int64)
	for rows.Next() {
		var id string
		var updatedAt int64
		if err := rows.Scan(&id, &updatedAt); err != nil {
			return revisions, fmt.Errorf("booksource: scan definition revision: %w", err)
		}
		revisions[id] = updatedAt
	}
	if err := rows.Err(); err != nil {
		return revisions, fmt.Errorf("booksource: list definition revisions: %w", err)
	}
	return revisions, nil
}

// GetByID returns one installed source definition by its immutable Source ID.
func (s *Store) GetByID(id string) (*BookSource, error) {
	row := s.db.QueryRow(`SELECT `+sourceColumns+` FROM book_sources WHERE id = ?`, id)
	src, err := scanSource(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("booksource: get %s: %w", id, err)
	}
	return src, nil
}

// Upsert inserts or replaces a source definition while retaining installed ownership metadata.
func (s *Store) Upsert(src *BookSource) error {
	if src.ID == "" {
		id, err := newSourceID()
		if err != nil {
			return err
		}
		src.ID = id
	}
	var collectionID sql.NullString
	var collectionPosition sql.NullInt64
	if err := s.db.QueryRow(`SELECT collection_id, collection_position FROM book_sources WHERE id = ?`, src.ID).Scan(&collectionID, &collectionPosition); err != nil && err != sql.ErrNoRows {
		return err
	}
	if collectionPosition.Valid {
		position := int(collectionPosition.Int64)
		src.collectionPosition = &position
	}
	return upsertSource(context.Background(), s.db, src, collectionID.String, time.Now())
}

// UpdatePreferences changes management preferences without replacing the source definition.
func (s *Store) UpdatePreferences(id string, enabled, enabledExplore *bool) (*BookSource, error) {
	if enabled == nil && enabledExplore == nil {
		return nil, fmt.Errorf("booksource: source preference update is empty")
	}
	result, err := s.db.Exec(`UPDATE book_sources SET
		enabled = COALESCE(?, enabled),
		enabled_explore = COALESCE(?, enabled_explore),
		updated_at = ?
		WHERE id = ?`, nullableBoolValue(enabled), nullableBoolValue(enabledExplore), time.Now().Unix(), id)
	if err != nil {
		return nil, fmt.Errorf("booksource: update preferences %s: %w", id, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("booksource: update preferences result %s: %w", id, err)
	}
	if count == 0 {
		return nil, nil
	}
	return s.GetByID(id)
}

// Delete removes an installed source definition by immutable Source ID.
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM book_sources WHERE id = ?`, id)
	return err
}

// ImportBatch inserts or replaces multiple sources in a transaction.
func (s *Store) ImportBatch(sources []*BookSource) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("booksource: import batch begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now()
	for _, src := range sources {
		id, err := newSourceID()
		if err != nil {
			return 0, err
		}
		src.ID = id
		if err := upsertSource(context.Background(), tx, src, "", now); err != nil {
			return 0, fmt.Errorf("booksource: import batch insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("booksource: import batch commit: %w", err)
	}
	return len(sources), nil
}

// scanSources scans all rows from a query into a slice.
func scanSources(rows *sql.Rows) ([]BookSource, error) {
	var list []BookSource
	for rows.Next() {
		var s BookSource
		if err := scanRow(rows, &s); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// scanSource scans a single row.
func scanSource(row *sql.Row) (*BookSource, error) {
	var s BookSource
	if err := scanRow(row, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// scanRow scans column values into a BookSource.
// ponytail: positional scan matching the column order in ColumnDefs. Brittle if columns change,
// but avoids reflection. Add when schema stabilises.
func scanRow(scanner interface {
	Scan(dest ...interface{}) error
}, s *BookSource) error {
	return scanner.Scan(
		&s.ID, &s.BookSourceURL, &s.BookSourceName, &s.BookSourceGroup, &s.BookSourceType,
		&s.BookURLPattern, &s.CustomOrder,
		&s.Enabled, &s.EnabledExplore, &s.EnabledCookieJar,
		&s.SearchURL, &s.ExploreURL, &s.ExploreScreen,
		&s.RuleSearch, &s.RuleBookInfo, &s.RuleToc, &s.RuleContent, &s.RuleExplore, &s.RuleReview,
		&s.JSLib, &s.Header, &s.LoginURL, &s.LoginUI, &s.LoginCheckJS, &s.CoverDecodeJS, &s.ConcurrentRate,
		&s.BookSourceComment, &s.VariableComment, &s.LastUpdateTime, &s.RespondTime, &s.Weight,
		&s.CreatedAt, &s.UpdatedAt, &s.sourceJSON, &s.CollectionID, &s.collectionPosition,
	)
}

type sourceExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// upsertSource persists both the imported definition and NovelReader-owned installation metadata.
func upsertSource(ctx context.Context, db sourceExecutor, src *BookSource, collectionID string, now time.Time) error {
	src.UpdatedAt = now.UnixMilli()
	if src.CreatedAt == 0 {
		src.CreatedAt = src.UpdatedAt
	}
	src.CollectionID = collectionID
	_, err := db.ExecContext(ctx, `INSERT OR REPLACE INTO book_sources (
		id, book_source_url, name, group_name, source_type, book_url_pattern, custom_order,
		enabled, enabled_explore, enabled_cookie_jar,
		search_url, explore_url, explore_screen,
		rule_search, rule_book_info, rule_toc, rule_content, rule_explore, rule_review,
		js_lib, header, login_url, login_ui, login_check_js, cover_decode_js, concurrent_rate,
		comment, variable_comment, last_update_time, respond_time, weight,
		created_at, updated_at, source_json, collection_id, collection_position
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULLIF(?, ''),?)`,
		src.ID, src.BookSourceURL, src.BookSourceName, src.BookSourceGroup, src.BookSourceType,
		src.BookURLPattern, src.CustomOrder,
		boolToInt(src.Enabled), boolToInt(src.EnabledExplore), nullableBoolValue(src.EnabledCookieJar),
		src.SearchURL, src.ExploreURL, src.ExploreScreen,
		src.RuleSearch, src.RuleBookInfo, src.RuleToc, src.RuleContent, src.RuleExplore, src.RuleReview,
		src.JSLib, src.Header, src.LoginURL, src.LoginUI, src.LoginCheckJS, src.CoverDecodeJS, src.ConcurrentRate,
		src.BookSourceComment, src.VariableComment, src.LastUpdateTime, src.RespondTime, src.Weight,
		src.CreatedAt, src.UpdatedAt, src.sourceJSON, collectionID, src.collectionPosition,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableBoolValue preserves omitted versus explicitly enabled/disabled imported options.
func nullableBoolValue(value *bool) any {
	if value == nil {
		return nil
	}
	return boolToInt(*value)
}
