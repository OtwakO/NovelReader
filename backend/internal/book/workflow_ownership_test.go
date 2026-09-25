package book

import (
	"context"
	"errors"
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

func TestChapterPublicationAndImagesShareWorkflowOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, finish := make(chan struct{}), make(chan struct{})
		searcher := NewSearcher(nil, nil, analyzer.NewCacheManager(), nil, nil)
		searcher.SetTransportFactory(func(_ *fetcher.Client, _ *sourceexec.SourceSession) sourceexec.Transport {
			return workflowTransport(func(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
				return sourceexec.Response{StatusCode: 200, Body: "<body>Readable text</body>", FinalURL: spec.URL}, nil
			})
		})
		source := booksource.BookSource{ID: "source", BookSourceURL: "https://example.invalid"}
		item := &Book{BookURL: "https://example.invalid/book"}
		first := &Chapter{URL: "https://example.invalid/first"}
		firstDone := make(chan error, 1)
		go func() {
			err := searcher.WithChapterWorkflow(t.Context(), source, item, first, nil, func(ctx context.Context, retrieve func() (ChapterDocument, error)) error {
				if _, err := retrieve(); err != nil {
					return err
				}
				// Upstream has finished, but document processing/publication still owns the session.
				close(started)
				select {
				case <-finish:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			firstDone <- err
		}()
		<-started
		imageDone := make(chan error, 1)
		go func() {
			_, _, err := searcher.GetChapterImageForContext(t.Context(), source, bookContext(item, source), chapterContext(item, first, first.URL), "data:image/png;base64,aW1hZ2U=")
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

// Exercise the actual reGetBook path, not a direct registry reassignment.
func TestReGetBookDoesNotOverlapExistingDestinationWorkflow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started, finish := make(chan struct{}), make(chan struct{})
		overlapped := make(chan struct{}, 1)
		searcher := NewSearcher(nil, analyzer.NewJSVMWithPoolSize(1), analyzer.NewCacheManager(), nil, nil)
		var existing *sourceexec.SourceSession
		searcher.SetTransportFactory(func(_ *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport {
			return workflowTransport(func(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
				body := ""
				switch spec.URL {
				case "https://example.invalid/new-book":
					if existing == nil {
						existing = session
						close(started)
						<-finish
					} else if session != existing {
						select {
						case <-finish:
						default:
							overlapped <- struct{}{}
						}
					}
					body = `<a class="toc" href="/toc">Contents</a>`
				case "https://example.invalid/search":
					body = `<div class="book"><span class="name">Novel</span><span class="author">Writer</span><a href="/new-book">Details</a></div>`
				case "https://example.invalid/toc":
					body = `<a class="chapter" href="/chapter">Chapter</a>`
				default:
					t.Errorf("unexpected request: %s", spec.URL)
				}
				return sourceexec.Response{StatusCode: 200, Body: body, FinalURL: spec.URL}, nil
			})
		})
		source := booksource.BookSource{ID: "source", BookSourceURL: "https://example.invalid",
			SearchURL:    "https://example.invalid/search",
			RuleSearch:   `{"bookList":"@css:.book","name":"@css:.name@text","author":"@css:.author@text","bookUrl":"@css:a@href"}`,
			RuleBookInfo: `{"tocUrl":"@css:.toc@href"}`,
			RuleToc:      `{"preUpdateJs":"java.reGetBook()","chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href"}`,
		}
		detailDone := make(chan error, 1)
		go func() {
			_, err := searcher.GetBookInfoForBookContext(t.Context(), source, &Book{Name: "Novel", Author: "Writer", BookURL: "https://example.invalid/new-book"}, "https://example.invalid/new-book")
			detailDone <- err
		}()
		<-started
		tocDone := make(chan error, 1)
		go func() {
			_, err := searcher.GetChapterListForBookContext(t.Context(), source, &Book{Name: "Novel", Author: "Writer", BookURL: "https://example.invalid/old-book"}, "")
			tocDone <- err
		}()
		synctest.Wait()
		close(finish)
		if err := <-detailDone; err != nil {
			t.Fatal(err)
		}
		if err := <-tocDone; !errors.Is(err, sourceexec.ErrSessionAliasConflict) {
			t.Fatalf("expected explicit ownership conflict, got %v", err)
		}
		select {
		case <-overlapped:
			t.Fatal("reGetBook entered the destination with a different session while its detail workflow was still active")
		default:
		}
	})
}
