// Regression test for callable JavaScript response methods returned by java.get.
package analyzer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestJavaGetResponseHeaderIsCallable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/next")
		_, _ = w.Write([]byte("body"))
	}))
	defer server.Close()

	vm := NewJSVM()
	vm.SetFetcher(fetcher.NewWithTimeout(3 * time.Second))
	value, err := vm.Eval(`java.get("`+server.URL+`").header("location")`, "", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := ToString(value); got != "/next" {
		t.Fatalf("header=%q", got)
	}
}
