package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type webViewProbeStub struct{ err error }

func (p webViewProbeStub) Probe(context.Context) error { return p.err }

func TestWebViewStatusDistinguishesConfigurationAndExecution(t *testing.T) {
	tests := []struct {
		name  string
		probe interface{ Probe(context.Context) error }
		want  string
	}{
		{name: "not configured", want: `"status":"not_configured"`},
		{name: "unavailable", probe: webViewProbeStub{err: errors.New("worker down")}, want: `"status":"unavailable"`},
		{name: "ready", probe: webViewProbeStub{}, want: `"status":"ready"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{webViewProbe: test.probe, mux: http.NewServeMux()}
			server.registerRoutesWithoutHealth()
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/webview-status", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.want) || !strings.Contains(response.Body.String(), `"checkedAt":`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestWebViewStatusRouteIsAuthenticated(t *testing.T) {
	server, _, _, _, closeServer := newOwnershipServer(t)
	defer closeServer()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/webview-status", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
