package book

import (
	"context"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
)

// ChapterDocument retains the document-owned inputs needed for later image
// execution. It does not retain a mutable session or historical source scripts.
type ChapterDocument struct {
	Content        string
	Title          string
	RetrievedAt    time.Time
	BookContext    map[string]any
	ChapterContext map[string]any
}

func (s *Searcher) GetChapterDocument(ctx context.Context, src booksource.BookSource, b *Book, current, next *Chapter) (ChapterDocument, error) {
	var result ChapterDocument
	var err error
	result.Content, result.Title, err = s.getChapterContent(ctx, src, b, current, next, &result)
	if err != nil {
		return ChapterDocument{}, err
	}
	result.RetrievedAt = time.Now()
	return result, nil
}
