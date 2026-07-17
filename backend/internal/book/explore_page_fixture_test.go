// Raw Explore page fixtures pin request and rule behavior across parser modes.
package book

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type exploreResponseTransport struct {
	body string
	spec *sourceexec.RequestSpec
}

func (t exploreResponseTransport) Do(_ context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	*t.spec = spec
	return sourceexec.Response{StatusCode: 200, Body: t.body, FinalURL: spec.URL, Transport: "fixture"}, nil
}

func TestPinnedExploreNormalHTMLPage(t *testing.T) {
	source := pinnedExploreSource(t, 7)
	page, spec := runPinnedExplorePage(t, source, "entry-0", "explore-html.html")
	if spec.Method != "GET" || spec.URL != "http://www.shukuge.com/i-xuanhuan/1" {
		t.Fatalf("request=%+v", spec)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "HTML Book" || page.Books[0].Author != "HTML Author" || page.Books[0].Kind != "Fantasy" {
		t.Fatalf("page=%+v", page)
	}
}

func TestPinnedExploreAnglePageSelectorAdvances(t *testing.T) {
	source := pinnedExploreSource(t, 12)
	var specs []sourceexec.RequestSpec
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	searcher.SetTransportFactory(func(*fetcher.Client, *sourceexec.SourceSession) sourceexec.Transport {
		return recordingExploreTransport{body: exploreResponseFixture(t, "explore-page-selector.html"), specs: &specs}
	})
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	for page := 1; page <= 2; page++ {
		if _, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: page}); err != nil {
			t.Fatal(err)
		}
	}
	if len(specs) != 2 || specs[0].URL != "https://www.sudugu.org/xuanhuan/" || specs[1].URL != "https://www.sudugu.org/xuanhuan/2.html" {
		t.Fatalf("requests=%+v", specs)
	}
}

type recordingExploreTransport struct {
	body  string
	specs *[]sourceexec.RequestSpec
}

func (t recordingExploreTransport) Do(_ context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	*t.specs = append(*t.specs, spec)
	return sourceexec.Response{StatusCode: 200, Body: t.body, FinalURL: spec.URL}, nil
}

func TestPinnedExploreXPathPage(t *testing.T) {
	source := pinnedExploreSource(t, 1)
	page, spec := runPinnedExplorePage(t, source, "entry-1", "explore-xpath.html")
	if spec.Method != "GET" || spec.URL != "https://bcshuku.com/booklist1/0.html" {
		t.Fatalf("request=%+v", spec)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "XPath Book" || page.Books[0].Author != "XPath Author" || page.Books[0].BookURL != "https://bcshuku.com/book/xpath-1" {
		t.Fatalf("page=%+v", page)
	}
}

func TestPinnedExploreJSONPageExecutesFieldTemplatesAndJavaScript(t *testing.T) {
	source := pinnedExploreSource(t, 3)
	source.ExploreURL = "Books::https://fixture.test/json?page={{page}}"
	page, spec := runPinnedExplorePage(t, source, "entry-0", "explore-json.json")
	if spec.Method != "GET" || spec.URL != "https://fixture.test/json?page=1" {
		t.Fatalf("request=%+v", spec)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "JSON Book" || page.Books[0].Author != "JSON Author" || page.Books[0].BookURL != `data:;base64,MTIz,{"type":"X-QD"}` || page.Books[0].CoverURL != "https://qidian.qpic.cn/qdbimg/349573/123/600" {
		t.Fatalf("page=%+v", page)
	}
}

func TestPinnedExploreWebViewDetailOptionIsPreserved(t *testing.T) {
	source := pinnedExploreSource(t, 160)
	page, spec := runPinnedExplorePage(t, source, "entry-0", "explore-webview.json")
	if spec.Method != "GET" || !strings.Contains(spec.URL, "filters=0_4_0_0_0&page=1") || spec.WebView {
		t.Fatalf("request=%+v", spec)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "Audio Book" || page.Books[0].BookURL != `https://www.missevan.com/mdrama/drama/42,{"webView":true}` {
		t.Fatalf("page=%+v", page)
	}
}

func TestPinnedExplorePOSTPageBuildsBodyHeadersAndRules(t *testing.T) {
	source := pinnedExploreSource(t, 184)
	page, spec := runPinnedExplorePage(t, source, "entry-1", "explore-post.json")
	if spec.Method != "POST" || spec.Body != "null" || spec.Headers["Content-Type"] != "application/x-www-form-urlencoded;charset=utf-8" || !strings.Contains(spec.URL, "offset=0") {
		t.Fatalf("request=%+v", spec)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "POST Book" || page.Books[0].Author != "POST Author" || !strings.Contains(page.Books[0].BookURL, "postid=99") {
		t.Fatalf("page=%+v", page)
	}
}

func TestExplorePageRoutesWebViewCategory(t *testing.T) {
	source := pinnedExploreSource(t, 160)
	source.ExploreURL = `Browser::https://fixture.test/books,{"webView":true}`
	var normalCalls, browserCalls int
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	searcher.SetTransportFactory(func(*fetcher.Client, *sourceexec.SourceSession) sourceexec.Transport {
		return countingExploreTransport{calls: &normalCalls}
	})
	searcher.SetWebViewTransportFactory(func(*sourceexec.SourceSession) sourceexec.Transport {
		return countingExploreTransport{calls: &browserCalls, body: exploreResponseFixture(t, "explore-webview.json")}
	})
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if normalCalls != 0 || browserCalls != 1 || len(page.Books) != 1 {
		t.Fatalf("normal=%d browser=%d page=%+v", normalCalls, browserCalls, page)
	}
}

type countingExploreTransport struct {
	calls *int
	body  string
}

func (t countingExploreTransport) Do(_ context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	(*t.calls)++
	return sourceexec.Response{StatusCode: 200, Body: t.body, FinalURL: spec.URL}, nil
}

func runPinnedExplorePage(t *testing.T, source booksource.BookSource, categoryID, fixture string) (ExplorePage, sourceexec.RequestSpec) {
	t.Helper()
	var spec sourceexec.RequestSpec
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	searcher.SetTransportFactory(func(*fetcher.Client, *sourceexec.SourceSession) sourceexec.Transport {
		return exploreResponseTransport{body: exploreResponseFixture(t, fixture), spec: &spec}
	})
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: categoryID, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	return page, spec
}

func exploreResponseFixture(t *testing.T, name string) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "..", "testdata", "booksource", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
