// Conformance tests for fingerprint-first transport selection.
package fingerprint

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestClientFallsBackToNormalHTTPAfterFingerprintRejection(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "fingerprint rejected", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	client, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, fetcher.NewInsecure(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Body != "success" {
		t.Fatalf("status=%d body=%q calls=%d", response.StatusCode, response.Body, calls.Load())
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", calls.Load())
	}
}
