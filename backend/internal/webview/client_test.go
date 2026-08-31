// Deterministic contract tests for the Patchright worker client.
package webview

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestClientProbeUsesWorkerExecutionProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request protocolRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if !request.Probe || request.Version != protocolVersion || request.URL != "" {
			t.Fatalf("probe request=%+v", request)
		}
		_ = json.NewEncoder(w).Encode(protocolResponse{Version: protocolVersion, StatusCode: http.StatusOK, Body: "novelreader-webview-ok"})
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Probe(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestClientSendsSessionRequestAndSyncsWorkerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request protocolRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.URL != "https://example.test/page" || request.Method != http.MethodGet {
			t.Errorf("request=%+v", request)
		}
		if request.Headers["x-source"] != "configured" {
			t.Errorf("headers=%v", request.Headers)
		}
		if len(request.Cookies) != 1 || request.Cookies[0].Name != "before" {
			t.Errorf("cookies=%v", request.Cookies)
		}
		if request.WebJS != "document.body.dataset.ready = 'yes'" || request.SourceRegex != `.*\.(mp3|m4a).*` || request.DelayMS != 250 {
			t.Errorf("browser options=%+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(protocolResponse{
			Version: protocolVersion, StatusCode: http.StatusOK, Body: "<html>ok</html>",
			FinalURL: "https://example.test/final",
			Cookies:  []protocolCookie{{Name: "after", Value: "yes", Domain: "example.test", Path: "/"}},
		})
	}))
	defer server.Close()

	session := sourceexec.NewSourceSession()
	if err := session.SetCookie("https://example.test/page", "before", "one"); err != nil {
		t.Fatal(err)
	}
	session.SetRequestHeaders(map[string]string{"x-source": "configured"})
	client, err := NewClient(Config{Endpoint: server.URL, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.ForSession(session).Do(t.Context(), sourceexec.RequestSpec{
		URL: "https://example.test/page", WebView: true, WebJS: "document.body.dataset.ready = 'yes'", SourceRegex: `.*\.(mp3|m4a).*`, WebViewDelay: 250,
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Transport != "webview" || response.FinalURL != "https://example.test/final" || response.Body != "<html>ok</html>" {
		t.Fatalf("response=%+v", response)
	}
	if got := session.GetCookie("https://example.test/final", "after"); got != "yes" {
		t.Fatalf("synced cookie=%q", got)
	}
}

func TestClientRetriesTransientWorkerBusyResponses(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"version":3,"error":"browser worker is busy"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(protocolResponse{Version: protocolVersion, StatusCode: http.StatusOK, Body: "ok"})
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(t.Context(), sourceexec.RequestSpec{URL: "https://example.test"})
	if err != nil || response.Body != "ok" || attempts != 3 {
		t.Fatalf("response=%+v error=%v attempts=%d", response, err, attempts)
	}
}

func TestClientRejectsOlderWorkerProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"statusCode":200,"body":"html instead of sniffed resource"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(t.Context(), sourceexec.RequestSpec{URL: "https://example.test", SourceRegex: `.*\.mp3`})
	if err == nil || !strings.Contains(err.Error(), "unsupported worker protocol 1") {
		t.Fatalf("error=%v", err)
	}
}

func TestClientRejectsWorkerProtocolErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":3,"error":"browser unavailable"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(t.Context(), sourceexec.RequestSpec{URL: "https://example.test"})
	if err == nil || !strings.Contains(err.Error(), "browser unavailable") {
		t.Fatalf("error=%v", err)
	}
}

func TestInteractiveClientStartsInputsAndClosesSession(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sessions":
			var request interactiveRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.URL != "https://example.test/login" || len(request.Cookies) != 1 || request.Viewport.Width != 1200 || request.Viewport.Height != 800 {
				t.Fatalf("request=%+v", request)
			}
			_ = json.NewEncoder(w).Encode(interactiveResult{Version: protocolVersion, InteractiveFrame: InteractiveFrame{SessionID: "session-1", Image: "frame", Width: 390, Height: 720}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/input"):
			_ = json.NewEncoder(w).Encode(interactiveResult{Version: protocolVersion, InteractiveFrame: InteractiveFrame{SessionID: "session-1", Image: "next"}})
		case r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(interactiveResult{Version: protocolVersion, Closed: true, FinalURL: "https://example.test/account", Cookies: []protocolCookie{{Name: "login", Value: "ready", Domain: "example.test", Path: "/"}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	session := sourceexec.NewSourceSession()
	if err := session.SetCookie("https://example.test/login", "before", "one"); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(Config{Endpoint: server.URL, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := client.StartInteractive(t.Context(), "https://example.test/login", "Login", InteractiveViewport{Width: 1200, Height: 800}, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.SendInteractiveInput(t.Context(), frame.SessionID, InteractiveInput{Type: "click", X: 10, Y: 20}); err != nil {
		t.Fatal(err)
	}
	if err := client.CloseInteractive(t.Context(), frame.SessionID, "https://example.test/login", true, session); err != nil {
		t.Fatal(err)
	}
	if session.GetCookie("https://example.test/account", "login") != "ready" {
		t.Fatal("interactive cookie was not synchronized")
	}
	if len(requests) != 3 {
		t.Fatalf("requests=%v", requests)
	}
}
