// Conformance test for URL-option response charset handling.
package sourceexec

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestHTTPTransportEncodesPostBodyUsingRequestCharset(t *testing.T) {
	encodedValue, _, err := transform.String(simplifiedchinese.GBK.NewEncoder(), "凡人修仙传")
	if err != nil {
		t.Fatal(err)
	}
	want := "q=" + url.QueryEscape(encodedValue)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != want {
			t.Errorf("request body=%q want=%q", body, want)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	response, err := NewHTTPTransport(fetcher.NewWithTimeout(3*time.Second)).Do(t.Context(), RequestSpec{
		URL: server.URL, Method: http.MethodPost, Body: "q=凡人修仙传", Charset: "gbk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "ok" {
		t.Fatalf("body=%q", response.Body)
	}
}

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
