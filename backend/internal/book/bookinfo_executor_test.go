// Integration test for book-info requests using Legado URL options.
package book

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestGetBookInfoAppliesInitBeforeDetailRules(t *testing.T) {
	fixtures := map[string]string{
		"/css":     `<div><span class="name">Wrong</span></div><table><tr class="noise"><td>Noise</td></tr><tr class="detail"><td class="name">CSS Book</td><td class="author">CSS Author</td></tr></table>`,
		"/mutable": `<h1 class="name">Mutable Book</h1>`,
		"/json":    `{"wrong":{"name":"Wrong"},"payload":{"name":"JSON Book","author":"JSON Author"}}`,
		"/array":   `{"payload":[{"name":"Wrong"},{"name":"Array Book","author":"Array Author"}]}`,
		"/js":      `{"wrong":{"name":"Wrong"},"payload":{"name":"JS Book","author":"JS Author"}}`,
		"/jsoup":   `<table><tr class="detail"><td class="name">JSoup Book</td><td class="author">JSoup Author</td></tr></table>`,
		"/null":    `{"payload":null}`,
		"/put":     `<h1 class="name">Put Book</h1><span class="author">Put Author</span>`,
		"/regex":   `<script>window.book={"name":"Regex Book","author":"Regex Author"}</script>`,
		"/xpath":   `<ul>` + strings.Repeat(`<li></li>`, 55) + `<li><span class="name">XPath Book</span><span class="author">XPath Author</span></li></ul>`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtures[r.URL.Path]))
	}))
	defer server.Close()

	tests := []struct {
		name, path, init, nameRule, authorRule, wantName, wantAuthor, wantErr string
	}{
		{"CSS merged collection", "/css", ".noise&&tr.detail", ".name@text", ".author@text", "CSS Book", "CSS Author", ""},
		{"mutable book context", "/mutable", "", ".name@text", `<js>book.name + ' Author'</js>`, "Mutable Book", "Mutable Book Author", ""},
		{"CSS alternative", "/css", ".missing||tr.detail", ".name@text", ".author@text", "CSS Book", "CSS Author", ""},
		{"JSON object", "/json", "$.payload", "$.name", "$.author", "JSON Book", "JSON Author", ""},
		{"JSON array", "/array", "$.payload", "$[1].name", "$[1].author", "Array Book", "Array Author", ""},
		{"JavaScript object", "/js", `<js>JSON.parse(result).payload</js>`, `<js>result.name</js>`, `<js>result.author</js>`, "JS Book", "JS Author", ""},
		{"JavaScript JSoup collection", "/jsoup", `<js>org.jsoup.Jsoup.parse(result).select("tr.detail")</js>`, ".name@text", ".author@text", "JSoup Book", "JSoup Author", ""},
		{"XPath collection beyond 50", "/xpath", "@XPath://li", ".name@text", ".author@text", "XPath Book", "XPath Author", ""},
		{"put/get variables", "/put", `@put:{n:".name@text",a:'.author@text'}`, `@get:{n}`, `@get:{a}`, "Put Book", "Put Author", ""},
		{"regex capture", "/regex", `:(\{"name".*?\})`, `<js>JSON.parse(result[0]).name</js>`, `<js>JSON.parse(result[0]).author</js>`, "Regex Book", "Regex Author", ""},
		{"null result", "/null", "$.payload", "$.name", "$.author", "", "", "init rule returned null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
			rule, err := json.Marshal(map[string]string{
				"init": tt.init, "name": tt.nameRule, "author": tt.authorRule,
			})
			if err != nil {
				t.Fatal(err)
			}
			book, err := s.GetBookInfo(booksource.BookSource{
				BookSourceURL: server.URL,
				RuleBookInfo:  string(rule),
			}, server.URL+tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if book.Name != tt.wantName || book.Author != tt.wantAuthor {
				t.Fatalf("book = %+v, want name %q and author %q", book, tt.wantName, tt.wantAuthor)
			}
		})
	}
}

func TestGetBookInfoKeepsMutableBookContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<h1 class="name">Mutable Book</h1>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleBookInfo:  `{"name":"@css:.name@text","author":"<js>book.author='Mutated Author'; book.author</js>","intro":"<js>book.author + ' intro'</js>"}`,
	}
	book, err := s.GetBookInfo(src, server.URL+"/book")
	if err != nil {
		t.Fatal(err)
	}
	if book.Author != "Mutated Author" || book.Intro != "Mutated Author intro" {
		t.Fatalf("book=%+v, want mutable author in later fields", book)
	}
}

func TestGetBookInfoUsesDetailURLOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/book" {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if err := r.ParseForm(); err != nil || r.Form.Get("id") != "1" {
				t.Errorf("form = %v, want id=1", r.Form)
			}
			http.Redirect(w, r, "/details/", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<html><h1 class="name">凡人修仙传</h1><span class="author">忘语</span><img class="cover" src="cover.jpg"></html>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleBookInfo:   `{"name":".name@text","author":".author@text","coverUrl":".cover@src"}`,
	}

	book, err := s.GetBookInfo(src, `/book,{"method":"POST","body":"id=1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if book.Name != "凡人修仙传" || book.Author != "忘语" {
		t.Fatalf("book = %+v, want parsed detail", book)
	}
	if book.CoverURL != server.URL+"/details/cover.jpg" {
		t.Fatalf("cover URL = %q, want final-page-relative URL", book.CoverURL)
	}
}
