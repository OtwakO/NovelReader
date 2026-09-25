package book

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type workflowTransport func(context.Context, sourceexec.RequestSpec) (sourceexec.Response, error)

func (f workflowTransport) Do(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	return f(ctx, spec)
}

func TestContentAndImagesShareWorkflowOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, finish := make(chan struct{}), make(chan struct{})
		searcher := NewSearcher(nil, nil, analyzer.NewCacheManager(), nil, nil)
		searcher.SetTransportFactory(func(_ *fetcher.Client, _ *sourceexec.SourceSession) sourceexec.Transport {
			return workflowTransport(func(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
				if spec.URL == "https://example.invalid/first" {
					close(started)
					select {
					case <-finish:
					case <-ctx.Done():
						return sourceexec.Response{}, ctx.Err()
					}
				}
				return sourceexec.Response{StatusCode: 200, Body: "<body>Readable text</body>", FinalURL: spec.URL}, nil
			})
		})
		source := booksource.BookSource{ID: "source", BookSourceURL: "https://example.invalid"}
		item := &Book{BookURL: "https://example.invalid/book"}
		first := &Chapter{URL: "https://example.invalid/first"}
		firstDone := make(chan error, 1)
		go func() {
			_, _, err := searcher.GetChapterContentForBookContext(t.Context(), source, item, first, nil)
			firstDone <- err
		}()
		<-started
		imageDone := make(chan error, 1)
		go func() {
			_, _, err := searcher.GetChapterImage(t.Context(), source, item, first, "data:image/png;base64,aW1hZ2U=")
			imageDone <- err
		}()
		synctest.Wait()
		select {
		case err := <-imageDone:
			t.Fatalf("image entered active chapter workflow: %v", err)
		default:
		}
		// A different book does not wait for this book's upstream request.
		if _, _, err := searcher.GetChapterContentForBookContext(t.Context(), source, &Book{BookURL: "https://example.invalid/other"}, &Chapter{URL: "https://example.invalid/other-chapter"}, nil); err != nil {
			t.Fatal(err)
		}
		close(finish)
		if err := <-firstDone; err != nil {
			t.Fatal(err)
		}
		if err := <-imageDone; err != nil {
			t.Fatal(err)
		}
	})
}
