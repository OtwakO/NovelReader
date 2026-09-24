package reading

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/epubstore"
)

type epubReader struct {
	store        *epubstore.Store
	resourceHref func(string, int64, string) string
}

func (p epubReader) catalog(ctx context.Context, id string, _ bool) (Catalog, error) {
	metadata, revision, err := p.store.GetCatalog(ctx, id)
	if err != nil {
		return Catalog{}, err
	}
	return epubCatalog(revision, metadata), nil
}

func (p epubReader) chapter(ctx context.Context, id string, revision int64, index int) (Chapter, error) {
	info, err := p.store.GetSection(ctx, id, revision, index)
	if errors.Is(err, epubstore.ErrNotFound) {
		return Chapter{}, ErrChapterNotFound
	}
	if err != nil {
		return Chapter{}, err
	}
	return Chapter{Index: info.Index, Title: info.Title, Auxiliary: !info.Main}, nil
}

func (p epubReader) open(ctx context.Context, id string, revision int64, index int) (Content, error) {
	content, err := p.store.ReadSection(ctx, id, revision, index)
	if errors.Is(err, epubstore.ErrNotFound) {
		return Content{}, ErrChapterNotFound
	}
	if err != nil {
		return Content{}, err
	}
	return epubContent(ctx, revision, content.Title, content.Section, func(key string) string { return p.resourceHref(id, revision, content.Resources[key]) })
}
