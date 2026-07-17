package booksource

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store handles BookSource persistence.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init creates the table if it doesn't exist.
func (s *Store) Init() error {
	if _, err := s.db.Exec(ColumnDefs()); err != nil {
		return err
	}
	// Add the lossless import column to databases created before source_json existed.
	if _, err := s.db.Exec(`ALTER TABLE book_sources ADD COLUMN source_json TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return fmt.Errorf("booksource: migrate source_json: %w", err)
	}
	return nil
}

// ponytail: sourceColumns must stay field-synced with scanSource's scan order.
var sourceColumns = `id, name, group_name, source_type, book_url_pattern,
	custom_order, enabled, enabled_explore, enabled_cookie_jar,
	search_url, explore_url, explore_screen,
	rule_search, rule_book_info, rule_toc, rule_content, rule_explore, rule_review,
	js_lib, header, login_url, login_ui, login_check_js, cover_decode_js, concurrent_rate,
	comment, variable_comment, last_update_time, respond_time, weight,
	created_at, updated_at, source_json`

// List returns all book sources in deterministic user order.
func (s *Store) List() ([]BookSource, error) {
	rows, err := s.db.Query(`SELECT ` + sourceColumns + ` FROM book_sources ORDER BY custom_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list: %w", err)
	}
	defer rows.Close()
	return scanSources(rows)
}

// ListEnabled returns enabled sources, used for search.
func (s *Store) ListEnabled() ([]BookSource, error) {
	rows, err := s.db.Query(`SELECT ` + sourceColumns + ` FROM book_sources WHERE enabled = 1 ORDER BY custom_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list enabled: %w", err)
	}
	defer rows.Close()
	return scanSources(rows)
}

// ListExploreEnabled returns independently enabled sources with Explore definitions.
func (s *Store) ListExploreEnabled() ([]BookSource, error) {
	rows, err := s.db.Query(`SELECT ` + sourceColumns + ` FROM book_sources WHERE enabled_explore = 1 AND explore_url != '' ORDER BY custom_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list explore enabled: %w", err)
	}
	defer rows.Close()
	return scanSources(rows)
}

// GetByID returns a single source by its URL (primary key).
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

// Upsert inserts or replaces a book source.
func (s *Store) Upsert(src *BookSource) error {
	now := time.Now().UnixMilli()
	src.UpdatedAt = now
	if src.CreatedAt == 0 {
		src.CreatedAt = now
	}
	// ponytail: simple REPLACE — no merge logic. Import overwrites existing fields.
	_, err := s.db.Exec(`INSERT OR REPLACE INTO book_sources (
		id, name, group_name, source_type, book_url_pattern, custom_order,
		enabled, enabled_explore, enabled_cookie_jar,
		search_url, explore_url, explore_screen,
		rule_search, rule_book_info, rule_toc, rule_content, rule_explore, rule_review,
		js_lib, header, login_url, login_ui, login_check_js, cover_decode_js, concurrent_rate,
		comment, variable_comment, last_update_time, respond_time, weight,
		created_at, updated_at, source_json
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		src.BookSourceURL, src.BookSourceName, src.BookSourceGroup, src.BookSourceType,
		src.BookURLPattern, src.CustomOrder,
		boolToInt(src.Enabled), boolToInt(src.EnabledExplore), boolToIntPtr(src.EnabledCookieJar),
		src.SearchURL, src.ExploreURL, src.ExploreScreen,
		src.RuleSearch, src.RuleBookInfo, src.RuleToc, src.RuleContent, src.RuleExplore, src.RuleReview,
		src.JSLib, src.Header, src.LoginURL, src.LoginUI, src.LoginCheckJS, src.CoverDecodeJS, src.ConcurrentRate,
		src.BookSourceComment, src.VariableComment, src.LastUpdateTime, src.RespondTime, src.Weight,
		src.CreatedAt, src.UpdatedAt, src.sourceJSON,
	)
	return err
}

// Delete removes a source by its URL.
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

	for _, src := range sources {
		now := time.Now().UnixMilli()
		src.UpdatedAt = now
		if src.CreatedAt == 0 {
			src.CreatedAt = now
		}
		_, err := tx.Exec(`INSERT OR REPLACE INTO book_sources VALUES (
			?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?
		)`,
			src.BookSourceURL, src.BookSourceName, src.BookSourceGroup, src.BookSourceType,
			src.BookURLPattern, src.CustomOrder,
			boolToInt(src.Enabled), boolToInt(src.EnabledExplore), boolToIntPtr(src.EnabledCookieJar),
			src.SearchURL, src.ExploreURL, src.ExploreScreen,
			src.RuleSearch, src.RuleBookInfo, src.RuleToc, src.RuleContent, src.RuleExplore, src.RuleReview,
			src.JSLib, src.Header, src.LoginURL, src.LoginUI, src.LoginCheckJS, src.CoverDecodeJS, src.ConcurrentRate,
			src.BookSourceComment, src.VariableComment, src.LastUpdateTime, src.RespondTime, src.Weight,
			src.CreatedAt, src.UpdatedAt, src.sourceJSON,
		)
		if err != nil {
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
		&s.BookSourceURL, &s.BookSourceName, &s.BookSourceGroup, &s.BookSourceType,
		&s.BookURLPattern, &s.CustomOrder,
		&s.Enabled, &s.EnabledExplore, &s.EnabledCookieJar,
		&s.SearchURL, &s.ExploreURL, &s.ExploreScreen,
		&s.RuleSearch, &s.RuleBookInfo, &s.RuleToc, &s.RuleContent, &s.RuleExplore, &s.RuleReview,
		&s.JSLib, &s.Header, &s.LoginURL, &s.LoginUI, &s.LoginCheckJS, &s.CoverDecodeJS, &s.ConcurrentRate,
		&s.BookSourceComment, &s.VariableComment, &s.LastUpdateTime, &s.RespondTime, &s.Weight,
		&s.CreatedAt, &s.UpdatedAt, &s.sourceJSON,
	)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func boolToIntPtr(b *bool) int {
	if b == nil {
		return 1 // default true for legacy sources
	}
	return boolToInt(*b)
}
