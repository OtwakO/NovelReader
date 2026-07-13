// Regression coverage for URL-option DNS/IP overrides.
package sourceexec

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHTTPTransportUsesDNSIPWithoutChangingRequestHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Host, "example.test:") {
			t.Errorf("Host=%q", r.Host)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "127.0.0.1", "example.test", 1)
	transport := NewHTTPTransport(fetcher.NewWithTimeout(3 * time.Second))
	response, err := transport.Do(t.Context(), RequestSpec{URL: url, DNSIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "ok" {
		t.Fatalf("body=%q", response.Body)
	}
}
