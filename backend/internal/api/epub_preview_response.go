package api

import "github.com/otwako/novelreader/internal/epubstore"

type epubPreviewHeading struct {
	Index     int    `json:"index"`
	Title     string `json:"title"`
	Auxiliary bool   `json:"auxiliary"`
}

type epubPreviewResponse struct {
	Generation      int64                `json:"generation"`
	Title           string               `json:"title"`
	Authors         []string             `json:"authors"`
	Language        string               `json:"language,omitempty"`
	TotalSections   int                  `json:"totalSections"`
	Headings        []epubPreviewHeading `json:"headings"`
	HasMore         bool                 `json:"hasMore"`
	Sample          string               `json:"sample"`
	SampleTruncated bool                 `json:"sampleTruncated"`
	NeedsReview     bool                 `json:"needsReview"`
	Diagnostics     []string             `json:"diagnostics"`
	Notices         []string             `json:"notices,omitempty"`
	Images          struct {
		Mode            string `json:"mode"`
		Profile         string `json:"profile,omitempty"`
		Backend         string `json:"backend,omitempty"`
		DerivativeCount int    `json:"derivativeCount"`
		DerivativeBytes int64  `json:"derivativeBytes"`
	} `json:"images"`
}

func epubPreviewDTO(review epubstore.Review) epubPreviewResponse {
	response := epubPreviewResponse{Generation: review.Generation, Title: review.Title, Authors: append([]string{}, review.Authors...),
		Language: review.Language, TotalSections: review.TotalSections, Headings: make([]epubPreviewHeading, 0, len(review.Headings)),
		HasMore: review.HasMore, Sample: review.Sample, SampleTruncated: review.SampleTruncated,
		NeedsReview: len(review.Diagnostics) > 0, Diagnostics: append([]string{}, review.Diagnostics...)}
	for _, heading := range review.Headings {
		response.Headings = append(response.Headings, epubPreviewHeading{heading.Index, heading.Title, !heading.Main})
	}
	response.Images.Mode = string(review.ImageProcessing.Mode)
	response.Images.Profile = review.ImageProcessing.Profile
	response.Images.Backend = review.ImageProcessing.Backend
	response.Images.DerivativeCount = review.ImageProcessing.DerivativeCount
	response.Images.DerivativeBytes = review.ImageProcessing.DerivativeBytes
	if review.ImageProcessing.Backend == "portable" {
		response.Notices = []string{"epub_portable_encoder"}
	}
	return response
}
