package conformance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkflowReplaysSource779WebViewContent(t *testing.T) {
	bookPage := `<html>
<div class="baseinfo"><img src="/cover.jpg"></div>
<div class="pt-info">分类</div><div class="pt-info">作者：Fixture</div><div class="pt-info"><a>玄幻</a></div>
<div class="intro">Fixture introduction</div>
<ul id="chapterlist"><li><a href="/chapter/1">第一章</a></li></ul>
</html>`
	normal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/book" {
			t.Errorf("normal request path=%q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "detail", Value: "ready", Path: "/"})
		_, _ = w.Write([]byte(bookPage))
	}))
	defer normal.Close()

	browserCalls := 0
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		browserCalls++
		var request struct {
			URL     string `json:"url"`
			Cookies []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"cookies"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(request.URL, "webView") {
			t.Errorf("worker received unparsed URL option: %q", request.URL)
		}
		if !hasReplayCookie(request.Cookies, "detail", "ready") {
			t.Errorf("request %d lost detail cookie: %+v", browserCalls, request.Cookies)
		}

		body := `<div id="BookText">page one</div><a href="/chapter/2">下一页</a>`
		cookies := []map[string]interface{}{{"name": "browser", "value": "ready", "path": "/"}}
		if browserCalls == 2 {
			if !hasReplayCookie(request.Cookies, "browser", "ready") {
				t.Errorf("paginated request lost browser cookie: %+v", request.Cookies)
			}
			body = `<div id="BookText">page two</div>`
			cookies = nil
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "statusCode": 200, "finalUrl": request.URL,
			"body": body, "cookies": cookies,
		})
	}))
	defer worker.Close()

	// These are source 779's real detail/TOC/content rule shapes. Only the host
	// and canned pages are local so the regression remains deterministic.
	source, err := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": normal.URL, "bookSourceName": "桃桃书 replay", "bookSourceType": 0,
		"ruleBookInfo": map[string]string{
			"author": "class.pt-info.1@text##作者：", "coverUrl": "class.baseinfo@img@src",
			"intro": "class.intro@text", "kind": "class.pt-info.2@a@text", "tocUrl": "",
		},
		"ruleToc": map[string]string{
			"chapterList": "id.chapterlist@li", "chapterName": "a@text",
			"chapterUrl": "a@href##$##,{'webView': true}", "nextTocUrl": "text.下一页@href",
		},
		"ruleContent": map[string]string{
			"content":        "id.BookText@html",
			"nextContentUrl": "text.下一页@href##$##,{'webView': true}",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	record, err := RunWorkflowWithOptions(context.Background(), source, 0, normal.URL+"/book", Options{
		Timeout: 2 * time.Second, WebViewEndpoint: worker.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Classification != "success" || record.ChapterCount != 1 {
		t.Fatalf("record=%+v", record)
	}
	if record.FirstChapter == nil || !strings.Contains(record.FirstChapter.URL, "{'webView': true}") {
		t.Fatalf("chapter=%+v", record.FirstChapter)
	}
	if !strings.Contains(record.ContentSample, "page one") || !strings.Contains(record.ContentSample, "page two") || browserCalls != 2 {
		t.Fatalf("content=%q browserCalls=%d", record.ContentSample, browserCalls)
	}
}

func hasReplayCookie(cookies []struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}, name, value string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value == value {
			return true
		}
	}
	return false
}
