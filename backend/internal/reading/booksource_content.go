package reading

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/processor"
)

func (p *BookSource) open(ctx context.Context, id string, revision int64, index int) (Content, error) {
	item, ch, next, err := p.Store.GetChapterSnapshot(ctx, id, index)
	if err != nil {
		return Content{}, err
	}
	if item == nil {
		return Content{}, library.ErrNotFound
	}
	if item.ContentRevision != revision {
		return Content{}, library.ErrStateChanged
	}
	if ch == nil {
		return Content{}, ErrChapterNotFound
	}
	src, err := p.Sources.GetByID(item.SourceID)
	if err != nil {
		return p.fallback(ctx, item, ch, err)
	}
	if src == nil {
		return p.fallback(ctx, item, ch, ErrSourceNotFound)
	}
	raw, title, err := p.Searcher.GetChapterContentForBookContext(ctx, *src, item, ch, next)
	if err == nil && strings.TrimSpace(raw) == "" {
		err = errors.New("content: empty extraction")
	}
	if err != nil {
		return p.fallback(ctx, item, ch, &CrawlError{Stage: "content", Err: err})
	}
	if title == "" {
		title = ch.Title
	}
	result := processor.New(p.ProcessorConfig).Process(title, raw)
	if err := p.current(ctx, item); err != nil {
		return Content{}, err
	}
	if err := p.Store.SaveChapterCache(book.CachedChapter{
		ContentRevision: revision, BookID: id, SourceID: item.SourceID, ChapterIndex: index,
		ChapterURL: ch.URL, Title: result.Title, Paragraphs: result.Paragraphs, Blocks: result.Blocks,
	}); err != nil {
		slog.Warn("reading: chapter cache save failed", "book_id", id, "chapter_index", index, "error", err)
	}
	return p.content(id, revision, index, result.Title, result.Paragraphs, result.Blocks, false), nil
}

func (p *BookSource) current(ctx context.Context, snapshot *book.Book) error {
	current, err := p.Store.IsChapterSnapshotCurrent(ctx, snapshot)
	if err != nil {
		return err
	}
	if !current {
		return library.ErrStateChanged
	}
	return nil
}

func (p *BookSource) fallback(ctx context.Context, item *book.Book, chapter *book.Chapter, cause error) (Content, error) {
	if err := p.current(ctx, item); err != nil {
		return Content{}, err
	}
	cached, err := p.Store.GetChapterCache(item.ID, item.SourceID, chapter.Index, chapter.URL, item.ContentRevision)
	if err != nil {
		slog.Warn("reading: chapter cache lookup failed", "book_id", item.ID, "chapter_index", chapter.Index, "error", err)
		return Content{}, cause
	}
	if cached == nil {
		return Content{}, cause
	}
	return p.content(item.ID, item.ContentRevision, chapter.Index, cached.Title, cached.Paragraphs, cached.Blocks, true), nil
}

func (p *BookSource) content(id string, revision int64, index int, title string, paragraphs []string, blocks []processor.ProseBlock, offline bool) Content {
	if len(blocks) == 0 {
		blocks = make([]processor.ProseBlock, len(paragraphs))
		for i, paragraph := range paragraphs {
			blocks[i] = processor.ProseBlock{Kind: processor.ProseBlockParagraph, Text: paragraph}
		}
	}
	output := make([]Block, 0, len(blocks))
	imageIndex := 0
	for _, block := range blocks {
		item := Block{Kind: block.Kind, Text: block.Text, Alt: block.Alt}
		// Preserve existing processed-cache normalization.
		if item.Kind == "text" {
			item.Kind = processor.ProseBlockParagraph
		}
		if block.Kind == processor.ProseBlockImage {
			item.Resource = &ResourceReference{Href: p.ImageHref(id, revision, index, imageIndex)}
			imageIndex++
		}
		output = append(output, item)
	}
	content := prose(revision, title, output)
	content.OfflineCopy = offline
	return content
}
