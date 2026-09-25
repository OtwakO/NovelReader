package reading

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

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
	identity, err := src.DefinitionIdentity()
	if err != nil {
		return Content{}, err
	}
	document, err := p.Searcher.GetChapterDocument(ctx, *src, item, ch, next)
	raw, title := document.Content, document.Title
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
	if err := p.currentDefinition(ctx, item, identity); err != nil {
		return Content{}, err
	}
	entry := book.CachedChapter{
		SourceIdentity: identity, BookContext: document.BookContext, ChapterContext: document.ChapterContext, CachedAt: document.RetrievedAt.UnixNano(),
		ContentRevision: revision, BookID: id, SourceID: item.SourceID, ChapterIndex: index,
		ChapterURL: ch.URL, Title: result.Title, Paragraphs: result.Paragraphs, Blocks: result.Blocks,
	}
	if err := p.Store.SaveChapterCache(entry); err != nil {
		slog.Warn("reading: chapter cache save failed", "book_id", id, "chapter_index", index, "error", err)
	}
	return p.content(ctx, item, entry, false)
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
	if cached.SourceIdentity == "" || time.Since(time.Unix(0, cached.CachedAt)) >= chapterFreshness {
		return Content{}, cause
	}
	if err := p.currentDefinition(ctx, item, cached.SourceIdentity); err != nil {
		return Content{}, cause
	}
	return p.content(ctx, item, *cached, true)
}

const chapterFreshness = 24 * time.Hour

func (p *BookSource) content(ctx context.Context, item *book.Book, entry book.CachedChapter, offline bool) (Content, error) {
	reference, unavailable := p.prepareImages(ctx, item, entry)
	if err := p.currentDefinition(ctx, item, entry.SourceIdentity); err != nil {
		return Content{}, err
	}
	return p.document(entry, reference.ID, unavailable, offline)
}

func (p *BookSource) document(entry book.CachedChapter, bundleID string, unavailable, offline bool) (Content, error) {
	paragraphs, blocks := entry.Paragraphs, entry.Blocks
	hasText := false
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
		if item.Kind == processor.ProseBlockParagraph && strings.TrimSpace(block.Text) != "" {
			hasText = true
		}
		if block.Kind == processor.ProseBlockImage {
			item.Resource = &ResourceReference{Unavailable: unavailable}
			if !unavailable {
				item.Resource.Href = p.ImageHref(entry.BookID, entry.ContentRevision, entry.ChapterIndex, imageIndex, bundleID)
			}
			imageIndex++
		}
		output = append(output, item)
	}
	if unavailable && !hasText {
		return Content{}, ErrImageUnavailable
	}
	content := prose(entry.ContentRevision, entry.Title, output)
	remaining := max(int64(0), time.Until(time.Unix(0, entry.CachedAt).Add(chapterFreshness)).Milliseconds())
	if unavailable {
		remaining = 0
	}
	content.FreshForMS = &remaining
	content.OfflineCopy = offline
	return content, nil
}
