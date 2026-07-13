// Regression test for aborting conformance runs after a server crash.
package conformance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunSearchAbortsWhenHealthCheckFails(t *testing.T) {
	var searches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			if searches.Load() > 0 {
				http.Error(w, "server stopped", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte("ok"))
		case "/search":
			searches.Add(1)
			_, _ = w.Write([]byte(`<div class="bookbox"><a class="book" href="/book/1">书</a></div>`))
		}
	}))
	defer server.Close()

	source, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": server.URL, "bookSourceName": "health fixture", "bookSourceType": 0,
		"searchUrl":  server.URL + `/search,{"method":"GET"}`,
		"ruleSearch": map[string]string{"bookList": ".bookbox", "name": ".book@text", "bookUrl": ".book@href"},
	}})
	records, err := RunSearchWithOptions(context.Background(), source, []int{0}, "书", Options{
		Timeout: 2 * time.Second, HealthURL: server.URL + "/health",
	})
	if err == nil || !strings.Contains(err.Error(), "health check") {
		t.Fatalf("err=%v records=%d", err, len(records))
	}
	if len(records) != 1 {
		t.Fatalf("records=%d", len(records))
	}
}
