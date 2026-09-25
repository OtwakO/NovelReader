package fetcher

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTextResponseLimit(t *testing.T) {
	const limit = 10 * 1024 * 1024
	for _, tc := range []struct {
		name string
		size int
	}{{"exact-limit", limit}, {"over-limit", limit + 1}} {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.size
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.(http.Flusher).Flush() // Exercise an unknown Content-Length, not just a header check.
				_, _ = w.Write([]byte(strings.Repeat("a", size)))
			}))
			defer upstream.Close()
			response, err := New().GetContext(t.Context(), upstream.URL, nil)
			if size > limit {
				if err == nil || response != nil {
					t.Fatal("oversized response accepted as success")
				}
			} else if err != nil || len(response.Body) != size {
				t.Fatalf("exact-limit response rejected or truncated: %v", err)
			}
		})
	}
}
