package api

import (
	"net/http"
	"strconv"

	"github.com/otwako/novelreader/internal/txtstore"
)

type txtReviewSectionResponse struct {
	Generation int64  `json:"generation"`
	Index      int    `json:"index"`
	Title      string `json:"title"`
	Text       string `json:"text"`
}

func (s *readerAPI) handleTXTReviewSection(w http.ResponseWriter, r *http.Request) {
	s.writeTXTReviewSection(w, r, false)
}

func (s *readerAPI) handleTXTReparseSection(w http.ResponseWriter, r *http.Request) {
	s.writeTXTReviewSection(w, r, true)
}

func (s *readerAPI) writeTXTReviewSection(w http.ResponseWriter, r *http.Request, reparse bool) {
	generation, err := strconv.ParseInt(r.URL.Query().Get("generation"), 10, 64)
	index, indexErr := strconv.Atoi(r.PathValue("section"))
	if err != nil || generation < 1 || indexErr != nil || index < 0 {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Expected a saved generation and section index")
		return
	}
	var value txtstore.Review
	if reparse {
		value, err = s.txtStore.ReviewReparseSection(r.Context(), r.PathValue("id"), generation, index)
	} else {
		value, err = s.txtStore.ReviewSection(r.Context(), r.PathValue("id"), generation, index)
	}
	if err != nil {
		writeTXTError(w, err)
		return
	}
	if len(value.Headings) == 0 {
		writeTXTError(w, txtstore.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, txtReviewSectionResponse{Generation: value.Version, Index: index, Title: value.Headings[0].Title, Text: value.Sample})
}
