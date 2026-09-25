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

// WithChapterWorkflow keeps one session lease through the caller's cache recheck,
// processing and publication. The supplied retrieval function is scoped to work;
// callers must not retain it or invoke it concurrently.
func (s *Searcher) WithChapterWorkflow(ctx context.Context, src booksource.BookSource, b *Book, current, next *Chapter, work func(context.Context, func() (ChapterDocument, error)) error) error {
	ctx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
	defer cancel()
	bookURL, chapterURL := "", ""
	if b != nil {
		bookURL = b.BookURL
	}
	if current != nil {
		chapterURL = current.URL
	}
	session, release, err := s.sessions.AcquireWorkflow(ctx, src.ID, bookURL, chapterURL)
	if err != nil {
		return err
	}
	defer release()
	return work(ctx, func() (ChapterDocument, error) {
		var result ChapterDocument
		var err error
		result.Content, result.Title, err = s.getChapterContent(ctx, src, b, current, next, &result, session)
		if err != nil {
			return ChapterDocument{}, err
		}
		result.RetrievedAt = time.Now()
		return result, nil
	})
}

func (s *Searcher) GetChapterDocument(ctx context.Context, src booksource.BookSource, b *Book, current, next *Chapter) (ChapterDocument, error) {
	var result ChapterDocument
	err := s.WithChapterWorkflow(ctx, src, b, current, next, func(_ context.Context, retrieve func() (ChapterDocument, error)) error {
		var err error
		result, err = retrieve()
		return err
	})
	return result, err
}
