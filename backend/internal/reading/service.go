package reading

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txtstore"
)

var (
	ErrChapterNotFound     = errors.New("reading: chapter not found")
	ErrInvalidLocation     = errors.New("reading: chapter is not readable")
	ErrSourceNotFound      = errors.New("reading: source not found")
	ErrUnsupportedProvider = errors.New("reading: provider not supported")
)

type CrawlError struct {
	Stage string
	Err   error
}

func (e *CrawlError) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e *CrawlError) Unwrap() error { return e.Err }

// Acquisition, recovery, source management and file removal deliberately have
// no place in this interface. Only actual reading operations vary here.
type provider interface {
	catalog(context.Context, string, bool) (Catalog, error)
	open(context.Context, string, int64, int) (Content, error)
	chapter(context.Context, string, int64, int) (Chapter, error)
}

type Service struct {
	Library          *library.Store
	BookSource       *BookSource
	TXT              *txtstore.Store
	EPUB             *epubstore.Store
	EPUBResourceHref func(string, int64, string) string
}

func (s *Service) resolve(ctx context.Context, id string) (provider, *library.Item, error) {
	item, err := s.Library.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if item == nil {
		return nil, nil, library.ErrNotFound
	}
	switch item.Provider {
	case library.BookSource:
		return s.BookSource, item, nil
	case library.EPUB:
		if s.EPUB != nil && s.EPUBResourceHref != nil {
			return epubReader{s.EPUB, s.EPUBResourceHref}, item, nil
		}
	case library.TXT:
		if s.TXT != nil {
			return txtReader{s.TXT}, item, nil
		}
	}
	return nil, nil, ErrUnsupportedProvider
}

func (s *Service) Catalog(ctx context.Context, id string, retry bool) (Catalog, error) {
	p, _, err := s.resolve(ctx, id)
	if err != nil {
		return Catalog{}, err
	}
	return p.catalog(ctx, id, retry)
}

func (s *Service) Open(ctx context.Context, id string, revision int64, index int) (Content, error) {
	return s.open(ctx, id, revision, index, false)
}

// Refresh bypasses fetched BookSource copies. Imported publications are already
// prepared immutable data under their content revision, so they reopen normally.
func (s *Service) Refresh(ctx context.Context, id string, revision int64, index int) (Content, error) {
	return s.open(ctx, id, revision, index, true)
}

func (s *Service) open(ctx context.Context, id string, revision int64, index int, refresh bool) (Content, error) {
	p, item, err := s.resolve(ctx, id)
	if err != nil {
		return Content{}, err
	}
	if item.ContentRevision != revision {
		return Content{}, library.ErrStateChanged
	}
	if refresh && item.Provider == library.BookSource {
		return s.BookSource.openChapter(ctx, id, revision, index, true)
	}
	return p.open(ctx, id, revision, index)
}

func location(ctx context.Context, p provider, id string, revision int64, index int, forProgress bool) (Chapter, error) {
	chapter, err := p.chapter(ctx, id, revision, index)
	if errors.Is(err, ErrChapterNotFound) || err == nil && (chapter.IsVolume || forProgress && chapter.Auxiliary) {
		return Chapter{}, ErrInvalidLocation
	}
	return chapter, err
}

func (s *Service) UpdateProgress(ctx context.Context, id string, expected library.Revision, index int, position float64) (int64, error) {
	p, item, err := s.resolve(ctx, id)
	if err != nil {
		return 0, err
	}
	if item.ContentRevision != expected.Content || item.StateVersion != expected.State {
		return 0, library.ErrStateChanged
	}
	chapter, err := location(ctx, p, id, expected.Content, index, true)
	if err != nil {
		return 0, err
	}
	return s.Library.UpdateProgress(ctx, id, expected, library.Location{ChapterIndex: index, Position: position, ChapterTitle: chapter.Title})
}

func (s *Service) AddBookmark(ctx context.Context, mark *library.Bookmark, expected library.Revision) (int64, error) {
	p, item, err := s.resolve(ctx, mark.BookID)
	if err != nil {
		return 0, err
	}
	if item.ContentRevision != expected.Content {
		return 0, library.ErrStateChanged
	}
	chapter, err := location(ctx, p, mark.BookID, expected.Content, mark.ChapterIndex, false)
	if err != nil {
		return 0, err
	}
	// Location lookup establishes identity; optional title metadata may be empty.
	mark.ChapterTitle = chapter.Title
	// The library owns both CAS and exact repeated-ID idempotency; do not reject
	// the repeated request's old state version before it can recognize that ID.
	return s.Library.AddBookmark(ctx, mark, expected)
}
