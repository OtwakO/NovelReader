// Deterministic tests for raw source identity and request/search reporting.
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

func TestRunSearchExecutesTypedDataThroughRoutingTransport(t *testing.T) {
	raw, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": "fixture", "bookSourceName": "typed fixture", "bookSourceType": 0,
		"searchUrl":  `data:;base64,PGEgY2xhc3M9ImJvb2siIGhyZWY9Ii9ib29rIj5GaXh0dXJlPC9hPg==,{"type":"fixture","bodyJs":"java.hexDecodeToString(result)"}`,
		"ruleSearch": map[string]string{"bookList": ".book", "name": "text", "bookUrl": "href"},
	}})
	records, err := RunSearch(t.Context(), raw, []int{0}, "fixture", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Classification != "success" || records[0].Response == nil || records[0].Response.Transport != "data" {
		t.Fatalf("records=%+v", records)
	}
}

func TestRunSearchBuildsURLWithSourceJSLib(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "fixture" {
			http.Error(w, "missing jsLib query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`<a class="book" href="/book">Fixture</a>`))
	}))
	defer server.Close()

	raw, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": server.URL, "bookSourceName": "jsLib fixture", "bookSourceType": 0,
		"jsLib":      `function makeSearch(value) { return baseUrl + '/search?q=' + value; }`,
		"searchUrl":  `<js>makeSearch(key)</js>`,
		"ruleSearch": map[string]string{"bookList": ".book", "name": "text", "bookUrl": "href"},
	}})
	records, err := RunSearch(t.Context(), raw, []int{0}, "fixture", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Classification != "success" {
		t.Fatalf("records=%+v", records)
	}
}

func TestRunSearchRecordsExpandedRequestAndExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.FormValue("q") != "凡人修仙传" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Diagnostic", "visible")
		w.Header().Set("Set-Cookie", "session=secret")
		_, _ = w.Write([]byte(`<div class="bookbox"><a class="book" href="/book/1">凡人修仙传</a></div>`))
	}))
	defer server.Close()

	source := map[string]interface{}{
		"bookSourceUrl":  server.URL,
		"bookSourceName": "raw fixture",
		"bookSourceType": 0,
		"enabled":        true,
		"searchUrl":      server.URL + `/search,{"method":"POST","body":"q={{key}}"}`,
		"ruleSearch": map[string]string{
			"bookList": ".bookbox",
			"name":     ".book@text",
			"bookUrl":  ".book@href",
		},
	}
	raw, err := json.Marshal([]interface{}{source})
	if err != nil {
		t.Fatal(err)
	}

	records, err := RunSearch(context.Background(), raw, []int{0}, "凡人修仙传", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records=%d", len(records))
	}
	record := records[0]
	if record.Identity.Index != 0 || len(record.Identity.SHA256) != 64 {
		t.Fatalf("identity=%+v", record.Identity)
	}
	if record.Classification != "success" || len(record.Extracted) != 1 {
		t.Fatalf("record=%+v", record)
	}
	if record.Request.Method != "POST" || record.Request.Body != "q=凡人修仙传" {
		t.Fatalf("request=%+v", record.Request)
	}
	if record.Response.Headers["X-Diagnostic"][0] != "visible" || record.Response.Headers["Set-Cookie"][0] != "[redacted]" {
		t.Fatalf("response diagnostics=%+v", record.Response)
	}
	if record.Extracted[0].Name != "凡人修仙传" || !strings.HasSuffix(record.Extracted[0].BookURL, "/book/1") {
		t.Fatalf("extracted=%+v", record.Extracted)
	}
}

func TestRunSearchRejectsUnknownRawIndex(t *testing.T) {
	raw := []byte(`[{"bookSourceUrl":"https://example.test"}]`)
	_, err := RunSearch(context.Background(), raw, []int{1}, "key", time.Second)
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err=%v", err)
	}
}
