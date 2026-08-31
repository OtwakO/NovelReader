package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
	"github.com/otwako/novelreader/internal/webview"
)

func TestSourceInteractionHTTPReturnsNormalizedControlsWithoutAuthentication(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Fixture",
		LoginUI: `[{"name":"账号","type":"text"},{"name":"登录","type":"button","action":"login()"}]`}
	server := NewServer(nil, nil, nil, nil, nil, analyzer.NewJSVM(), nil, processor.DefaultConfig(), "", nil)
	server.sourceInteractions = sourceinteraction.NewDescriber(apiInteractionSourceStore{source}, &apiInteractionProfileStore{profile: sourceprofile.Profile{
		SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{"token":"secret"}`),
	}}, analyzer.NewJSVM())
	request := httptest.NewRequest(http.MethodGet, "/api/sources/source-a/interaction", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "secret") || !strings.Contains(response.Body.String(), `"actionId":"action-1"`) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

type apiInteractionSourceStore struct{ source *booksource.BookSource }

func (s apiInteractionSourceStore) GetByID(id string) (*booksource.BookSource, error) {
	if s.source == nil || s.source.ID != id {
		return nil, nil
	}
	copy := *s.source
	return &copy, nil
}

func TestSourceInteractionActionHTTPRejectsStaleRevision(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Fixture",
		LoginUI: `[{"name":"Save","type":"button","action":"save()"}]`, LoginURL: `function save(){}`}
	profiles := &apiInteractionProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{}`)}}
	server := NewServer(nil, nil, nil, nil, nil, analyzer.NewJSVM(), nil, processor.DefaultConfig(), "", nil)
	server.sourceInteractions = sourceinteraction.NewDescriber(apiInteractionSourceStore{source}, profiles, analyzer.NewJSVM())
	request := httptest.NewRequest(http.MethodPost, "/api/sources/source-a/interaction/actions", strings.NewReader(`{"revision":"stale","actionId":"action-0","values":{}}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSourceInteractionResetLoginKeepsSettings(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", LoginUI: `[]`}
	profiles := &apiInteractionProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{"variable":"kept"}`), Authentication: json.RawMessage(`{"loginInfo":{"user":"clear"}}`)}}
	server := NewServer(nil, nil, nil, nil, nil, analyzer.NewJSVM(), nil, processor.DefaultConfig(), "", nil)
	server.sourceInteractions = sourceinteraction.NewDescriber(apiInteractionSourceStore{source}, profiles, analyzer.NewJSVM())
	request := httptest.NewRequest(http.MethodDelete, "/api/sources/source-a/interaction/login", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || string(profiles.profile.Settings) != `{"variable":"kept"}` || string(profiles.profile.Authentication) != `{}` {
		t.Fatalf("status=%d body=%s profile=%+v", response.Code, response.Body.String(), profiles.profile)
	}
}

type apiInteractionProfileStore struct{ profile sourceprofile.Profile }

func (s *apiInteractionProfileStore) Load(context.Context, string) (sourceprofile.Profile, error) {
	return s.profile, nil
}

func (s *apiInteractionProfileStore) SaveSettings(_ context.Context, _ string, value json.RawMessage) error {
	s.profile.Settings = append(json.RawMessage(nil), value...)
	return nil
}

func (s *apiInteractionProfileStore) SaveAuthentication(_ context.Context, _ string, value json.RawMessage) error {
	s.profile.Authentication = append(json.RawMessage(nil), value...)
	return nil
}

func (s *apiInteractionProfileStore) ClearAuthentication(context.Context, string) error {
	s.profile.Authentication = json.RawMessage(`{}`)
	return nil
}

func (s *apiInteractionProfileStore) ResetSettings(context.Context, string) error {
	s.profile.Settings = json.RawMessage(`{}`)
	return nil
}

func (s *apiInteractionProfileStore) Reset(context.Context, string) error {
	s.profile.Settings = json.RawMessage(`{}`)
	s.profile.Authentication = json.RawMessage(`{}`)
	return nil
}

func TestSourceInteractionAwaitBrowserActionReturnsLaunchReference(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", LoginUI: `[{"name":"Register","type":"button","action":"register()"}]`, LoginURL: `function register(){java.startBrowserAwait('https://register.test','Register')}`}
	profiles := &apiInteractionProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{}`)}}
	server := NewServer(nil, nil, nil, nil, nil, analyzer.NewJSVM(), nil, processor.DefaultConfig(), "", nil)
	server.sourceInteractions = sourceinteraction.NewDescriber(apiInteractionSourceStore{source}, profiles, analyzer.NewJSVM())
	server.browserSessions = sourceinteraction.NewBrowserSessions(&apiBrowserFixture{})
	view, err := server.sourceInteractions.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/sources/source-a/interaction/actions", strings.NewReader(`{"revision":"`+view.Revision+`","actionId":"action-0","values":{}}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"browserRequestId"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

type apiBrowserFixture struct{ html string }

func (*apiBrowserFixture) StartInteractive(context.Context, string, string, webview.InteractiveViewport, *sourceexec.SourceSession) (webview.InteractiveFrame, error) {
	return webview.InteractiveFrame{SessionID: "browser"}, nil
}
func (*apiBrowserFixture) InteractiveFrame(context.Context, string) (webview.InteractiveFrame, error) {
	return webview.InteractiveFrame{}, nil
}
func (*apiBrowserFixture) SendInteractiveInput(context.Context, string, webview.InteractiveInput) (webview.InteractiveFrame, error) {
	return webview.InteractiveFrame{}, nil
}
func (b *apiBrowserFixture) CloseInteractive(context.Context, string, string, bool, bool, *sourceexec.SourceSession) (webview.InteractiveCloseResult, error) {
	return webview.InteractiveCloseResult{HTML: b.html}, nil
}
