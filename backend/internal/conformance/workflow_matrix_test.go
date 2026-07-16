// End-to-end source-class matrix for detail, TOC, and chapter content.
package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkflowSourceClassMatrix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(serveWorkflowMatrix))
	defer server.Close()

	cases := []struct {
		name    string
		source  map[string]interface{}
		bookURL string
		want    string
	}{
		{name: "normal HTML", source: matrixHTMLSource(server.URL, "html"), bookURL: server.URL + "/html/book", want: "html content"},
		{name: "JSON", source: matrixJSONSource(server.URL), bookURL: server.URL + "/json/book", want: "json content"},
		{name: "XPath and Regex", source: matrixXPathRegexSource(server.URL), bookURL: server.URL + "/xpath/book", want: "regex content"},
		{name: "POST charset", source: matrixHTMLSource(server.URL, "post"), bookURL: server.URL + `/post/book,{"method":"POST","body":"q=中文","charset":"gbk"}`, want: "post content"},
		{name: "multi-page TOC and content", source: matrixMultiPageSource(server.URL), bookURL: server.URL + "/multi/book", want: "page two"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal([]map[string]interface{}{test.source})
			if err != nil {
				t.Fatal(err)
			}
			record, err := RunWorkflowWithOptions(context.Background(), raw, 0, test.bookURL, Options{Timeout: 2 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if record.Classification != "success" || len(record.ChapterChecks) != 3 {
				t.Fatalf("record=%+v", record)
			}
			for _, check := range record.ChapterChecks {
				if !strings.Contains(check.ContentSample, test.want) {
					t.Fatalf("%s content=%q, want %q", check.Position, check.ContentSample, test.want)
				}
			}
		})
	}
}

func serveWorkflowMatrix(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	kind, stage := parts[0], parts[1]
	if kind == "post" && stage == "book" {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || string(body) != "q=%D6%D0%CE%C4" {
			http.Error(w, "expected GBK-encoded POST", http.StatusBadRequest)
			return
		}
	}
	if kind == "json" {
		serveJSONWorkflow(w, stage, parts)
		return
	}
	if kind == "xpath" {
		serveXPathWorkflow(w, stage, parts)
		return
	}
	if kind == "multi" {
		serveMultiPageWorkflow(w, stage, parts, r.URL.Query().Get("page"))
		return
	}
	serveHTMLWorkflow(w, kind, stage, parts)
}

func serveHTMLWorkflow(w http.ResponseWriter, kind, stage string, parts []string) {
	switch stage {
	case "book":
		_, _ = fmt.Fprintf(w, `<h1 class="name">%s book</h1><a class="toc" href="/%s/toc">toc</a>`, kind, kind)
	case "toc":
		for i := 1; i <= 3; i++ {
			_, _ = fmt.Fprintf(w, `<a class="chapter" href="/%s/chapter/%d">chapter %d</a>`, kind, i, i)
		}
	case "chapter":
		_, _ = fmt.Fprintf(w, `<article class="content">%s content %s</article>`, kind, parts[2])
	default:
		http.NotFound(w, nil)
	}
}

func serveJSONWorkflow(w http.ResponseWriter, stage string, parts []string) {
	w.Header().Set("Content-Type", "application/json")
	switch stage {
	case "book":
		_, _ = fmt.Fprint(w, `{"name":"json book","toc":"/json/toc"}`)
	case "toc":
		_, _ = fmt.Fprint(w, `{"chapters":[{"name":"chapter 1","url":"/json/chapter/1"},{"name":"chapter 2","url":"/json/chapter/2"},{"name":"chapter 3","url":"/json/chapter/3"}]}`)
	case "chapter":
		_, _ = fmt.Fprintf(w, `{"content":"json content %s"}`, parts[2])
	}
}

func serveXPathWorkflow(w http.ResponseWriter, stage string, parts []string) {
	switch stage {
	case "book":
		_, _ = fmt.Fprint(w, `<book><name>xpath book</name><toc href="/xpath/toc"/></book>`)
	case "toc":
		_, _ = fmt.Fprint(w, `<chapters><a href="/xpath/chapter/1">chapter 1</a><a href="/xpath/chapter/2">chapter 2</a><a href="/xpath/chapter/3">chapter 3</a></chapters>`)
	case "chapter":
		_, _ = fmt.Fprintf(w, "title=chapter %s\ncontent=regex content %s", parts[2], parts[2])
	}
}

func serveMultiPageWorkflow(w http.ResponseWriter, stage string, parts []string, page string) {
	switch stage {
	case "book":
		_, _ = fmt.Fprint(w, `<h1 class="name">multi book</h1><a class="toc" href="/multi/toc">toc</a>`)
	case "toc":
		if page == "2" {
			_, _ = fmt.Fprint(w, `<a class="chapter" href="/multi/chapter/3">chapter 3</a>`)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="chapter" href="/multi/chapter/1">chapter 1</a><a class="chapter" href="/multi/chapter/2">chapter 2</a><a class="next" href="/multi/toc?page=2">next</a>`)
	case "chapter":
		if page == "2" {
			_, _ = fmt.Fprintf(w, `<article class="content">page two %s</article>`, parts[2])
			return
		}
		_, _ = fmt.Fprintf(w, `<article class="content">page one %s</article><a class="next" href="/multi/chapter/%s?page=2">next</a>`, parts[2], parts[2])
	}
}

func matrixHTMLSource(baseURL, kind string) map[string]interface{} {
	return map[string]interface{}{
		"bookSourceUrl": baseURL, "bookSourceName": kind, "bookSourceType": 0,
		"ruleBookInfo": map[string]string{"name": ".name@text", "tocUrl": ".toc@href"},
		"ruleToc":      map[string]string{"chapterList": ".chapter", "chapterName": "text", "chapterUrl": "href"},
		"ruleContent":  map[string]string{"content": ".content@text"},
	}
}

func matrixJSONSource(baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"bookSourceUrl": baseURL, "bookSourceName": "json", "bookSourceType": 0,
		"ruleBookInfo": map[string]string{"name": "@Json:$.name", "tocUrl": "@Json:$.toc"},
		"ruleToc":      map[string]string{"chapterList": "@Json:$.chapters[*]", "chapterName": "@Json:$.name", "chapterUrl": "@Json:$.url"},
		"ruleContent":  map[string]string{"content": "@Json:$.content"},
	}
}

func matrixXPathRegexSource(baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"bookSourceUrl": baseURL, "bookSourceName": "xpath-regex", "bookSourceType": 0,
		"ruleBookInfo": map[string]string{"name": "@XPath://book/name", "tocUrl": "@XPath://book/toc/@href"},
		"ruleToc":      map[string]string{"chapterList": "@XPath://chapters/a", "chapterName": "@XPath://a/text()", "chapterUrl": "@XPath://a/@href"},
		"ruleContent":  map[string]string{"content": "##content=([^\\n]+)##$1"},
	}
}

func matrixMultiPageSource(baseURL string) map[string]interface{} {
	source := matrixHTMLSource(baseURL, "multi")
	source["ruleToc"] = map[string]string{"chapterList": ".chapter", "chapterName": "text", "chapterUrl": "href", "nextTocUrl": ".next@href"}
	source["ruleContent"] = map[string]string{"content": ".content@text", "nextContentUrl": ".next@href"}
	return source
}
