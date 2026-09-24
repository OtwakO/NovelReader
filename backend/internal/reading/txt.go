package reading

import (
	"context"
	"errors"
	"strings"

	"github.com/otwako/novelreader/internal/txtstore"
)

type txtReader struct{ store *txtstore.Store }

func (p txtReader) catalog(ctx context.Context, id string, _ bool) (Catalog, error) {
	sections, revision, err := p.store.GetCatalog(ctx, id)
	if err != nil {
		return Catalog{}, err
	}
	chapters := make([]Chapter, len(sections))
	for i, section := range sections {
		chapters[i] = Chapter{Index: section.Index, Title: section.Title}
	}
	return Catalog{Chapters: chapters, ContentRevision: revision}, nil
}

func (p txtReader) chapter(ctx context.Context, id string, revision int64, index int) (Chapter, error) {
	section, err := p.store.GetSection(ctx, id, revision, index)
	if errors.Is(err, txtstore.ErrNotFound) {
		return Chapter{}, ErrChapterNotFound
	}
	return Chapter{Index: section.Index, Title: section.Title}, err
}

func (p txtReader) open(ctx context.Context, id string, revision int64, index int) (Content, error) {
	section, err := p.store.ReadSection(ctx, id, revision, index)
	if errors.Is(err, txtstore.ErrNotFound) {
		return Content{}, ErrChapterNotFound
	}
	if err != nil {
		return Content{}, err
	}
	blocks := make([]Block, 0)
	// TXT is literal prose, never HTML. Do not run BookSource extraction or ad
	// removal over an author's original text, including markup-looking lines.
	for _, line := range strings.Split(section.Text, "\n") {
		if strings.TrimSpace(line) != "" {
			blocks = append(blocks, Block{Kind: "paragraph", Text: line})
		}
	}
	return prose(section.ContentRevision, section.Title, blocks), nil
}
