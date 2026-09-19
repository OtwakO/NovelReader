package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/txtstore"
)

func TestTXTAnalysisFailureResponseIsAllowlisted(t *testing.T) {
	for _, tc := range []struct{ stored, want string }{
		{"txt_encoding_required", "txt_encoding_required"},
		{"txt_invalid_encoding", "txt_invalid_encoding"},
		{"txt_unsupported_encoding", "txt_unsupported_encoding"},
		{"txt_no_readable_text", "txt_no_readable_text"},
		{"txt_non_text", "txt_non_text"},
		{"txt_section_limit", "txt_section_limit"},
		{"txt_storage_error", "txt_storage_error"},
		{"open /private/reader/original.txt: denied", "txt_analysis_failed"},
		{"txt_future_code", "txt_analysis_failed"},
		{"", ""},
	} {
		t.Run(tc.stored, func(t *testing.T) {
			dto := txtReceiptDTO(txtstore.Receipt{State: txtstore.AnalysisFailed, Error: tc.stored})
			raw, err := json.Marshal(dto)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err = json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			code, _ := fields["errorCode"].(string)
			if code != tc.want || dto.HasError != (tc.stored != "") {
				t.Fatalf("response=%s want=%q", raw, tc.want)
			}
			if strings.Contains(string(raw), "/private/") {
				t.Fatal("raw error leaked")
			}
		})
	}
}

func TestTXTReparseAnalysisFailureResponse(t *testing.T) {
	f := newTXTReadingFixture(t)
	resource := "/api/books/" + f.item.ID + "/txt/reparse"
	var status txtReparseResponse
	response := f.request("GET", resource, "")
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil || response.Code != 200 {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	response = f.request("POST", resource, fmt.Sprintf(`{"contentRevision":%d,"generation":0,"encoding":"utf-16le"}`, status.ContentRevision))
	if response.Code != 202 {
		t.Fatalf("prepare: %d %s", response.Code, response.Body.String())
	}
	var queued struct{ Generation int64 }
	if err := json.Unmarshal(response.Body.Bytes(), &queued); err != nil {
		t.Fatal(err)
	}
	if worked, err := f.store.AnalyzePending(t.Context(), txtstore.PendingAnalysis{ReceiptID: f.item.ID, Generation: queued.Generation}); !worked || err == nil {
		t.Fatalf("expected encoding failure: %v %v", worked, err)
	}
	response = f.request("GET", resource, "")
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil || response.Code != 200 {
		t.Fatalf("failed status: %d %s", response.Code, response.Body.String())
	}
	if status.Candidate == nil || !status.Candidate.HasError || status.Candidate.ErrorCode != "txt_invalid_encoding" {
		t.Fatalf("failure response: %s", response.Body.String())
	}
}
