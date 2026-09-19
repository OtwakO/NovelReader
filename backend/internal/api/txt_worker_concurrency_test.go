package api

import (
	"context"
	"crypto/rand"
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
	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceprofile"
	"github.com/otwako/novelreader/internal/txtstore"
)

func TestTXTWorkersLeaveForegroundCapacityAndReleaseHomes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const foregroundSlots = 2
		readers, err := readerstore.NewManager(t.TempDir(), foregroundSlots+fileimport.Workers+fileimport.Transfers,
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
		held := make([]heldImport, 0, fileimport.Workers)
		for index := range fileimport.Workers {
			id := readerstore.UserID(fmt.Sprintf("%08x-1111-4111-8111-111111111111", index+1))
			if err := readers.Create(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			home, err := readers.Open(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			defer home.Close()
			receipt, err := txtstore.NewStore(home.DB(), home.Files()).Receive(t.Context(), rand.Text(), "novel.txt", strings.NewReader("Chapter 1\nStart.\nChapter 2\nEnd.\n"))
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
		pool := fileimport.NewPool(readers)
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
		// Active transfers have their own home allowance; queued readers hold none.
		admission := fileimport.NewAdmission()
		defer admission.Close()
		type heldTransfer struct {
			home    *readerstore.Home
			release func()
		}
		transfers := make([]heldTransfer, 0, fileimport.Transfers)
		for index := range fileimport.Transfers {
			id := readerstore.UserID(fmt.Sprintf("%08x-3333-4333-8333-333333333333", index+1))
			if err := readers.Create(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			ticket, err := admission.Request(id)
			if err != nil {
				t.Fatal(err)
			}
			ctx, release, err := admission.Begin(t.Context(), id, ticket.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			home, err := readers.Open(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			defer home.Close()
			transfers = append(transfers, heldTransfer{home: home, release: release})
		}
		// This reader has no home: scheduling cannot create one or borrow a slot.
		waiting, err := admission.Request("cccccccc-3333-4333-8333-333333333333")
		if err != nil || waiting.State != fileimport.TicketWaiting {
			t.Fatalf("unexpected intake admission: %+v %v", waiting, err)
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
		for _, transfer := range transfers {
			if err := transfer.home.Close(); err != nil {
				t.Fatal(err)
			}
			transfer.release()
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			err := readers.Remove(ctx, transfer.home.ID())
			cancel()
			if err != nil {
				t.Fatalf("transfer leaked a home lease: %v", err)
			}
		}
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
