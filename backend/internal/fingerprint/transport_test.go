// Conformance test for fingerprint-first regular source transport.
package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
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
