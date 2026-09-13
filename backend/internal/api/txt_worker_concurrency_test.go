package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceprofile"
	"github.com/otwako/novelreader/internal/txtimport"
	"github.com/otwako/novelreader/internal/txtstore"
)

func TestTXTWorkersLeaveForegroundCapacityAndReleaseHomes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const foregroundSlots = 2
		readers, err := readerstore.NewManager(t.TempDir(), foregroundSlots+txtimport.Workers,
			library.ReaderSchema(), booksource.ReaderSchema(), book.ReaderSchema(), fontstore.ReaderSchema(), sourceprofile.ReaderSchema(), txtstore.ReaderSchema())
		if err != nil {
			t.Fatal(err)
		}
		defer readers.Close()
		type heldImport struct {
			home    *readerstore.Home
			conn    *sql.Conn
			receipt txtstore.Receipt
			id      readerstore.UserID
		}
		held := make([]heldImport, 0, txtimport.Workers)
		for index := range txtimport.Workers {
			id := readerstore.UserID(fmt.Sprintf("%08x-1111-4111-8111-111111111111", index+1))
			if err := readers.Create(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			home, err := readers.Open(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			defer home.Close()
			receipt, err := txtstore.NewStore(home.DB(), home.Files()).Receive(t.Context(), "novel.txt", strings.NewReader("Chapter 1\nStart.\nChapter 2\nEnd.\n"))
			if err != nil {
				t.Fatal(err)
			}
			// Hold the sole connection so a real worker waits inside TXT storage, after
			// acquiring its own home lease. No sleeps, network, or parser timing assumptions.
			home.DB().SetMaxOpenConns(1)
			conn, err := home.DB().Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			held = append(held, heldImport{home: home, conn: conn, receipt: receipt, id: id})
		}
		pool := txtimport.NewPool(readers)
		defer pool.Close()
		for _, current := range held {
			if err := pool.Notify(current.id); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		for _, current := range held {
			if current.home.DB().Stats().WaitCount != 1 {
				t.Fatal("worker did not reach the storage boundary")
			}
		}
		limits := book.DefaultSearcherLimits()
		js := analyzer.NewJSVM()
		searcher := book.NewSearcherWithLimits(fetcher.New(), js, analyzer.NewCacheManager(), nil, nil, limits)
		runtimes := newReaderRuntimeManager(readers, searcher, js, nil, limits, foregroundSlots, time.Hour, &readerServices{})
		defer runtimes.Close()
		// Keep one foreground reader active while 100 other identities cycle through
		// the remaining runtime slot. This is a capacity/lifetime regression, not a
		// throughput benchmark or a claim of 100 simultaneous expensive requests.
		pinnedID := readerstore.UserID("aaaaaaaa-1111-4111-8111-111111111111")
		if err := readers.Create(t.Context(), pinnedID); err != nil {
			t.Fatal(err)
		}
		pinned, releasePinned, err := runtimes.acquire(t.Context(), pinnedID)
		if err != nil {
			t.Fatal(err)
		}
		defer releasePinned()
		for index := range 100 {
			id := readerstore.UserID(fmt.Sprintf("%08x-2222-4222-8222-222222222222", index+1))
			if err := readers.Create(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			runtime, release, err := runtimes.acquire(t.Context(), id)
			if err != nil {
				t.Fatalf("foreground reader %d blocked by imports: %v", index, err)
			}
			response := httptest.NewRecorder()
			runtime.api.handleListBooks(response, httptest.NewRequest(http.MethodGet, "/api/books", nil))
			release()
			if response.Code != http.StatusOK {
				t.Fatalf("foreground read=%d %s", response.Code, response.Body.String())
			}
		}
		response := httptest.NewRecorder()
		pinned.api.handleListBooks(response, httptest.NewRequest(http.MethodGet, "/api/books", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("active reader interrupted: %s", response.Body.String())
		}
		for _, current := range held {
			if err := current.conn.Close(); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		pool.Close()
		for _, current := range held {
			stored, err := txtstore.NewStore(current.home.DB(), current.home.Files()).Get(t.Context(), current.receipt.ID)
			if err != nil || stored.State != txtstore.Ready {
				t.Fatalf("analysis did not finish: %+v %v", stored, err)
			}
			if err := current.home.Close(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			err = readers.Remove(ctx, current.id)
			cancel()
			if err != nil {
				t.Fatalf("worker leaked a home lease: %v", err)
			}
		}
	})
}
