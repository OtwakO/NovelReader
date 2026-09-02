package book

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func TestUnmodifiedAggregateExploreUsesHydratedSourceSettings(t *testing.T) {
	selectedSource := "番茄"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/discovestyle" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("source"); got != selectedSource {
			http.Error(w, fmt.Sprintf("source=%q want=%q", got, selectedSource), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":[{"title":%q,"url":%q}]}`, selectedSource, server.URL+"/books?page={{page}}")
	}))
	defer server.Close()

	raw, err := os.ReadFile("../../../test-booksources/test_光遇聚合_aggregated_booksource.json")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("private aggregate BookSource fixture is not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	source := sources[0]
	source.ID = "aggregate-fixture"

	client := fetcher.NewWithTimeout(3 * time.Second)
	vm := analyzer.NewJSVM()
	vm.SetFetcher(client)
	searcher := NewSearcher(client, vm, analyzer.NewCacheManager(), exploreSourceFixtureStore{source: source}, nil)
	searcher.SetSourceSessionHydrator(func(_ context.Context, current booksource.BookSource, session *sourceexec.SourceSession) error {
		settings := sourceprofile.Settings{Variable: aggregateExploreSettings(t, server.URL, selectedSource)}
		sourceprofile.ApplySettings(session, current.BookSourceURL, settings)
		return nil
	})

	first, err := searcher.OpenExplore(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !catalogHasTitle(first, "番茄") {
		t.Fatalf("first catalog did not reflect hydrated source: %+v", first.Entries)
	}

	selectedSource = "七猫"
	searcher.DeleteSourceSession(source.ID)
	second, err := searcher.OpenExplore(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !catalogHasTitle(second, "七猫") {
		t.Fatalf("refreshed catalog did not reflect updated source: %+v", second.Entries)
	}
}

func aggregateExploreSettings(t *testing.T, gateway, selectedSource string) string {
	t.Helper()
	value, err := json.Marshal(map[string]interface{}{
		"线路":    gateway,
		"发现页来源": selectedSource,
		"发现页类型": "小说",
		"云端配置": map[string]interface{}{
			"hosts": []string{gateway},
			"小说":    []string{"番茄", "七猫"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(value)
}

func catalogHasTitle(catalog ExploreCatalog, title string) bool {
	for _, entry := range catalog.Entries {
		if entry.Title == title {
			return true
		}
	}
	return false
}
