package api

import (
	"context"
	"net/http"
	"time"
)

const webViewProbeTimeout = 8 * time.Second

type webViewStatusResponse struct {
	Status    string `json:"status"`
	CheckedAt int64  `json:"checkedAt"`
}

func (s *readerAPI) handleWebViewStatus(w http.ResponseWriter, r *http.Request) {
	checkedAt := time.Now().UnixMilli()
	if s.webViewProbe == nil {
		writeJSON(w, http.StatusOK, webViewStatusResponse{Status: "not_configured", CheckedAt: checkedAt})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), webViewProbeTimeout)
	defer cancel()
	if err := s.webViewProbe.Probe(ctx); err != nil {
		writeJSON(w, http.StatusOK, webViewStatusResponse{Status: "unavailable", CheckedAt: checkedAt})
		return
	}
	writeJSON(w, http.StatusOK, webViewStatusResponse{Status: "ready", CheckedAt: checkedAt})
}
