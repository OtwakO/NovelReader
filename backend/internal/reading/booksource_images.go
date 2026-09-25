package reading

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/chapterresource"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/processor"
)

var ErrImageUnavailable = errors.New("reading: chapter images unavailable; reload the chapter")

func (p *BookSource) imageOwner(item *book.Book, index int, identity string) chapterresource.Owner {
	owner := p.ResourceOwner
	owner.BookID, owner.Revision, owner.SourceID = item.ID, item.ContentRevision, item.SourceID
	owner.ChapterIndex, owner.SourceIdentity = index, identity
	return owner
}

func (p *BookSource) currentDefinition(ctx context.Context, item *book.Book, identity string) error {
	if err := p.current(ctx, item); err != nil {
		return err
	}
	source, err := p.Sources.GetByID(item.SourceID)
	if err != nil {
		return err
	}
	if source == nil {
		return ErrSourceNotFound
	}
	current, err := source.DefinitionIdentity()
	if err != nil {
		return err
	}
	if current != identity {
		return library.ErrStateChanged
	}
	return nil
}

func (p *BookSource) prepareImages(ctx context.Context, item *book.Book, entry book.CachedChapter) (chapterresource.Reference, bool) {
	var urls []string
	for _, block := range entry.Blocks {
		if block.Kind == processor.ProseBlockImage {
			urls = append(urls, block.Src)
		}
	}
	if len(urls) == 0 {
		return chapterresource.Reference{}, false
	}
	if p.Resources == nil || entry.BookContext == nil || entry.ChapterContext == nil {
		return chapterresource.Reference{}, true
	}
	ref, err := p.Resources.Admit(ctx, p.imageOwner(item, entry.ChapterIndex, entry.SourceIdentity), chapterresource.Images{Book: entry.BookContext, Chapter: entry.ChapterContext, URLs: urls}, time.Unix(0, entry.CachedAt).Add(chapterFreshness))
	if err != nil {
		slog.Warn("chapter image preparation unavailable", "book_id", item.ID, "error", err)
		return chapterresource.Reference{}, true
	}
	return ref, false
}

// Image resolves only an immutable bundle, never the current chapter-cache row.
func (p *BookSource) Image(ctx context.Context, id string, revision int64, index, imageIndex int, bundle string) ([]byte, string, error) {
	if p.Resources == nil || bundle == "" || imageIndex < 0 {
		return nil, "", ErrImageUnavailable
	}
	item, chapter, _, err := p.Store.GetChapterSnapshot(ctx, id, index)
	if err != nil {
		return nil, "", err
	}
	if item == nil {
		return nil, "", library.ErrNotFound
	}
	if item.ContentRevision != revision {
		return nil, "", library.ErrStateChanged
	}
	if chapter == nil {
		return nil, "", ErrChapterNotFound
	}
	source, err := p.Sources.GetByID(item.SourceID)
	if err != nil {
		return nil, "", err
	}
	if source == nil {
		return nil, "", ErrSourceNotFound
	}
	identity, err := source.DefinitionIdentity()
	if err != nil {
		return nil, "", err
	}
	images, err := p.Resources.Resolve(ctx, p.imageOwner(item, index, identity), bundle)
	if errors.Is(err, chapterresource.ErrUnavailable) {
		return nil, "", ErrImageUnavailable
	}
	if err != nil {
		return nil, "", err
	}
	if imageIndex >= len(images.URLs) {
		return nil, "", ErrImageUnavailable
	}
	data, mediaType, err := p.Searcher.GetChapterImageForContext(ctx, *source, images.Book, images.Chapter, images.URLs[imageIndex])
	if err != nil {
		return nil, "", err
	}
	if err := p.currentDefinition(ctx, item, identity); err != nil {
		return nil, "", err
	}
	return data, mediaType, nil
}
