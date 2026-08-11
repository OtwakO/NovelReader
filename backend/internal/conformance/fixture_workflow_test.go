// Exercises transport-facing contracts with the checked-in response fixtures.
package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestFixtureJavaScriptPostUsesExpandedRequest(t *testing.T) {
	body := readFixture(t, "js-post.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.FormValue("q") != "凡人修仙传" {
			http.Error(w, "request mismatch", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	source, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": server.URL, "bookSourceName": "fixture JS POST", "bookSourceType": 0,
		"searchUrl": fmt.Sprintf(`@js:%q + ',{"method":"POST","body":"q=' + key + '"}'`, server.URL+"/search"),
		"ruleSearch": map[string]string{
			"bookList": ".result", "name": "@CSS:a@text", "bookUrl": "@CSS:a@href",
		},
	}})
	records, err := RunSearch(context.Background(), source, []int{0}, "凡人修仙传", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Classification != "success" {
		t.Fatalf("records=%+v", records)
	}
}

func TestFixtureGBKResponseIsDecodedThroughTransport(t *testing.T) {
	body := readFixture(t, "post-gbk.html")
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=gbk")
		_, _ = w.Write(encoded)
	}))
	defer server.Close()

	transport := sourceexec.NewHTTPTransport(fetcher.NewInsecure(2 * time.Second))
	response, err := transport.Do(context.Background(), sourceexec.RequestSpec{URL: server.URL, Charset: "gbk"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != body {
		t.Fatalf("decoded body=%q want=%q", response.Body, body)
	}
}

func TestFixtureCookieResponseUsesSourceSession(t *testing.T) {
	body := readFixture(t, "cookie.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" {
			http.SetCookie(w, &http.Cookie{Name: "fixture", Value: "ok", Path: "/"})
			return
		}
		if r.URL.Path == "/second" {
			cookie, _ := r.Cookie("fixture")
			if cookie == nil {
				http.Error(w, "cookie missing", http.StatusForbidden)
				return
			}
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	session := sourceexec.NewSourceSession()
	transport := sourceexec.NewHTTPTransportForSession(fetcher.NewInsecure(2*time.Second), session)
	if _, err := transport.Do(context.Background(), sourceexec.RequestSpec{URL: server.URL + "/first"}); err != nil {
		t.Fatal(err)
	}
	response, err := transport.Do(context.Background(), sourceexec.RequestSpec{URL: server.URL + "/second"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != body {
		t.Fatalf("body=%q want fixture body", response.Body)
	}
}

func TestFixturePaginationResponsesRemainSeparate(t *testing.T) {
	pageOne := readFixture(t, "pagination-1.html")
	pageTwo := readFixture(t, "pagination-2.html")
	first := newFixtureAnalyzer(pageOne).GetString
	next, err := first("@CSS:.next@href")
	if err != nil || next != "/toc?page=2" {
		t.Fatalf("next=%q err=%v", next, err)
	}
	chapter, err := newFixtureAnalyzer(pageTwo).GetString("@CSS:.chapter@text")
	if err != nil || chapter != "第二章" {
		t.Fatalf("chapter=%q err=%v", chapter, err)
	}
}

func TestFixtureWebViewUsesConfiguredBrowserTransport(t *testing.T) {
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Version int `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Version != 2 {
			t.Fatalf("worker request=%+v err=%v", request, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 2, "statusCode": 200, "finalUrl": "https://fixture.test/rendered",
			"body": `<div class="book"><a class="name" href="/book">browser fixture</a></div>`,
		})
	}))
	defer worker.Close()

	source, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": "https://fixture.test", "bookSourceName": "fixture browser", "bookSourceType": 0,
		"searchUrl":  `https://fixture.test/search,{"webView":true}`,
		"ruleSearch": map[string]string{"bookList": ".book", "name": ".name@text", "bookUrl": ".name@href"},
	}})
	records, err := RunSearchWithOptions(context.Background(), source, []int{0}, "书", Options{
		Timeout: 2 * time.Second, WebViewEndpoint: worker.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Classification != "success" || records[0].Response.Transport != "webview" {
		t.Fatalf("records=%+v", records)
	}
	if len(records[0].Extracted) != 1 || records[0].Extracted[0].Name != "browser fixture" {
		t.Fatalf("results=%+v", records[0].Extracted)
	}
}

func TestFixtureWebViewIsClassifiedBeforeHTTPParsing(t *testing.T) {
	body := readFixture(t, "webview-shell.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	source, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": server.URL, "bookSourceName": "fixture WebView", "bookSourceType": 1,
		"searchUrl": server.URL, "ruleSearch": map[string]string{"bookList": "#app"},
	}})
	records, err := RunSearch(context.Background(), source, []int{0}, "书", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Classification != "unsupported_webview" {
		t.Fatalf("records=%+v", records)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate fixture test")
	}
	root := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "testdata", "booksource", "conformance", "core")
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(body), "\n")
}

func newFixtureAnalyzer(content string) *analyzer.Analyzer {
	return analyzer.New(content, "https://fixture.test/", analyzer.NewJSVM(), nil)
}
