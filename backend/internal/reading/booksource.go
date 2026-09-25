package reading

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/chapterresource"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/processor"
)

// BookSource adapts native catalogs, crawls and processed caches. Resource URLs
// are issued by the transport, not copied from upstream content into documents.
type BookSource struct {
	chapters        chapterFlights
	Store           *book.Store
	Sources         *booksource.Store
	Catalogs        *book.Catalogs
	Searcher        *book.Searcher
	ProcessorConfig processor.Config
	Resources       *chapterresource.Store
	ResourceOwner   chapterresource.Owner
	ImageHref       func(bookID string, revision int64, chapterIndex, imageIndex int, bundleID string) string
}

func (p *BookSource) catalog(_ context.Context, id string, retry bool) (Catalog, error) {
	if p.Catalogs == nil {
		return Catalog{}, errors.New("reading: catalog service unavailable")
	}
	var result book.CatalogResult
	if retry {
		result = p.Catalogs.Retry(id)
	} else {
		result = p.Catalogs.Get(id)
	}
	switch result.State {
	case book.CatalogSyncing:
		return Catalog{Syncing: true}, nil
	case book.CatalogReady:
		chapters := make([]Chapter, len(result.Chapters))
		for i, ch := range result.Chapters {
			chapters[i] = Chapter{Index: ch.Index, Title: ch.Title, IsVolume: ch.IsVolume}
		}
		return Catalog{Chapters: chapters, ContentRevision: result.ContentRevision}, nil
	default:
		switch result.Failure {
		case book.CatalogFailureBookNotFound:
			return Catalog{}, library.ErrNotFound
		case book.CatalogFailureSourceNotFound:
			return Catalog{}, ErrSourceNotFound
		case book.CatalogFailureStorage:
			return Catalog{}, result.Err
		default:
			err := result.Err
			if err == nil {
				err = errors.New("catalog synchronization failed")
			}
			return Catalog{}, &CrawlError{Stage: "toc", Err: err}
		}
	}
}

func (p *BookSource) chapter(ctx context.Context, id string, revision int64, index int) (Chapter, error) {
	item, ch, _, err := p.Store.GetChapterSnapshot(ctx, id, index)
	if err != nil {
		return Chapter{}, err
	}
	if item == nil {
		return Chapter{}, library.ErrNotFound
	}
	if item.ContentRevision != revision {
		return Chapter{}, library.ErrStateChanged
	}
	if ch == nil {
		return Chapter{}, ErrChapterNotFound
	}
	return Chapter{Index: ch.Index, Title: ch.Title, IsVolume: ch.IsVolume}, nil
}
