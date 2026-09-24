package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestImportHistory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		request := func(method, path, body string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, strings.NewReader(body))
		}
		upload := func(format string, content []byte) string {
			t.Helper()
			admitted := request("POST", "/api/imports/admission", "")
			requireTXTStatus(t, admitted, http.StatusOK)
			var ticket struct{ ID string }
			if err := json.Unmarshal(admitted.Body.Bytes(), &ticket); err != nil {
				t.Fatal(err)
			}
			uploaded := txtImportRequest(t, server, sessions, alice, "PUT", "/api/imports/"+format+"/uploads/"+ticket.ID+"?filename=Example."+format, bytes.NewReader(content))
			requireTXTStatus(t, uploaded, http.StatusCreated)
			synctest.Wait()
			return ticket.ID
		}
		txtReview := upload("txt", []byte("A readable paragraph without chapter headings.\n"))
		epubReady := upload("epub", readingEPUBBytes(t))
		epubReview := upload("epub", readingEPUBBytes(t, [2]string{"main.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Readable prose</p><iframe src="https://example.invalid/"/></body></html>`}))
		time.Sleep(time.Second)
		failed := upload("epub", []byte("not an EPUB archive"))
		type item struct {
			Format  string
			Status  string
			Receipt struct {
				ID        string
				CreatedAt int64
			}
		}
		type page struct {
			Items      []item
			NextCursor string
		}
		list := func(query string) page {
			t.Helper()
			response := request("GET", "/api/imports/receipts?"+query, "")
			requireTXTStatus(t, response, http.StatusOK)
			var result page
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(response.Body.String(), "metadata_json") || strings.Contains(response.Body.String(), "private/pic.png") {
				t.Fatal("private preparation data in history")
			}
			return result
		}
		var ids []string
		cursor := ""
		for range 5 {
			result := list("limit=1&before=" + url.QueryEscape(cursor))
			for _, value := range result.Items {
				ids = append(ids, value.Receipt.ID)
			}
			cursor = result.NextCursor
			if cursor == "" {
				break
			}
		}
		epubIDs := []string{epubReady, epubReview}
		slices.Sort(epubIDs)
		slices.Reverse(epubIDs)
		want := append([]string{failed}, append(epubIDs, txtReview)...)
		if !slices.Equal(ids, want) || cursor != "" {
			t.Fatalf("history order: %v; want %v", ids, want)
		}
		checks := []struct {
			query string
			ids   []string
		}{
			{"status=needs_review", []string{epubReview, txtReview}},
			{"status=ready", []string{epubReady}},
			{"format=epub&status=needs_review", []string{epubReview}},
			{"format=txt", []string{txtReview}},
			{"status=failed", []string{failed}},
		}
		for _, check := range checks {
			got := []string{}
			for _, value := range list(check.query).Items {
				got = append(got, value.Receipt.ID)
			}
			if !slices.Equal(got, check.ids) {
				t.Fatalf("%s: %v; want %v", check.query, got, check.ids)
			}
		}
		requireTXTStatus(t, request("POST", "/api/imports/epub/receipts/"+epubReady+"/accept", `{"generation":1,"name":"Example","author":""}`), http.StatusOK)
		added := list("status=added")
		if len(added.Items) != 1 || added.Items[0].Receipt.ID != epubReady || len(list("status=ready").Items) != 0 {
			t.Fatal("published receipt classified as pending")
		}
		if list("limit=1").Items[0].Receipt.ID != failed {
			t.Fatal("publication reordered history")
		}
		foreign := txtImportRequest(t, server, sessions, ownershipBob, "GET", "/api/imports/receipts", nil)
		requireTXTStatus(t, foreign, http.StatusOK)
		var empty page
		if err := json.Unmarshal(foreign.Body.Bytes(), &empty); err != nil || len(empty.Items) != 0 {
			t.Fatal("cross-reader history")
		}
		anonymous := httptest.NewRecorder()
		server.ServeHTTP(anonymous, httptest.NewRequest("GET", "/api/imports/receipts", nil))
		requireTXTStatus(t, anonymous, http.StatusUnauthorized)
		for _, query := range []string{"before=broken", "format=pdf", "status=unknown"} {
			requireTXTStatus(t, request("GET", "/api/imports/receipts?"+query, ""), http.StatusBadRequest)
		}
	})
}
