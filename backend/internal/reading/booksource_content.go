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
	return p.openChapter(ctx, id, revision, index, false)
}

func (p *BookSource) openChapter(ctx context.Context, id string, revision int64, index int, refresh bool) (Content, error) {
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
		return Content{}, err
	}
	if src == nil {
		return Content{}, ErrSourceNotFound
	}
	identity, err := src.DefinitionIdentity()
	if err != nil {
		return Content{}, err
	}
	if !refresh {
		if cached := p.freshChapter(item, ch, identity); cached != nil {
			return p.content(ctx, item, *cached)
		}
	}
	key := chapterKey{bookID: id, sourceID: item.SourceID, definition: identity, bookURL: item.BookURL, chapterURL: ch.URL, revision: revision, index: index, refresh: refresh}
	content, err := p.chapters.do(ctx, key, func(workCtx context.Context) (chapterResult, error) {
		var result chapterResult
		err := p.Searcher.WithChapterWorkflow(workCtx, *src, item, ch, next, func(workCtx context.Context, retrieve func() (book.ChapterDocument, error)) error {
			if err := p.currentDefinition(workCtx, item, identity); err != nil {
				return err
			}
			// A preceding workflow may have filled the cache while we waited for its session.
			if !refresh {
				if cached := p.freshChapter(item, ch, identity); cached != nil {
					var err error
					result.content, err = p.content(workCtx, item, *cached)
					result.freshUntil = time.Unix(0, cached.CachedAt).Add(chapterFreshness)
					return err
				}
			}
			document, err := retrieve()
			if err == nil && strings.TrimSpace(document.Content) == "" {
				err = errors.New("content: empty extraction")
			}
			if err != nil {
				return &CrawlError{Stage: "content", Err: err}
			}
			title := document.Title
			if title == "" {
				title = ch.Title
			}
			processed := processor.New(p.ProcessorConfig).Process(title, document.Content)
			entry := book.CachedChapter{
				SourceIdentity: identity, BookContext: document.BookContext, ChapterContext: document.ChapterContext, CachedAt: document.RetrievedAt.UnixNano(),
				ContentRevision: revision, BookID: id, SourceID: item.SourceID, ChapterIndex: index,
				ChapterURL: ch.URL, Title: processed.Title, Paragraphs: processed.Paragraphs, Blocks: processed.Blocks,
			}
			result.content, err = p.content(workCtx, item, entry)
			result.freshUntil = document.RetrievedAt.Add(chapterFreshness)
			if err != nil {
				return err
			}
			if err := p.Store.SaveChapterCache(entry); err != nil {
				slog.Warn("reading: chapter cache save failed", "book_id", id, "chapter_index", index, "error", err)
			}
			return nil
		})
		return result, err
	})
	if err != nil {
		return Content{}, err
	}
	if err := p.currentDefinition(ctx, item, identity); err != nil {
		return Content{}, err
	}
	return content, nil
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

// Only qualified, unexpired copies can satisfy a load. Access never renews age.
func (p *BookSource) freshChapter(item *book.Book, chapter *book.Chapter, identity string) *book.CachedChapter {
	cached, err := p.Store.GetChapterCache(item.ID, item.SourceID, chapter.Index, chapter.URL, item.ContentRevision)
	if err != nil {
		slog.Warn("reading: chapter cache lookup failed", "book_id", item.ID, "chapter_index", chapter.Index, "error", err)
		return nil
	}
	if cached == nil || cached.SourceIdentity != identity || time.Since(time.Unix(0, cached.CachedAt)) >= chapterFreshness {
		return nil
	}
	return cached
}

const chapterFreshness = 24 * time.Hour

func (p *BookSource) content(ctx context.Context, item *book.Book, entry book.CachedChapter) (Content, error) {
	reference, unavailable := p.prepareImages(ctx, item, entry)
	if err := p.currentDefinition(ctx, item, entry.SourceIdentity); err != nil {
		return Content{}, err
	}
	return p.document(entry, reference.ID, unavailable)
}

func (p *BookSource) document(entry book.CachedChapter, bundleID string, unavailable bool) (Content, error) {
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
	return content, nil
}
