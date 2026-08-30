package book

import (
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestDeleteSourceSessionInvalidatesExecutionAndExploreState(t *testing.T) {
	searcher := NewSearcherWithLimits(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil, SearcherLimits{MaxSessions: 4, SessionTTL: time.Minute})
	source := booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test"}
	other := booksource.BookSource{ID: "source-b", BookSourceURL: "https://other.test"}
	bookSession := searcher.sessions.GetOrCreateBook(source.ID, "book-a")
	exploreID, _, err := searcher.explore.create(source, []exploreKind{{Type: exploreKindURL, Title: "Books", URL: "/books"}}, sourceexec.NewSourceSession())
	if err != nil {
		t.Fatal(err)
	}
	otherExploreID, _, err := searcher.explore.create(other, []exploreKind{{Type: exploreKindURL, Title: "Books", URL: "/books"}}, sourceexec.NewSourceSession())
	if err != nil {
		t.Fatal(err)
	}

	searcher.DeleteSourceSession(source.ID)

	if replacement := searcher.sessions.GetOrCreateBook(source.ID, "book-a"); replacement == bookSession {
		t.Fatal("book execution session was not invalidated")
	}
	if session, release := searcher.explore.acquire(exploreID); session != nil {
		release()
		t.Fatal("Explore session was not invalidated")
	}
	if session, release := searcher.explore.acquire(otherExploreID); session == nil {
		release()
		t.Fatal("unrelated Explore session was invalidated")
	} else {
		release()
	}
}
