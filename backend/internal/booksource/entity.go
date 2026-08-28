// Package booksource implements the legado-compatible BookSource entity and persistence.
package booksource

import (
	"encoding/json"
)

const BookSourceTypeFile = 3

// BookSource mirrors the legado book source JSON format.
// Fields are tagged with db column names for SQLite storage and JSON for import/export.
//
// Unknown imported fields are retained in sourceJSON so imports can be exported without data loss.
type BookSource struct {
	BookSourceURL    string `json:"bookSourceUrl" db:"id"`
	BookSourceName   string `json:"bookSourceName" db:"name"`
	BookSourceGroup  string `json:"bookSourceGroup,omitempty" db:"group_name"`
	BookSourceType   int    `json:"bookSourceType" db:"source_type"`
	BookURLPattern   string `json:"bookUrlPattern,omitempty" db:"book_url_pattern"`
	CustomOrder      int    `json:"customOrder,omitempty" db:"custom_order"`
	Enabled          bool   `json:"enabled" db:"enabled"`
	EnabledExplore   bool   `json:"enabledExplore" db:"enabled_explore"`
	EnabledCookieJar *bool  `json:"enabledCookieJar,omitempty" db:"enabled_cookie_jar"`

	SearchURL     string `json:"searchUrl,omitempty" db:"search_url"`
	ExploreURL    string `json:"exploreUrl,omitempty" db:"explore_url"`
	ExploreScreen string `json:"exploreScreen,omitempty" db:"explore_screen"`

	// Rule fields are stored as JSON strings in the database.
	RuleSearch   string `json:"ruleSearch,omitempty" db:"rule_search"`
	RuleBookInfo string `json:"ruleBookInfo,omitempty" db:"rule_book_info"`
	RuleToc      string `json:"ruleToc,omitempty" db:"rule_toc"`
	RuleContent  string `json:"ruleContent,omitempty" db:"rule_content"`
	RuleExplore  string `json:"ruleExplore,omitempty" db:"rule_explore"`
	RuleReview   string `json:"ruleReview,omitempty" db:"rule_review"`

	JSLib          string `json:"jsLib,omitempty" db:"js_lib"`
	Header         string `json:"header,omitempty" db:"header"`
	LoginURL       string `json:"loginUrl,omitempty" db:"login_url"`
	LoginUI        string `json:"loginUi,omitempty" db:"login_ui"`
	LoginCheckJS   string `json:"loginCheckJs,omitempty" db:"login_check_js"`
	CoverDecodeJS  string `json:"coverDecodeJs,omitempty" db:"cover_decode_js"`
	ConcurrentRate string `json:"concurrentRate,omitempty" db:"concurrent_rate"`

	BookSourceComment string `json:"bookSourceComment,omitempty" db:"comment"`
	VariableComment   string `json:"variableComment,omitempty" db:"variable_comment"`
	LastUpdateTime    int64  `json:"lastUpdateTime,omitempty" db:"last_update_time"`
	RespondTime       int64  `json:"respondTime,omitempty" db:"respond_time"`
	Weight            int    `json:"weight,omitempty" db:"weight"`

	CreatedAt int64 `json:"createdAt" db:"created_at"`
	UpdatedAt int64 `json:"updatedAt" db:"updated_at"`

	CollectionID string `json:"-" db:"collection_id"`
	sourceJSON   string `json:"-" db:"source_json"`
}

// ponytail: flat struct, no sub-objects for rules. Rule JSON strings are parsed on demand.
// Add nested rule structs only when we have a real use case for querying rule sub-fields in SQL.

func (s *BookSource) GetKey() string { return s.BookSourceURL }
func (s *BookSource) GetTag() string { return s.BookSourceName }

// TableName returns the SQLite table name.
func (s *BookSource) TableName() string { return "book_sources" }

// ColumnDefs returns the CREATE TABLE statement for the book_sources table.
func ColumnDefs() string {
	return `CREATE TABLE IF NOT EXISTS book_source_collections (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL COLLATE NOCASE UNIQUE,
		origin_kind TEXT NOT NULL,
		origin_url TEXT,
		origin_filename TEXT,
		sync_interval TEXT NOT NULL DEFAULT 'manual',
		last_attempt_at INTEGER,
		last_success_at INTEGER,
		next_sync_at INTEGER,
		last_error TEXT NOT NULL DEFAULT '',
		etag TEXT NOT NULL DEFAULT '',
		last_modified TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS book_sources (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		group_name TEXT DEFAULT '',
		source_type INTEGER DEFAULT 0,
		book_url_pattern TEXT DEFAULT '',
		custom_order INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 1,
		enabled_explore INTEGER DEFAULT 1,
		enabled_cookie_jar INTEGER DEFAULT 1,
		search_url TEXT DEFAULT '',
		explore_url TEXT DEFAULT '',
		explore_screen TEXT DEFAULT '',
		rule_search TEXT DEFAULT '',
		rule_book_info TEXT DEFAULT '',
		rule_toc TEXT DEFAULT '',
		rule_content TEXT DEFAULT '',
		rule_explore TEXT DEFAULT '',
		rule_review TEXT DEFAULT '',
		js_lib TEXT DEFAULT '',
		header TEXT DEFAULT '',
		login_url TEXT DEFAULT '',
		login_ui TEXT DEFAULT '',
		login_check_js TEXT DEFAULT '',
		cover_decode_js TEXT DEFAULT '',
		concurrent_rate TEXT DEFAULT '',
		comment TEXT DEFAULT '',
		variable_comment TEXT DEFAULT '',
		last_update_time INTEGER DEFAULT 0,
		respond_time INTEGER DEFAULT 180000,
		weight INTEGER DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		source_json TEXT NOT NULL DEFAULT '',
		collection_id TEXT REFERENCES book_source_collections(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS book_sources_collection_id ON book_sources(collection_id);`
}

// NewFromJSON creates a BookSource from raw JSON bytes (single source).
// Defaults are consolidated in UnmarshalJSON — this just unmarshals.
func NewFromJSON(data []byte) (*BookSource, error) {
	var s BookSource
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
