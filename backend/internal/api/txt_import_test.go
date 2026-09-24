package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

func txtImportRequest(t *testing.T, server *Server, sessions *auth.SessionService, user readerstore.UserID, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	session, err := sessions.Create(t.Context(), user, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, body)
	r.Header.Set("Content-Type", "application/json")
	if method == http.MethodPut {
		r.Header.Set("Content-Type", "application/octet-stream")
	}
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, r)
	return response
}

func requireTXTStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
	}
}

func TestTXTBrowserUploadReviewAndPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		request := func(method, path, body string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, strings.NewReader(body))
		}
		admitted := request(http.MethodPost, "/api/imports/txt/admission", "")
		requireTXTStatus(t, admitted, http.StatusOK)
		var ticket struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(admitted.Body.Bytes(), &ticket); err != nil {
			t.Fatal(err)
		}
		uploadPath := "/api/imports/txt/uploads/" + ticket.ID + "?filename=example.txt"
		foreign := txtImportRequest(t, server, sessions, ownershipBob, http.MethodPut, uploadPath, strings.NewReader("other reader"))
		requireTXTStatus(t, foreign, http.StatusNotFound)
		source := "Chapter 1\nFirst paragraph with enough readable text.\nChapter 2\nSecond paragraph with enough readable text.\n"
		uploaded := request(http.MethodPut, uploadPath, source)
		requireTXTStatus(t, uploaded, http.StatusCreated)
		if uploaded.Header().Get("Location") != "/api/imports/txt/receipts/"+ticket.ID {
			t.Fatal("receipt identity lost")
		}
		requireTXTStatus(t, request(http.MethodPut, uploadPath, source), http.StatusNotFound)
		synctest.Wait() // The actual production worker completes without polling.
		receiptPath := "/api/imports/txt/receipts/" + ticket.ID
		readReceipt := func() txtReceiptResponse {
			response := request(http.MethodGet, receiptPath, "")
			requireTXTStatus(t, response, http.StatusOK)
			var value txtReceiptResponse
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(response.Body.String(), "\"path\"") || strings.Contains(response.Body.String(), "\"error\"") {
				t.Fatal("native storage details exposed")
			}
			return value
		}
		value := readReceipt()
		if value.ID != ticket.ID || value.AnalysisVersion < 1 || (value.State != txtstore.Ready && value.State != txtstore.NeedsReview) {
			t.Fatalf("worker did not finish: %+v", value)
		}
		requireTXTStatus(t, request(http.MethodGet, "/api/books/"+ticket.ID, ""), http.StatusNotFound)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodGet, receiptPath, nil), http.StatusNotFound)
		preview := request(http.MethodGet, fmt.Sprintf("%s/preview?analysisVersion=%d&limit=1", receiptPath, value.AnalysisVersion), "")
		requireTXTStatus(t, preview, http.StatusOK)
		if !strings.Contains(preview.Body.String(), "First paragraph") || strings.Contains(preview.Body.String(), "start_byte") {
			t.Fatalf("preview: %s", preview.Body.String())
		}
		requireTXTStatus(t, request(http.MethodGet, "/api/imports/txt/receipts?limit=101", ""), http.StatusBadRequest)
		requireTXTStatus(t, request(http.MethodGet, "/api/imports/txt/receipts?limit=1&state="+string(value.State), ""), http.StatusOK)
		invalid := fmt.Sprintf(`{"analysisVersion":%d,"encoding":"invalid"}`, value.AnalysisVersion)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/analysis", invalid), http.StatusBadRequest)
		oldVersion := value.AnalysisVersion
		badPattern := request(http.MethodPost, receiptPath+"/analysis", fmt.Sprintf(`{"analysisVersion":%d,"preset":"custom","pattern":"(?=Chapter)Chapter"}`, oldVersion))
		requireTXTStatus(t, badPattern, http.StatusBadRequest)
		if !strings.Contains(badPattern.Body.String(), "txt_invalid_pattern") || readReceipt().AnalysisVersion != oldVersion {
			t.Fatal("invalid pattern changed the prepared interpretation")
		}
		requireTXTStatus(t, request(http.MethodGet, fmt.Sprintf("%s/preview?analysisVersion=%d", receiptPath, oldVersion), ""), http.StatusOK)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/analysis", fmt.Sprintf(`{"analysisVersion":%d,"preset":"custom","pattern":"(?i)chapter ([0-9]+)"}`, oldVersion)), http.StatusAccepted)
		synctest.Wait()
		value = readReceipt()
		if value.Preset != txt.CustomPattern || value.Pattern != `(?i)chapter ([0-9]+)` {
			t.Fatalf("custom request lost: %+v", value)
		}
		customPreview := request(http.MethodGet, fmt.Sprintf("%s/preview?analysisVersion=%d", receiptPath, value.AnalysisVersion), "")
		requireTXTStatus(t, customPreview, http.StatusOK)
		if !strings.Contains(customPreview.Body.String(), `"preset":"custom"`) || !strings.Contains(customPreview.Body.String(), `"title":"Chapter 1"`) {
			t.Fatalf("custom interpretation not used: %s", customPreview.Body.String())
		}
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/accept", fmt.Sprintf(`{"analysisVersion":%d,"name":"Example"}`, oldVersion)), http.StatusConflict)
		accept := fmt.Sprintf(`{"analysisVersion":%d,"name":"Example","author":"Synthetic"}`, value.AnalysisVersion)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/accept", accept), http.StatusOK)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/accept", accept), http.StatusOK)
		requireTXTStatus(t, request(http.MethodDelete, receiptPath, ""), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/analysis", fmt.Sprintf(`{"analysisVersion":%d,"preset":"custom","pattern":"other"}`, value.AnalysisVersion)), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodGet, "/api/books/"+ticket.ID+"/chapters", ""), http.StatusOK)
	})
}

type unreadTXTBody struct{ read bool }

func (b *unreadTXTBody) Read([]byte) (int, error) { b.read = true; return 0, io.ErrUnexpectedEOF }

func TestTXTUploadRejectsBeforeConsumptionAndRetainsFailedReceipt(t *testing.T) {
	server, sessions, _, alice, cleanup := newOwnershipServer(t)
	defer cleanup()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/imports/txt/admission", nil))
	requireTXTStatus(t, response, http.StatusUnauthorized)
	body := &unreadTXTBody{}
	response = txtImportRequest(t, server, sessions, alice, http.MethodPut, "/api/imports/txt/uploads/missing?filename=example.txt", body)
	requireTXTStatus(t, response, http.StatusNotFound)
	if body.read {
		t.Fatal("unadmitted input consumed")
	}
	ticket, err := server.fileAdmission.Request(alice)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/imports/txt/uploads/" + ticket.ID + "?filename=example.txt"
	session, err := sessions.Create(t.Context(), alice, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	oversize := httptest.NewRequest(http.MethodPut, path, body)
	oversize.Header.Set("Content-Type", "application/octet-stream")
	oversize.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.Token})
	oversize.ContentLength = txt.MaxInputBytes + 1
	response = httptest.NewRecorder()
	server.ServeHTTP(response, oversize)
	requireTXTStatus(t, response, http.StatusRequestEntityTooLarge)
	if body.read {
		t.Fatal("oversized input consumed")
	}
	response = txtImportRequest(t, server, sessions, alice, http.MethodPut, path, body)
	requireTXTStatus(t, response, http.StatusBadRequest)
	receiptPath := "/api/imports/txt/receipts/" + ticket.ID
	response = txtImportRequest(t, server, sessions, alice, http.MethodGet, receiptPath, nil)
	requireTXTStatus(t, response, http.StatusOK)
	var value txtReceiptResponse
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.State != txtstore.Failed || !value.HasError {
		t.Fatalf("lost failed acquisition: %+v", value)
	}
	for range 2 {
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodDelete, receiptPath, nil), http.StatusOK)
	}
	requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodGet, receiptPath, nil), http.StatusNotFound)
}
