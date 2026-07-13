// Regression test for resolving a JavaScript-returned relative URL after redirect.
package analyzer_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestBuildURLUsesFinalURLAfterJavaAjaxRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/new/base", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("token"))
	}))
	defer server.Close()

	vm := analyzer.NewJSVM()
	vm.SetFetcher(fetcher.NewWithTimeout(3 * time.Second))
	meta, err := analyzer.BuildURLWithState(
		`@js:java.ajax("`+server.URL+`/start"); "child"`,
		"", 1, server.URL+"/old/source", vm, sourceexec.NewSourceSession(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != server.URL+"/new/child" {
		t.Fatalf("url=%q", meta.URL)
	}
}
