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

func TestWorkflowRoutesWebViewTOCAndContent(t *testing.T) {
	detail := `<html><h1 class="title">Fixture novel</h1><a class="toc" href='/toc,{"webView":true}'>目录</a></html>`
	normal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/book" {
			t.Errorf("normal request path=%q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(detail))
	}))
	defer normal.Close()

	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		body := `<a class="chapter" href='/content,{"webView":true}'><span class="name">第一章</span></a>`
		if strings.Contains(request.URL, "/content") {
			body = `<article class="content">rendered browser chapter</article>`
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "statusCode": 200, "finalUrl": request.URL, "body": body,
		})
	}))
	defer worker.Close()

	source, err := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": normal.URL, "bookSourceName": "workflow fixture", "bookSourceType": 0,
		"ruleBookInfo": map[string]string{"name": ".title@text", "tocUrl": ".toc@href"},
		"ruleToc":      map[string]string{"chapterList": ".chapter", "chapterName": ".name@text", "chapterUrl": "href"},
		"ruleContent":  map[string]string{"content": ".content@text"},
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
	if record.FirstChapter == nil || !strings.Contains(record.FirstChapter.URL, "/content") {
		t.Fatalf("chapter=%+v", record.FirstChapter)
	}
	if record.ContentSample != "rendered browser chapter" {
		t.Fatalf("content=%q", record.ContentSample)
	}
}
