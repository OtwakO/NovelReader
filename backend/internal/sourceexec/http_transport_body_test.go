// Regression coverage for URL-option POST body preparation.
package sourceexec

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHTTPTransportSendsJSONBodyAsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if got := string(body); got != `{"q":"搜索"}` {
			t.Errorf("body=%q", got)
		}
	}))
	defer server.Close()

	transport := NewHTTPTransport(fetcher.NewWithTimeout(3 * time.Second))
	if _, err := transport.Do(t.Context(), RequestSpec{
		URL: server.URL, Method: http.MethodPost, Body: `{"q":"搜索"}`, Charset: "gbk",
	}); err != nil {
		t.Fatal(err)
	}
}
