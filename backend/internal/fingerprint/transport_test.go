// Conformance test for fingerprint-first regular source transport.
package fingerprint

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type captureTransport struct{}

func (captureTransport) Do(context.Context, sourceexec.RequestSpec) (sourceexec.Response, error) {
	return sourceexec.Response{StatusCode: http.StatusOK, Body: "normal", Transport: "http"}, nil
}

func TestTransportDelegatesDNSIPToNormalFallback(t *testing.T) {
	transport, err := NewTransport(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, captureTransport{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.Do(t.Context(), sourceexec.RequestSpec{URL: "http://example.test/", DNSIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "normal" || response.Transport != "http" {
		t.Fatalf("response=%+v", response)
	}
}

func TestTransportHonorsRetryAfterFingerprintNonSuccess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	normal := sourceexec.NewHTTPTransport(fetcher.NewInsecure(5 * time.Second))
	transport, err := NewTransport(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, normal, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.Do(t.Context(), sourceexec.RequestSpec{URL: server.URL, Method: http.MethodGet, Retry: 1})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "ok" || calls.Load() != 3 {
		t.Fatalf("body=%q calls=%d", response.Body, calls.Load())
	}
}

func TestTransportCloseIdleConnectionsReleasesScopedPool(t *testing.T) {
	var mu sync.Mutex
	states := make(map[net.Conn]http.ConnState)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	server.Config.ConnState = func(connection net.Conn, state http.ConnState) {
		mu.Lock()
		states[connection] = state
		mu.Unlock()
	}
	server.Start()
	defer server.Close()

	transport, err := NewTransport(Config{Timeout: time.Second}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transport.Do(t.Context(), sourceexec.RequestSpec{URL: server.URL}); err != nil {
		t.Fatal(err)
	}
	if !waitForConnectionState(time.Second, &mu, states, http.StateIdle) {
		t.Fatal("fingerprint connection never became idle")
	}
	transport.CloseIdleConnections()
	if !waitForNoOpenConnections(time.Second, &mu, states) {
		t.Fatalf("scoped fingerprint connections remained open: %+v", states)
	}
}

func TestTransportFallsBackForRegularSourceRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "reject fingerprint", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	normal := sourceexec.NewHTTPTransport(fetcher.NewInsecure(5 * time.Second))
	transport, err := NewTransport(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, normal, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.Do(t.Context(), sourceexec.RequestSpec{URL: server.URL, Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "ok" || response.Transport != "http" {
		t.Fatalf("body=%q transport=%q calls=%d", response.Body, response.Transport, calls.Load())
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", calls.Load())
	}
}

func waitForConnectionState(timeout time.Duration, mu *sync.Mutex, states map[net.Conn]http.ConnState, wanted http.ConnState) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		found := false
		for _, state := range states {
			found = found || state == wanted
		}
		mu.Unlock()
		if found {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func waitForNoOpenConnections(timeout time.Duration, mu *sync.Mutex, states map[net.Conn]http.ConnState) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		open := false
		for _, state := range states {
			open = open || (state != http.StateClosed && state != http.StateHijacked)
		}
		mu.Unlock()
		if !open {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}
