// Regression coverage for URL-option HTTP methods.
package sourceexec

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHTTPTransportExecutesHeadMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method=%q", r.Method)
		}
		w.Header().Set("X-Method", "head")
	}))
	defer server.Close()

	transport := NewHTTPTransport(fetcher.NewWithTimeout(3 * time.Second))
	response, err := transport.Do(t.Context(), RequestSpec{URL: server.URL, Method: http.MethodHead})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Headers["X-Method"][0] != "head" {
		t.Fatalf("response=%+v", response)
	}
}
