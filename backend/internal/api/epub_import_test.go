package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/imageproc"
	"github.com/otwako/novelreader/internal/txt"
)

func TestEPUBBrowserImportReviewAndPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		request := func(method, path, body string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, strings.NewReader(body))
		}
		admitted := request(http.MethodPost, "/api/imports/admission", "")
		requireTXTStatus(t, admitted, http.StatusOK)
		var ticket struct {
			ID     string
			Limits map[string]int64
		}
		if err := json.Unmarshal(admitted.Body.Bytes(), &ticket); err != nil {
			t.Fatal(err)
		}
		if ticket.Limits["txt"] != txt.MaxInputBytes || ticket.Limits["epub"] != epub.MaxInputBytes {
			t.Fatalf("limits: %+v", ticket)
		}
		// The TXT alias exposes the same ticket, not a separate admission allowance.
		legacy := request(http.MethodGet, "/api/imports/txt/admission/"+ticket.ID, "")
		requireTXTStatus(t, legacy, http.StatusOK)
		var old struct {
			ID            string
			MaxInputBytes int64
		}
		if err := json.Unmarshal(legacy.Body.Bytes(), &old); err != nil || old.ID != ticket.ID || old.MaxInputBytes != txt.MaxInputBytes {
			t.Fatalf("legacy: %s %v", legacy.Body.String(), err)
		}
		upload := "/api/imports/epub/uploads/" + ticket.ID + "?filename=Novel.epub&imageMode=optimized"
		source := readingEPUBBytes(t)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodPut, upload, bytes.NewReader(source)), http.StatusNotFound)
		response := request(http.MethodPut, upload, string(source))
		requireTXTStatus(t, response, http.StatusCreated)
		receiptPath := "/api/imports/epub/receipts/" + ticket.ID
		if response.Header().Get("Location") != receiptPath {
			t.Fatal("missing receipt location")
		}
		requireTXTStatus(t, request(http.MethodPut, upload, string(source)), http.StatusNotFound)
		synctest.Wait() // Real shared worker, no polling/sleeps.
		status := request(http.MethodGet, receiptPath, "")
		requireTXTStatus(t, status, http.StatusOK)
		var receipt epubReceiptResponse
		if err := json.Unmarshal(status.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.PreparationState != epubstore.PreparationReady || receipt.Generation != 1 || receipt.ImageMode != epub.OptimizedImages || len(receipt.Notices) != 0 {
			t.Fatalf("receipt: %+v", receipt)
		}
		if status.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private import evidence cacheable")
		}
		requireTXTStatus(t, request(http.MethodGet, "/api/books/"+ticket.ID, ""), http.StatusNotFound)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodGet, receiptPath, nil), http.StatusNotFound)
		previewPath := fmt.Sprintf("%s/preview?generation=%d&limit=1", receiptPath, receipt.Generation)
		preview := request(http.MethodGet, previewPath, "")
		requireTXTStatus(t, preview, http.StatusOK)
		var view epubPreviewResponse
		if err := json.Unmarshal(preview.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if len(view.Headings) != 1 || !view.HasMore || !strings.Contains(view.Sample, "Publication prose") || view.Images.Mode != "optimized" || view.Images.Backend != imageproc.EncoderBackend() || view.Images.DerivativeCount != 1 {
			t.Fatalf("preview: %+v", view)
		}
		if (len(view.Notices) > 0) != (view.Images.Backend == "portable") {
			t.Fatalf("persisted backend notice: %+v", view)
		}
		for _, private := range []string{"private/pic.png", "book.opf", "original.epub", "Reference", "AnchorTargets", "\"path\"", "\"error\""} {
			if strings.Contains(preview.Body.String()+status.Body.String(), private) {
				t.Fatalf("private evidence leaked: %s", private)
			}
		}
		requireTXTStatus(t, request(http.MethodGet, receiptPath+"/preview?generation=2", ""), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodGet, "/api/imports/epub/receipts?limit=101", ""), http.StatusBadRequest)
		listed := request(http.MethodGet, "/api/imports/epub/receipts?limit=1", "")
		requireTXTStatus(t, listed, http.StatusOK)
		if !strings.Contains(listed.Body.String(), ticket.ID) {
			t.Fatal("receipt missing from history")
		}
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/accept", `{"generation":2,"name":"Wrong generation"}`), http.StatusConflict)
		accept := fmt.Sprintf(`{"generation":%d,"name":"Example","author":"Synthetic"}`, receipt.Generation)
		requireTXTStatus(t, txtImportRequest(t, server, sessions, ownershipBob, http.MethodPost, receiptPath+"/accept", strings.NewReader(accept)), http.StatusNotFound)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/accept", accept), http.StatusOK)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/accept", accept), http.StatusOK)
		requireTXTStatus(t, request(http.MethodDelete, receiptPath, ""), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodPost, receiptPath+"/retry", `{"generation":1}`), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodGet, "/api/books/"+ticket.ID+"/chapters", ""), http.StatusOK)
	})
}

func TestEPUBFailedImportRetryAndDiscard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, sessions, _, alice, cleanup := newOwnershipServer(t)
		defer cleanup()
		request := func(method, path, body string) *httptest.ResponseRecorder {
			return txtImportRequest(t, server, sessions, alice, method, path, strings.NewReader(body))
		}
		admitted := request(http.MethodPost, "/api/imports/admission", "")
		requireTXTStatus(t, admitted, http.StatusOK)
		var ticket struct{ ID string }
		if err := json.Unmarshal(admitted.Body.Bytes(), &ticket); err != nil {
			t.Fatal(err)
		}
		upload := "/api/imports/epub/uploads/" + ticket.ID + "?filename=Invalid.epub"
		unread := &unreadTXTBody{}
		requireTXTStatus(t, txtImportRequest(t, server, sessions, alice, http.MethodPut, upload+"&imageMode=unknown", unread), http.StatusBadRequest)
		if unread.read {
			t.Fatal("invalid policy consumed body")
		}
		requireTXTStatus(t, request(http.MethodPut, upload, "not an EPUB"), http.StatusCreated)
		synctest.Wait()
		path := "/api/imports/epub/receipts/" + ticket.ID
		response := request(http.MethodGet, path, "")
		var receipt epubReceiptResponse
		if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.PreparationState != epubstore.PreparationFailed || receipt.ErrorCode != "epub_invalid_publication" || receipt.ImageMode != epub.OriginalImages {
			t.Fatalf("failure: %+v", receipt)
		}
		requireTXTStatus(t, request(http.MethodPost, path+"/retry", `{}`), http.StatusBadRequest)
		requireTXTStatus(t, request(http.MethodPost, path+"/retry", `{"generation":0}`), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodPost, path+"/retry", `{"generation":1}`), http.StatusAccepted)
		synctest.Wait()
		requireTXTStatus(t, request(http.MethodPost, path+"/retry", `{"generation":1}`), http.StatusConflict)
		requireTXTStatus(t, request(http.MethodDelete, path, ""), http.StatusOK)
		requireTXTStatus(t, request(http.MethodDelete, path, ""), http.StatusOK)
		requireTXTStatus(t, request(http.MethodGet, path, ""), http.StatusNotFound)
	})
}

func TestEPUBPortableEncoderNoticeIsNotContentDiagnostic(t *testing.T) {
	response := epubPreviewDTO(epubstore.Review{ImageProcessing: epub.ImageProcessing{Mode: epub.OptimizedImages, Backend: "portable"}})
	if response.NeedsReview || len(response.Diagnostics) != 0 || len(response.Notices) != 1 || response.Notices[0] != "epub_portable_encoder" {
		t.Fatalf("notice: %+v", response)
	}
}
