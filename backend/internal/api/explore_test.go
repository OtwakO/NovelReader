// Explore API tests keep source syntax private while exercising the complete domain boundary.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/processor"
)

func TestExploreAPIRoundTripKeepsRulesPrivate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book/1">API Book</a><span class="author">API Author</span></div>`)
	}))
	defer upstream.Close()
	server, sourceStore, closeDB := newExploreAPIServer(t)
	defer closeDB()
	source := &booksource.BookSource{
		BookSourceURL: upstream.URL, BookSourceName: "Explore API", EnabledExplore: true,
		ExploreURL:  `@js:JSON.stringify([{title:'Mode',type:'select',chars:['A','B'],default:'A',action:'java.refreshExplore()'},{title:String(infoMap['Mode']||'A'),url:'` + upstream.URL + `/explore?page={{page}}'}])`,
		RuleExplore: `{"bookList":".book","name":".name@text","author":".author@text","bookUrl":".name@href"}`,
	}
	if err := sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/explore/sources", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Explore API") {
		t.Fatalf("sources status=%d body=%s", response.Code, response.Body.String())
	}
	response = performAPIRequest(server, http.MethodPost, "/api/explore/catalog", []byte(`{"sourceId":"`+upstream.URL+`"}`))
	var catalog book.ExploreCatalog
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK || catalog.SessionID == "" {
		t.Fatalf("catalog status=%d value=%+v err=%v body=%s", response.Code, catalog, err, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "refreshExplore") || strings.Contains(response.Body.String(), "/explore?page=") || !strings.Contains(response.Body.String(), `"diagnostics":[]`) {
		t.Fatalf("catalog leaked source syntax or omitted diagnostics: %s", response.Body.String())
	}
	controlID := exploreEntryByTitle(t, catalog, "Mode").ID
	response = performAPIRequest(server, http.MethodPost, "/api/explore/control", []byte(fmt.Sprintf(`{"sessionId":%q,"controlId":%q,"value":"B"}`, catalog.SessionID, controlID)))
	var refreshed book.ExploreCatalog
	if err := json.Unmarshal(response.Body.Bytes(), &refreshed); err != nil || response.Code != http.StatusOK || exploreEntryByTitle(t, refreshed, "Mode").Value != "B" {
		t.Fatalf("control status=%d value=%+v err=%v body=%s", response.Code, refreshed, err, response.Body.String())
	}
	categoryID := exploreEntryByTitle(t, refreshed, "B").ID
	response = performAPIRequest(server, http.MethodPost, "/api/explore/page", []byte(fmt.Sprintf(`{"sessionId":%q,"categoryId":%q,"page":1}`, refreshed.SessionID, categoryID)))
	var page book.ExplorePage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || len(page.Books) != 1 || page.Books[0].Name != "API Book" {
		t.Fatalf("page status=%d value=%+v err=%v body=%s", response.Code, page, err, response.Body.String())
	}
	response = performAPIRequest(server, http.MethodPost, "/api/explore/page", []byte(fmt.Sprintf(`{"sessionId":%q,"categoryId":%q,"page":2}`, refreshed.SessionID, categoryID)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"books":[]`) || !strings.Contains(response.Body.String(), `"exhausted":true`) {
		t.Fatalf("exhausted page status=%d body=%s", response.Code, response.Body.String())
	}

	source.ExploreURL = `Browser::` + upstream.URL + `/browser,{"webView":true}`
	if err := sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	response = performAPIRequest(server, http.MethodPost, "/api/explore/catalog", []byte(`{"sourceId":"`+upstream.URL+`"}`))
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK {
		t.Fatalf("webview catalog status=%d err=%v body=%s", response.Code, err, response.Body.String())
	}
	response = performAPIRequest(server, http.MethodPost, "/api/explore/page", []byte(fmt.Sprintf(`{"sessionId":%q,"categoryId":%q,"page":1}`, catalog.SessionID, catalog.Entries[0].ID)))
	if response.Code != http.StatusNotImplemented || !strings.Contains(response.Body.String(), `"code":"unsupported_capability"`) || strings.Contains(response.Body.String(), "sourceexec") {
		t.Fatalf("webview status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestExploreAPIRejectsInvalidBodiesAndMethods(t *testing.T) {
	server, _, closeDB := newExploreAPIServer(t)
	defer closeDB()
	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/api/explore/catalog", "", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/explore/catalog", `{}`, http.StatusBadRequest},
		{http.MethodPost, "/api/explore/catalog", `{"sourceId":"x"}{}`, http.StatusBadRequest},
		{http.MethodPost, "/api/explore/control", `{"sessionId":"","controlId":"x"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/explore/page", `{"sessionId":"x","categoryId":"y","page":0}`, http.StatusBadRequest},
		{http.MethodPost, "/api/explore/catalog", `{"sourceId":"` + strings.Repeat("x", 33*1024) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		response := performAPIRequest(server, test.method, test.path, []byte(test.body))
		if response.Code != test.status {
			t.Fatalf("%s %s status=%d want=%d body=%s", test.method, test.path, response.Code, test.status, response.Body.String())
		}
	}
}

func TestExploreAPIErrorMappingHidesCauses(t *testing.T) {
	for _, test := range []struct {
		code   string
		status int
	}{
		{"source_unavailable", http.StatusNotFound},
		{"page_conflict", http.StatusConflict},
		{"invalid_control_value", http.StatusUnprocessableEntity},
		{"unsupported_control_type", http.StatusNotImplemented},
		{"result_capacity_exceeded", http.StatusInsufficientStorage},
		{"session_create_failed", http.StatusServiceUnavailable},
		{"category_cancelled", http.StatusGatewayTimeout},
		{"transport_failed", http.StatusBadGateway},
		{"source_list_failed", http.StatusInternalServerError},
	} {
		recorder := httptest.NewRecorder()
		writeExploreError(recorder, &book.ExploreError{Code: test.code, Stage: "fixture", Message: "Safe", Retryable: true, ExpectedPage: 3})
		if recorder.Code != test.status || strings.Contains(recorder.Body.String(), "expectedPage") || !strings.Contains(recorder.Body.String(), `"nextPage":3`) {
			t.Fatalf("code=%s status=%d body=%s", test.code, recorder.Code, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	writeExploreError(recorder, errors.New("RAW_SECRET_ACTION"))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "RAW_SECRET_ACTION") {
		t.Fatalf("unknown error leaked: %s", recorder.Body.String())
	}
}

func newExploreAPIServer(t *testing.T) (*Server, *booksource.Store, func()) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/explore.db")
	if err != nil {
		t.Fatal(err)
	}
	sourceStore := booksource.NewStore(db)
	initializeAPITestSchema(t, db, booksource.ReaderSchema())
	client := fetcher.NewInsecure(time.Second)
	jsVM := analyzer.NewJSVM()
	searcher := book.NewSearcher(client, jsVM, nil, sourceStore, nil)
	server := NewServer(sourceStore, nil, searcher, nil, client, jsVM, nil, processor.Config{}, t.TempDir(), db)
	return server, sourceStore, func() { _ = db.Close() }
}

func exploreEntryByTitle(t *testing.T, catalog book.ExploreCatalog, title string) book.ExploreEntry {
	t.Helper()
	for _, entry := range catalog.Entries {
		if entry.Title == title {
			return entry
		}
	}
	t.Fatalf("entry %q missing from %+v", title, catalog.Entries)
	return book.ExploreEntry{}
}
