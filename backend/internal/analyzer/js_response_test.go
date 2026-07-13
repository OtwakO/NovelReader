// Conformance test for Legado StrResponse body() access.
package analyzer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestJavaPostResponseSupportsBodyMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":"正文"}`))
	}))
	defer server.Close()

	vm := NewJSVM()
	vm.SetFetcher(fetcher.NewWithTimeout(3 * time.Second))
	value, err := vm.Eval(`JSON.parse(java.post("`+server.URL+`", "x", {}).body()).content`, "", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := ToString(value); got != "正文" {
		t.Fatalf("value=%q", got)
	}
}
