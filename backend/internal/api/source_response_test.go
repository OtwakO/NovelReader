package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/processor"
)

func TestSourceManagementListUsesCompactSummariesAndDetailPreservesDefinition(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "sources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initializeAPITestSchema(t, db, booksource.ReaderSchema())
	store := booksource.NewStore(db)
	source, err := booksource.NewFromJSON([]byte(`{
		"bookSourceUrl":"https://source.test","bookSourceName":"Fixture","bookSourceGroup":"Group",
		"enabled":true,"enabledExplore":true,"searchUrl":"/search","exploreUrl":"Books::/books",
		"ruleSearch":{"bookList":".book"},"ruleContent":"large private rule","futureField":{"kept":true}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportBatch([]*booksource.BookSource{source}); err != nil {
		t.Fatal(err)
	}

	server := NewServer(store, nil, nil, nil, nil, nil, nil, processor.DefaultConfig(), "", db)
	listResponse := httptest.NewRecorder()
	server.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/sources", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	if strings.Contains(listResponse.Body.String(), "ruleContent") || strings.Contains(listResponse.Body.String(), "futureField") {
		t.Fatalf("list exposed full definition: %s", listResponse.Body.String())
	}
	var summaries []sourceManagementSummary
	if err := json.Unmarshal(listResponse.Body.Bytes(), &summaries); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || !summaries[0].Searchable || summaries[0].SourceID == "" {
		t.Fatalf("summaries=%+v", summaries)
	}

	detailResponse := httptest.NewRecorder()
	server.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/api/sources/"+summaries[0].SourceID, nil))
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	if !strings.Contains(detailResponse.Body.String(), `"ruleContent":"large private rule"`) || !strings.Contains(detailResponse.Body.String(), `"futureField":{"kept":true}`) {
		t.Fatalf("detail lost definition: %s", detailResponse.Body.String())
	}

	patchResponse := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/sources/"+summaries[0].SourceID, strings.NewReader(`{"enabled":false}`))
	server.ServeHTTP(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patchResponse.Code, patchResponse.Body.String())
	}
	stored, err := store.GetByID(summaries[0].SourceID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := stored.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Enabled || !strings.Contains(string(encoded), `"ruleContent":"large private rule"`) || !strings.Contains(string(encoded), `"futureField":{"kept":true}`) {
		t.Fatalf("patched definition=%s", encoded)
	}
}
