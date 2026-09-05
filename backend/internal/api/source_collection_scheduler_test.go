package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/webview"
)

func TestScheduledCollectionStorageHonorsCancellation(t *testing.T) {
	for _, operation := range []string{"replacement", "success record"} {
		t.Run(operation, func(t *testing.T) {
			store, profiles, db, _, cleanup := newSourceLifecycleStores(t)
			defer cleanup()
			collection, _, err := store.CreateCollection(t.Context(), booksource.CreateCollection{Name: "Fixture", OriginKind: booksource.CollectionOriginUpload, SyncInterval: booksource.SyncManual}, nil, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			connection, err := db.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			done := make(chan error, 1)
			go func() {
				if operation == "success record" {
					done <- store.RecordCollectionSuccess(ctx, collection.ID, time.Now())
					return
				}
				_, err := replaceSourceCollection(ctx, store, profiles, nil, collection.ID, nil, "", "", "", time.Now())
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled operation: %v", err)
				}
			case <-time.After(time.Second):
				connection.Close()
				<-done
				t.Fatal("storage operation ignored cancellation while waiting for a connection")
			}
		})
	}
}

type collectionBrowser struct {
	apiBrowserFixture
	closed int
}

func (b *collectionBrowser) CloseInteractive(context.Context, string, string, bool, bool, *sourceexec.SourceSession) (webview.InteractiveCloseResult, error) {
	b.closed++
	return webview.InteractiveCloseResult{}, nil
}

func TestScheduledReplacementInvalidatesInteractiveBrowser(t *testing.T) {
	store, profiles, _, _, cleanup := newSourceLifecycleStores(t)
	defer cleanup()
	collection, _, err := store.CreateCollection(t.Context(), booksource.CreateCollection{Name: "Fixture", OriginKind: booksource.CollectionOriginURL, OriginURL: "https://collection.test", SyncInterval: booksource.SyncDaily}, []*booksource.BookSource{{BookSourceURL: "https://example.test", BookSourceName: "Before", Enabled: true}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sources, err := store.ListByCollection(t.Context(), collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	browser := &collectionBrowser{}
	sessions := sourceinteraction.NewBrowserSessions(browser)
	requestID := sessions.Register(sourceinteraction.BrowserRequest{URL: "https://example.test/login"})
	if _, err := sessions.Start(t.Context(), sources[0].ID, requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err != nil {
		t.Fatal(err)
	}
	runtime := &readerRuntime{sourceStore: store, sourceProfiles: profiles, browserSessions: sessions, searcher: book.NewSearcher(fetcher.New(), analyzer.NewJSVM(), analyzer.NewCacheManager(), store, nil)}
	scheduler := &sourceCollectionScheduler{ctx: t.Context(), load: func(context.Context, string, string, string) (booksource.RemoteDocument, error) {
		return booksource.RemoteDocument{Body: []byte(`[{"bookSourceUrl":"https://example.test","bookSourceName":"After","enabled":true}]`)}, nil
	}}
	scheduler.syncCollection(t.Context(), runtime, collection)
	if browser.closed != 1 {
		t.Fatalf("browser closes=%d", browser.closed)
	}
	updated, err := store.GetByID(sources[0].ID)
	if err != nil || updated.BookSourceName != "After" {
		t.Fatalf("updated=%+v error=%v", updated, err)
	}
}
