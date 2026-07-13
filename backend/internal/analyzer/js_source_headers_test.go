// Regression coverage for source headers on JavaScript bridge requests.
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

func TestJavaAjaxSendsSourceHeadersWithStatelessClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Source"); got != "configured" {
			t.Errorf("X-Source=%q", got)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	state := sourceexec.NewSourceSession()
	state.SetRequestHeaders(map[string]string{"x-source": "configured"})
	vm := analyzer.NewJSVM()
	vm.SetFetcher(fetcher.NewInsecureStateless(3 * time.Second))
	value, err := vm.Eval(`java.ajax("`+server.URL+`")`, "", server.URL,
		map[string]interface{}{"sourceState": state})
	if err != nil {
		t.Fatal(err)
	}
	if got := analyzer.ToString(value); got != "ok" {
		t.Fatalf("value=%q", got)
	}
}
