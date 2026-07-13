// Regression test for cancelling JavaScript HTTP helpers with the source request.
package analyzer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestEvalContextCancelsJavaAjax(t *testing.T) {
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()

	vm := NewJSVM()
	vm.SetFetcher(fetcher.NewWithTimeout(5 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := vm.EvalContext(ctx, `java.ajax("`+server.URL+`")`, "", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("JavaScript request outlived evaluation context")
	}
}
