// Golden classification coverage for the Phase 0 failure taxonomy.
package conformance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunSearchGoldenClassifications(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			_, _ = w.Write([]byte(`<div class="bookbox"><a class="book" href="/book/1">成功</a></div>`))
		case "/zero":
			_, _ = w.Write([]byte(`<div class="bookbox"></div>`))
		case "/mismatch":
			_, _ = w.Write([]byte(`<main>没有匹配规则</main>`))
		case "/webview":
			_, _ = w.Write([]byte("unused"))
		}
	}))
	defer server.Close()

	makeSource := func(path string, sourceType int, rule map[string]string) map[string]interface{} {
		return map[string]interface{}{
			"bookSourceUrl": server.URL, "bookSourceName": path, "bookSourceType": sourceType,
			"searchUrl":  server.URL + path,
			"ruleSearch": rule,
		}
	}
	rule := map[string]string{"bookList": ".bookbox", "name": ".book@text", "bookUrl": ".book@href"}
	raw, err := json.Marshal([]map[string]interface{}{
		makeSource("/success", 0, rule),
		makeSource("/zero", 0, rule),
		// A missing required rule is invalid; an unmatched selector alone now
		// legitimately permits the reference's empty-list detail fallback.
		makeSource("/mismatch", 0, map[string]string{"name": ".book@text"}),
		makeSource("/webview", 1, rule),
	})
	if err != nil {
		t.Fatal(err)
	}

	records, err := RunSearch(context.Background(), raw, nil, "书", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, record.Classification)
	}
	want := []string{"success", "legitimate_zero_results", "rule_mismatch", "unsupported_webview"}
	if len(got) != len(want) {
		t.Fatalf("classifications=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("classifications=%v want=%v", got, want)
		}
	}
}
