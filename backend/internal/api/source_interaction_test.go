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
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func TestSourceInteractionHTTPReturnsNormalizedControlsWithoutAuthentication(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Fixture",
		LoginUI: `[{"name":"账号","type":"text"},{"name":"登录","type":"button","action":"login()"}]`}
	server := NewServer(nil, nil, nil, nil, nil, analyzer.NewJSVM(), nil, processor.DefaultConfig(), "", nil)
	server.sourceInteractions = sourceinteraction.NewDescriber(apiInteractionSourceStore{source}, apiInteractionProfileStore{sourceprofile.Profile{
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

type apiInteractionProfileStore struct{ profile sourceprofile.Profile }

func (s apiInteractionProfileStore) Load(context.Context, string) (sourceprofile.Profile, error) {
	return s.profile, nil
}
