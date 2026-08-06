// Chapter-cache helpers expose processed offline copies without hiding their origin.
package api

import (
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/processor"
)

type chapterContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Index *int   `json:"index,omitempty"`
}

type chapterContentResponse struct {
	Title       string                `json:"title"`
	Paragraphs  []string              `json:"paragraphs"`
	Blocks      []chapterContentBlock `json:"blocks,omitempty"`
	OfflineCopy bool                  `json:"offlineCopy,omitempty"`
}

func (s *Server) writeChapterCacheFallback(w http.ResponseWriter, storedBook *book.Book, chapter *book.Chapter) bool {
	cached, err := s.bookStore.GetChapterCache(storedBook.ID, storedBook.SourceURL, chapter.Index, chapter.URL)
	if err != nil {
		slog.Warn("api: chapter cache lookup failed", "book_id", storedBook.ID, "chapter_index", chapter.Index, "error", err)
		return false
	}
	if cached == nil {
		return false
	}
	writeJSON(w, http.StatusOK, chapterContentResponse{Title: cached.Title, Paragraphs: cached.Paragraphs, Blocks: responseBlocks(cached.Blocks), OfflineCopy: true})
	return true
}

func (s *Server) saveChapterCache(storedBook *book.Book, chapter *book.Chapter, result processor.ProcessResult) {
	if err := s.bookStore.SaveChapterCache(book.CachedChapter{
		BookID: storedBook.ID, SourceURL: storedBook.SourceURL, ChapterIndex: chapter.Index,
		ChapterURL: chapter.URL, Title: result.Title, Paragraphs: result.Paragraphs, Blocks: result.Blocks,
	}); err != nil {
		slog.Warn("api: chapter cache save failed", "book_id", storedBook.ID, "chapter_index", chapter.Index, "error", err)
	}
}

func responseBlocks(blocks []processor.ContentBlock) []chapterContentBlock {
	if len(blocks) == 0 {
		return nil
	}
	response := make([]chapterContentBlock, 0, len(blocks))
	imageIndex := 0
	for _, block := range blocks {
		item := chapterContentBlock{Type: block.Type, Text: block.Text}
		if block.Type == "image" {
			index := imageIndex
			item.Index = &index
			imageIndex++
		}
		response = append(response, item)
	}
	return response
}
