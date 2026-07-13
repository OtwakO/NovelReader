// Conformance test for URL-option response charset handling.
package sourceexec

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestHTTPTransportDecodesResponseUsingRequestCharset(t *testing.T) {
	gbkBody, _, err := transform.String(simplifiedchinese.GBK.NewEncoder(), "章节内容")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(gbkBody))
	}))
	defer server.Close()

	response, err := NewHTTPTransport(fetcher.NewWithTimeout(3*time.Second)).Do(t.Context(), RequestSpec{
		URL:     server.URL,
		Method:  http.MethodGet,
		Charset: "gbk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "章节内容" {
		t.Fatalf("body = %q, want decoded GBK content", response.Body)
	}
}
