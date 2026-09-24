package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
)

func TestEPUBImportSelectedSectionPreview(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		request := func(method, path, body string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, strings.NewReader(body))
		}
		admitted := request("POST", "/api/imports/admission", "")
		requireTXTStatus(t, admitted, http.StatusOK)
		var ticket struct{ ID string }
		if err := json.Unmarshal(admitted.Body.Bytes(), &ticket); err != nil {
			t.Fatal(err)
		}
		source := readingEPUBBytes(t,
			[2]string{"main.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><head><title></title></head><body><img id="start" src="private/pic.png"/></body></html>`},
			[2]string{"notes.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><head><title></title></head><body><h1 id="note">Body heading</h1><p>Preview prose <a href="main.xhtml#start">Cover</a></p></body></html>`},
		)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, "PUT", "/api/imports/epub/uploads/"+ticket.ID+"?filename=Preview.epub", bytes.NewReader(source)), http.StatusCreated)
		synctest.Wait()
		base := "/api/imports/epub/receipts/" + ticket.ID
		catalog := request("GET", base+"/navigation?generation=1", "")
		requireTXTStatus(t, catalog, http.StatusOK)
		if !strings.Contains(catalog.Body.String(), "Authored start") || !strings.Contains(catalog.Body.String(), `"anchor"`) || strings.Contains(catalog.Body.String(), "contentRevision") {
			t.Fatal(catalog.Body.String())
		}
		cover := request("GET", base+"/sections/0?generation=1", "")
		requireTXTStatus(t, cover, http.StatusOK)
		// Walk the wire document rather than depending on the normalizer's grouping.
		var wire map[string]any
		if err := json.Unmarshal(cover.Body.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}
		var imageHref string
		var visit func(any)
		visit = func(v any) {
			switch node := v.(type) {
			case map[string]any:
				if resource, ok := node["resource"].(map[string]any); ok {
					imageHref, _ = resource["href"].(string)
				}
				for _, child := range node {
					visit(child)
				}
			case []any:
				for _, child := range node {
					visit(child)
				}
			}
		}
		visit(wire)
		if wire["generation"] != float64(1) || imageHref == "" {
			t.Fatal(cover.Body.String())
		}
		prose := request("GET", base+"/sections/1?generation=1", "")
		requireTXTStatus(t, prose, http.StatusOK)
		if !strings.Contains(prose.Body.String(), "Preview prose") || !strings.Contains(prose.Body.String(), `"unavailable":true`) || strings.Contains(prose.Body.String(), "contentRevision") {
			t.Fatal(prose.Body.String())
		}
		for _, response := range []*httptest.ResponseRecorder{catalog, cover, prose} {
			if strings.Contains(response.Body.String(), "private/pic.png") || strings.Contains(response.Body.String(), "main.xhtml") {
				t.Fatal("private paths in preview")
			}
		}
		image := request("GET", imageHref, "")
		requireTXTStatus(t, image, http.StatusOK)
		if image.Header().Get("Content-Type") != "image/png" || image.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(image.Header())
		}
		requireTXTStatus(t, request("GET", "/api/books/"+ticket.ID, ""), http.StatusNotFound)
		for _, path := range []string{base + "/navigation?generation=1", base + "/sections/0?generation=1", imageHref} {
			anonymous := httptest.NewRecorder()
			server.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, path, nil))
			requireTXTStatus(t, anonymous, http.StatusUnauthorized)
			requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, "GET", path, nil), http.StatusNotFound)
			requireTXTStatus(t, request("GET", strings.Replace(path, "generation=1", "generation=2", 1), ""), http.StatusConflict)
		}
		requireTXTStatus(t, request("DELETE", base, ""), http.StatusOK)
		for _, path := range []string{base + "/navigation?generation=1", base + "/sections/0?generation=1", imageHref} {
			requireTXTStatus(t, request("GET", path, ""), http.StatusNotFound)
		}
	})
}
