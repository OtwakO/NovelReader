// Regression test for cancelling JavaScript HTTP helpers with the source request.
package analyzer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestEvalContextInterruptsCPULoopAndReusesRuntime(t *testing.T) {
	vm := NewJSVMWithPoolSize(1)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := vm.EvalContext(ctx, `while (true) {}`, "", "https://fixture.test"); err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want deadline", err)
	}
	value, err := vm.EvalContext(t.Context(), `1 + 1`, "", "https://fixture.test")
	if err != nil || value != int64(2) {
		t.Fatalf("runtime after interrupt: value=%v err=%v", value, err)
	}
}

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
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want deadline", err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("JavaScript request outlived evaluation context")
	}
}
