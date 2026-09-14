package api

import (
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
	"net/http"
)

// Initial import and reparse share a bounded preview, never native file offsets.
// analysisVersion is the existing wire name for an interpretation generation.
func writeTXTPreview(w http.ResponseWriter, value txtstore.Review) {
	type heading struct {
		Index     int    `json:"index"`
		Title     string `json:"title"`
		Generated bool   `json:"generated"`
	}
	headings := make([]heading, 0, len(value.Headings))
	for _, item := range value.Headings {
		headings = append(headings, heading{item.Index, item.Title, item.Generated})
	}
	writeJSON(w, http.StatusOK, struct {
		AnalysisVersion int64              `json:"analysisVersion"`
		Encoding        txt.Encoding       `json:"encoding"`
		Preset          txt.Preset         `json:"preset"`
		ParserVersion   int                `json:"parserVersion"`
		ReviewReasons   []txt.ReviewReason `json:"reviewReasons"`
		TotalSections   int                `json:"totalSections"`
		Headings        []heading          `json:"headings"`
		HasMore         bool               `json:"hasMore"`
		Sample          string             `json:"sample"`
		SampleTruncated bool               `json:"sampleTruncated"`
	}{value.Version, value.Encoding, value.Preset, value.ParserVersion, value.ReviewReasons, value.TotalSections, headings, value.HasMore, value.Sample, value.SampleTruncated})
}
