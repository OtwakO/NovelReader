// Package chapterresource owns disposable, immutable BookSource image bundles.
// A bundle's expiry is an availability promise, not an LRU eviction hint.
package chapterresource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrCapacity    = errors.New("chapter resources: capacity exhausted")
	ErrUnavailable = errors.New("chapter resources: unavailable")
)

// Limits bound serialized bundles, including their ownership metadata. SQLite
// pages and transaction journals require additional disk space.
type Limits struct {
	ReaderBytes   int64
	TotalBytes    int64
	ReaderBundles int
	TotalBundles  int
}

func DefaultLimits() Limits {
	return Limits{ReaderBytes: 256 << 20, TotalBytes: 1 << 30, ReaderBundles: 2000, TotalBundles: 10000}
}

// Owner is supplied by the authenticated reader runtime, never taken on trust
// from a resource URL. Callers also validate current book/source ownership.
type Owner struct {
	ReaderID       string
	Generation     string
	BookID         string
	Revision       int64
	SourceID       string
	SourceIdentity string
	ChapterIndex   int
}

// Images contains private source recipes, not downloaded remote image bytes.
// Context holds only the document-owned script book/chapter inputs; source
// definitions, credentials and live sessions do not belong in this store.
type Images struct {
	Book    map[string]any `json:"book"`
	Chapter map[string]any `json:"chapter"`
	URLs    []string       `json:"urls"`
}

type Reference struct {
	ID             string
	AvailableUntil time.Time
}

type Store struct {
	db        *sql.DB
	limits    Limits
	now       func() time.Time
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// Open starts the store's expiry housekeeping. The single connection serializes
// short local transactions; callers must resolve resources before fetching them.
func Open(ctx context.Context, path string, limits Limits) (*Store, error) {
	return open(ctx, path, limits, time.Now)
}

func open(ctx context.Context, path string, limits Limits, now func() time.Time) (*Store, error) {
	if limits.ReaderBytes <= 0 || limits.TotalBytes <= 0 || limits.ReaderBundles <= 0 || limits.TotalBundles <= 0 {
		return nil, errors.New("chapter resources: limits must be positive")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	uri := &url.URL{Scheme: "file", Path: path}
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, limits: limits, now: now, done: make(chan struct{})}
	if err := s.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.Cleanup(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	lifetime, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.cleanupLoop(lifetime)
	return s, nil
}

func (s *Store) initialize(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version != 0 && version != 1 {
		return fmt.Errorf("chapter resources: unsupported format %d", version)
	}
	// DELETE journal mode avoids a second long-lived WAL budget. FULL auto-vacuum
	// returns deleted pages at commit; no recurring full database VACUUM is needed.
	_, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=DELETE;
 PRAGMA auto_vacuum=FULL;
 CREATE TABLE IF NOT EXISTS bundles (
 id TEXT PRIMARY KEY,
 reader_id TEXT NOT NULL,
 owner BLOB NOT NULL,
 payload BLOB NOT NULL,
 bytes INTEGER NOT NULL,
 expires_at INTEGER NOT NULL
 );
 CREATE INDEX IF NOT EXISTS bundles_reader ON bundles(reader_id);
 CREATE INDEX IF NOT EXISTS bundles_expiry ON bundles(expires_at);
 PRAGMA user_version=1;`)
	return err
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() { s.cancel(); <-s.done; s.closeErr = s.db.Close() })
	return s.closeErr
}
