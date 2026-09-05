package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/processor"
	_ "modernc.org/sqlite"
)

func TestSourceCollectionUploadRenameReplaceAndDelete(t *testing.T) {
	server := newCollectionAPIServer(t)
	created := performCollectionUpload(t, server, http.MethodPost, "/api/source-collections/upload", "Main Sources", "sources-a.json", `[
		{"bookSourceUrl":"https://one","bookSourceName":"One","enabled":true},
		{"bookSourceUrl":"https://two","bookSourceName":"Two","enabled":true}
	]`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", created.Code, created.Body.String())
	}
	var creation collectionMutationResponse
	decodeCollectionResponse(t, created, &creation)
	if creation.Collection.Name != "Main Sources" || creation.Changes.Added != 2 {
		t.Fatalf("unexpected creation response: %#v", creation)
	}
	listed := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	listedResponse := httptest.NewRecorder()
	server.ServeHTTP(listedResponse, listed)
	if listedResponse.Code != http.StatusOK || !strings.Contains(listedResponse.Body.String(), `"collectionId":"`+creation.Collection.ID+`"`) {
		t.Fatalf("source management response lost collection ownership: %d %s", listedResponse.Code, listedResponse.Body.String())
	}

	rename := httptest.NewRequest(http.MethodPatch, "/api/source-collections/"+creation.Collection.ID, strings.NewReader(`{"name":"Renamed"}`))
	rename.Header.Set("Content-Type", "application/json")
	renameResponse := httptest.NewRecorder()
	server.ServeHTTP(renameResponse, rename)
	if renameResponse.Code != http.StatusOK || !strings.Contains(renameResponse.Body.String(), `"name":"Renamed"`) {
		t.Fatalf("rename failed: %d %s", renameResponse.Code, renameResponse.Body.String())
	}

	replaced := performCollectionUpload(t, server, http.MethodPost, "/api/source-collections/"+creation.Collection.ID+"/replace", "", "sources-b.json", `[
		{"bookSourceUrl":"https://one","bookSourceName":"One updated","enabled":false},
		{"bookSourceUrl":"https://three","bookSourceName":"Three","enabled":true}
	]`)
	if replaced.Code != http.StatusOK {
		t.Fatalf("replace status %d: %s", replaced.Code, replaced.Body.String())
	}
	var replacement collectionMutationResponse
	decodeCollectionResponse(t, replaced, &replacement)
	if replacement.Changes.Added != 1 || replacement.Changes.Removed != 1 || replacement.Changes.Updated != 1 {
		t.Fatalf("unexpected replacement: %#v", replacement.Changes)
	}

	deleted := httptest.NewRequest(http.MethodDelete, "/api/source-collections/"+creation.Collection.ID, nil)
	deleteResponse := httptest.NewRecorder()
	server.ServeHTTP(deleteResponse, deleted)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	sources, err := server.sourceStore.List()
	if err != nil || len(sources) != 0 {
		t.Fatalf("collection delete left sources: %#v, %v", sources, err)
	}
}

func newCollectionAPIServer(t *testing.T) *Server {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	initializeAPITestSchema(t, db, booksource.ReaderSchema())
	return NewServer(booksource.NewStore(db), nil, nil, nil, nil, nil, nil, processor.Config{}, "", db)
}

func performCollectionUpload(t *testing.T, server *Server, method, path, name, filename, document string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if name != "" {
		if err := writer.WriteField("name", name); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, document); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func decodeCollectionResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCollectionAvailabilityPatchPreservesMemberSettings(t *testing.T) {
	server := newCollectionAPIServer(t)
	created := performCollectionUpload(t, server, http.MethodPost, "/api/source-collections/upload", "Temporary", "sources.json", `[
		{"bookSourceUrl":"https://off","bookSourceName":"Off","enabled":false,"enabledExplore":true,"exploreUrl":"Books::/books"},
		{"bookSourceUrl":"https://on","bookSourceName":"On","enabled":true,"enabledExplore":true,"exploreUrl":"Books::/books"}
	]`)
	var creation collectionMutationResponse
	decodeCollectionResponse(t, created, &creation)

	request := httptest.NewRequest(http.MethodPatch, "/api/source-collections/"+creation.Collection.ID, strings.NewReader(`{"enabled":false}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", response.Code, response.Body.String())
	}
	var collection booksource.Collection
	decodeCollectionResponse(t, response, &collection)
	if collection.Enabled {
		t.Fatalf("collection=%#v", collection)
	}
	members, err := server.sourceStore.ListByCollection(t.Context(), collection.ID)
	if err != nil || len(members) != 2 || members[0].Enabled || !members[1].Enabled {
		t.Fatalf("members=%#v err=%v", members, err)
	}
}
