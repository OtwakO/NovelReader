// Chapter-cache helpers expose processed offline copies without hiding their origin.
package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/processor"
)

const proseDocumentVersion = 1

type contentResourceReference struct {
	Href string `json:"href"`
}

type proseDocumentBlock struct {
	Kind     string                    `json:"kind"`
	Text     string                    `json:"text,omitempty"`
	Resource *contentResourceReference `json:"resource,omitempty"`
	Alt      string                    `json:"alt,omitempty"`
}

type proseDocument struct {
	Kind   string               `json:"kind"`
	Title  string               `json:"title"`
	Blocks []proseDocumentBlock `json:"blocks"`
}

type chapterContentResponse struct {
	Version     int           `json:"version"`
	Document    proseDocument `json:"document"`
	OfflineCopy bool          `json:"offlineCopy,omitempty"`
}

func (s *readerAPI) writeChapterCacheFallback(w http.ResponseWriter, storedBook *book.Book, chapter *book.Chapter) bool {
	cached, err := s.bookStore.GetChapterCache(storedBook.ID, storedBook.SourceID, chapter.Index, chapter.URL)
	if err != nil {
		slog.Warn("api: chapter cache lookup failed", "book_id", storedBook.ID, "chapter_index", chapter.Index, "error", err)
		return false
	}
	if cached == nil {
		return false
	}
	writeJSON(w, http.StatusOK, newChapterContentResponse(storedBook.ID, chapter.Index, cached.Title, cached.Paragraphs, cached.Blocks, true))
	return true
}

func (s *readerAPI) saveChapterCache(storedBook *book.Book, chapter *book.Chapter, result processor.ProcessResult) {
	if err := s.bookStore.SaveChapterCache(book.CachedChapter{
		BookID: storedBook.ID, SourceID: storedBook.SourceID, ChapterIndex: chapter.Index,
		ChapterURL: chapter.URL, Title: result.Title, Paragraphs: result.Paragraphs, Blocks: result.Blocks,
	}); err != nil {
		slog.Warn("api: chapter cache save failed", "book_id", storedBook.ID, "chapter_index", chapter.Index, "error", err)
	}
}

func newChapterContentResponse(bookID string, chapterIndex int, title string, paragraphs []string, blocks []processor.ProseBlock, offlineCopy bool) chapterContentResponse {
	if len(blocks) == 0 {
		blocks = make([]processor.ProseBlock, len(paragraphs))
		for index, paragraph := range paragraphs {
			blocks[index] = processor.ProseBlock{Kind: processor.ProseBlockParagraph, Text: paragraph}
		}
	}
	return chapterContentResponse{
		Version: proseDocumentVersion,
		Document: proseDocument{
			Kind:   "prose",
			Title:  title,
			Blocks: responseBlocks(bookID, chapterIndex, blocks),
		},
		OfflineCopy: offlineCopy,
	}
}

func responseBlocks(bookID string, chapterIndex int, blocks []processor.ProseBlock) []proseDocumentBlock {
	response := make([]proseDocumentBlock, 0, len(blocks))
	imageIndex := 0
	for _, block := range blocks {
		kind := block.Kind
		if kind == "text" { // Compatibility with image-bearing caches written before prose documents.
			kind = processor.ProseBlockParagraph
		}
		item := proseDocumentBlock{Kind: kind, Text: block.Text, Alt: block.Alt}
		if kind == processor.ProseBlockImage {
			item.Resource = &contentResourceReference{Href: chapterImageHref(bookID, chapterIndex, imageIndex)}
			imageIndex++
		}
		response = append(response, item)
	}
	return response
}

func chapterImageHref(bookID string, chapterIndex, imageIndex int) string {
	return "/api/books/" + url.PathEscape(bookID) + "/chapters/" + strconv.Itoa(chapterIndex) + "/images/" + strconv.Itoa(imageIndex)
}
