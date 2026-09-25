package book

import (
	"bytes"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchLogsExcludeSourcePayloads(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer upstream.Close()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)
	searcher := NewSearcher(fetcher.New(), nil, nil, nil, nil)
	src := booksource.BookSource{ID: "synthetic", BookSourceURL: upstream.URL, BookSourceName: "source-name-secret", SearchURL: upstream.URL + "/?token=review-secret", RuleSearch: `{"bookList":"a"}`}
	var sourceError error
	_ = searcher.searchSources(t.Context(), "query-secret", []booksource.BookSource{src}, 1, func(_ booksource.BookSource, _ []SearchResult, err error) { sourceError = err })
	if sourceError == nil {
		t.Fatal("expected synthetic failed request")
	}
	for _, secret := range []string{"review-secret", "source-name-secret", "query-secret"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("logs contain synthetic sensitive input %q", secret)
		}
	}
	if !strings.Contains(logs.String(), "source failed") || !strings.Contains(logs.String(), "synthetic") {
		t.Fatal("missing safe source failure diagnostic")
	}
}
