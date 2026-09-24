package api

import (
	"encoding/json"
	"fmt"
	"github.com/otwako/novelreader/internal/txtstore"
	"strings"
	"testing"
)

func TestTXTReparseHTTPReviewApplyAndDiscard(t *testing.T) {
	f := newTXTReadingFixture(t)
	base := "/api/books/" + f.item.ID
	resource := base + "/txt/reparse"
	response := f.request("GET", resource, "")
	var status txtReparseResponse
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil || response.Code != 200 || status.Candidate != nil || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status: %d %s: %v", response.Code, response.Body.String(), err)
	}
	request := fmt.Sprintf(`{"contentRevision":%d,"generation":0,"preset":"generated-sections"}`, status.ContentRevision)
	response = f.request("POST", resource, request)
	var queued struct{ Generation int64 }
	if err := json.Unmarshal(response.Body.Bytes(), &queued); err != nil || response.Code != 202 || queued.Generation <= status.ActiveGeneration {
		t.Fatalf("prepare: %d %s: %v", response.Code, response.Body.String(), err)
	}
	if response = f.request("POST", resource, request); response.Code != 409 {
		t.Fatalf("stale absence: %d %s", response.Code, response.Body.String())
	}
	if response = f.request("DELETE", "/api/imports/txt/receipts/"+f.item.ID, ""); response.Code != 409 {
		t.Fatalf("pending discard crossed publication boundary: %d", response.Code)
	}
	if worked, err := f.store.AnalyzePending(t.Context(), txtstore.PendingAnalysis{ReceiptID: f.item.ID, Generation: queued.Generation}); !worked || err != nil {
		t.Fatalf("analysis: %v %v", worked, err)
	}
	response = f.request("GET", fmt.Sprintf("%s/preview?generation=%d&limit=1", resource, queued.Generation), "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Literal") || strings.Contains(response.Body.String(), "startByte") {
		t.Fatalf("preview: %d %s", response.Code, response.Body.String())
	}
	sectionPath := fmt.Sprintf("%s/sections/0?generation=%d", resource, queued.Generation)
	if response = f.request("GET", sectionPath, ""); response.Code != 200 || !strings.Contains(response.Body.String(), "Literal") {
		t.Fatalf("candidate section: %d %s", response.Code, response.Body.String())
	}
	response = f.request("GET", fmt.Sprintf("%s/impact?generation=%d", resource, queued.Generation), "")
	var impact struct {
		ContentRevision, StateVersion int64
		Resume                        any
		TotalSections                 int
	}
	if err := json.Unmarshal(response.Body.Bytes(), &impact); err != nil || response.Code != 200 || impact.Resume != nil || impact.TotalSections != 1 {
		t.Fatalf("impact: %d %s %v", response.Code, response.Body.String(), err)
	}
	apply := fmt.Sprintf(`{"generation":%d,"activeGeneration":%d,"contentRevision":%d,"stateVersion":%d}`, queued.Generation, status.ActiveGeneration, impact.ContentRevision, impact.StateVersion)
	if response = f.request("POST", resource+"/apply", apply); response.Code != 409 || !strings.Contains(response.Body.String(), "txt_resume_required") {
		t.Fatalf("missing resume: %d %s", response.Code, response.Body.String())
	}
	response = f.request("PUT", base+"/progress", fmt.Sprintf(`{"contentRevision":%d,"stateVersion":%d,"chapterIndex":1,"position":0.4}`, impact.ContentRevision, impact.StateVersion))
	if response.Code != 200 {
		t.Fatalf("progress: %d %s", response.Code, response.Body.String())
	}
	if response = f.request("POST", resource+"/apply", apply); response.Code != 409 || !strings.Contains(response.Body.String(), "state_changed") {
		t.Fatalf("stale impact: %d %s", response.Code, response.Body.String())
	}
	response = f.request("GET", fmt.Sprintf("%s/impact?generation=%d", resource, queued.Generation), "")
	if err := json.Unmarshal(response.Body.Bytes(), &impact); err != nil || response.Code != 200 {
		t.Fatalf("refresh impact: %d %s", response.Code, response.Body.String())
	}
	apply = fmt.Sprintf(`{"generation":%d,"activeGeneration":%d,"contentRevision":%d,"stateVersion":%d,"resumeChapter":0}`, queued.Generation, status.ActiveGeneration, impact.ContentRevision, impact.StateVersion)
	response = f.request("POST", resource+"/apply", apply)
	if response.Code != 200 {
		t.Fatalf("apply: %d %s", response.Code, response.Body.String())
	}
	if response = f.request("POST", resource+"/apply", apply); response.Code != 200 || !strings.Contains(response.Body.String(), `"alreadyApplied":true`) {
		t.Fatalf("retry: %d %s", response.Code, response.Body.String())
	}
	if response = f.request("GET", fmt.Sprintf("%s/chapters/0/content?contentRevision=%d", base, status.ContentRevision), ""); response.Code != 409 {
		t.Fatalf("stale reader: %d %s", response.Code, response.Body.String())
	}
	response = f.request("GET", resource, "")
	status = txtReparseResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil || status.Candidate != nil || status.ActiveGeneration != queued.Generation {
		t.Fatalf("applied status: %+v %v", status, err)
	}
	if response = f.request("GET", sectionPath, ""); response.Code != 409 {
		t.Fatalf("applied candidate still previewable: %d", response.Code)
	}
	// Discarding later preparation cannot remove active text or the managed original.
	request = fmt.Sprintf(`{"contentRevision":%d,"generation":0}`, status.ContentRevision)
	response = f.request("POST", resource, request)
	if err := json.Unmarshal(response.Body.Bytes(), &queued); err != nil || response.Code != 202 {
		t.Fatalf("prepare again: %d %s", response.Code, response.Body.String())
	}
	if worked, err := f.store.AnalyzePending(t.Context(), txtstore.PendingAnalysis{ReceiptID: f.item.ID, Generation: queued.Generation}); !worked || err != nil {
		t.Fatalf("second analysis: %v %v", worked, err)
	}
	sectionPath = fmt.Sprintf("%s/sections/0?generation=%d", resource, queued.Generation)
	if response = f.request("GET", sectionPath, ""); response.Code != 200 {
		t.Fatalf("second candidate section: %d", response.Code)
	}
	response = f.request("DELETE", resource, fmt.Sprintf(`{"contentRevision":%d,"generation":%d}`, status.ContentRevision, queued.Generation))
	if response.Code != 200 {
		t.Fatalf("discard: %d %s", response.Code, response.Body.String())
	}
	if response = f.request("GET", sectionPath, ""); response.Code != 409 {
		t.Fatalf("discarded candidate still previewable: %d", response.Code)
	}
	if response = f.request("GET", fmt.Sprintf("%s/chapters/0/content?contentRevision=%d", base, status.ContentRevision), ""); response.Code != 200 {
		t.Fatalf("active damaged: %d %s", response.Code, response.Body.String())
	}
}
