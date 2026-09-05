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
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
	"github.com/otwako/novelreader/internal/webview"
)

const runtimeTestUser readerstore.UserID = "11111111-1111-4111-8111-111111111111"

func TestReaderRuntimeQuiesceDrainsWorkPurgesStateAndRejectsNewAcquisitions(t *testing.T) {
	readers, err := readerstore.NewManager(t.TempDir(), 2, booksource.ReaderSchema(), sourceprofile.ReaderSchema(), fontstore.ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if err := readers.Create(context.Background(), runtimeTestUser); err != nil {
		t.Fatal(err)
	}
	limits := book.DefaultSearcherLimits()
	root := book.NewSearcherWithLimits(fetcher.New(), analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil, limits)
	manager := newReaderRuntimeManager(readers, root, analyzer.NewJSVM(), nil, limits, 2, time.Hour, &readerServices{})
	defer manager.Close()
	runtime, release, err := manager.acquire(context.Background(), runtimeTestUser)
	if err != nil {
		t.Fatal(err)
	}
	runtime.searcher.CapacityStats()
	quiesced := make(chan error, 1)
	go func() { quiesced <- manager.quiesce(context.Background(), runtimeTestUser) }()
	deadline := time.Now().Add(time.Second)
	for {
		_, releaseProbe, err := manager.acquire(context.Background(), runtimeTestUser)
		if errors.Is(err, ErrReaderRuntimeDeleting) {
			break
		}
		if err != nil {
			t.Fatalf("probe acquire: %v", err)
		}
		releaseProbe()
		if time.Now().After(deadline) {
			t.Fatal("quiesce did not reject new acquisitions")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-quiesced:
		t.Fatalf("quiesce completed before request released: %v", err)
	default:
	}
	release()
	if err := <-quiesced; err != nil {
		t.Fatal(err)
	}
	if manager.runtimes[runtimeTestUser] != nil {
		t.Fatal("runtime state remains after quiesce")
	}
	if _, _, err := manager.acquire(context.Background(), runtimeTestUser); !errors.Is(err, ErrReaderRuntimeDeleting) {
		t.Fatalf("post-quiesce acquire error=%v", err)
	}
}

type blockedRuntimeBrowser struct {
	apiBrowserFixture
	started chan struct{}
	resume  chan struct{}
}

func (b *blockedRuntimeBrowser) CloseInteractive(context.Context, string, string, bool, bool, *sourceexec.SourceSession) (webview.InteractiveCloseResult, error) {
	close(b.started)
	<-b.resume
	return webview.InteractiveCloseResult{}, nil
}

func TestRuntimeEvictionKeepsOwnershipWithoutBlockingOtherReaders(t *testing.T) {
	const other readerstore.UserID = "22222222-2222-4222-8222-222222222222"
	const third readerstore.UserID = "33333333-3333-4333-8333-333333333333"
	readers, err := readerstore.NewManager(t.TempDir(), 3, booksource.ReaderSchema(), sourceprofile.ReaderSchema(), fontstore.ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	for _, id := range []readerstore.UserID{runtimeTestUser, other, third} {
		if err := readers.Create(t.Context(), id); err != nil {
			t.Fatal(err)
		}
	}
	browser := &blockedRuntimeBrowser{started: make(chan struct{}), resume: make(chan struct{})}
	limits := book.DefaultSearcherLimits()
	js := analyzer.NewJSVM()
	root := book.NewSearcherWithLimits(fetcher.New(), js, analyzer.NewCacheManager(), nil, nil, limits)
	manager := newReaderRuntimeManager(readers, root, js, browser, limits, 2, time.Hour, &readerServices{})
	defer manager.Close()
	defer close(browser.resume)
	first, releaseFirst, err := manager.acquire(t.Context(), runtimeTestUser)
	if err != nil {
		t.Fatal(err)
	}
	requestID := first.browserSessions.Register(sourceinteraction.BrowserRequest{URL: "https://example.test"})
	if _, err := first.browserSessions.Start(t.Context(), "source", requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err != nil {
		t.Fatal(err)
	}
	_, releaseOther, err := manager.acquire(t.Context(), other)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOther()
	releaseFirst()
	expired := time.Now().Add(2 * time.Hour)
	manager.now = func() time.Time { return expired }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, release, err := manager.acquire(ctx, runtimeTestUser)
		if release != nil {
			release()
		}
		result <- err
	}()
	select {
	case <-browser.started:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("same-reader wait: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("same-reader wait ignored cancellation")
	}
	ctxOther, cancelOther := context.WithTimeout(t.Context(), time.Second)
	defer cancelOther()
	_, releaseProbe, err := manager.acquire(ctxOther, other)
	if err != nil {
		t.Fatalf("unrelated reader blocked by cleanup: %v", err)
	}
	releaseProbe()
	ctxThird, cancelThird := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancelThird()
	_, releaseThird, err := manager.acquire(ctxThird, third)
	if releaseThird != nil {
		releaseThird()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("closing runtime lost its capacity slot: %v", err)
	}
}
