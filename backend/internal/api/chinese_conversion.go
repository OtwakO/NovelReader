package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/chineseconv"
)

const (
	maxConversionTexts = 20000
	maxConversionBytes = 2 << 20
)

type chineseConversionRequest struct {
	Mode  string   `json:"mode"`
	Texts []string `json:"texts"`
}

type chineseConversionResponse struct {
	Texts []string `json:"texts"`
}

func (s *Server) conversionCapability() chineseconv.Capability {
	if s.chineseConversion == nil {
		return chineseconv.Capability{Available: false, Modes: []string{}}
	}
	return s.chineseConversion.Capability()
}

func (s *Server) handleChineseConversionCapability(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.conversionCapability())
}

func (s *Server) handleChineseConversion(w http.ResponseWriter, r *http.Request) {
	if !s.conversionCapability().Available {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"code":  "chinese_conversion_unavailable",
			"error": "Chinese conversion is unavailable in this server build",
		})
		return
	}

	var input chineseConversionRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxConversionBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid Chinese conversion request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid Chinese conversion request")
		return
	}
	if len(input.Texts) > maxConversionTexts || (input.Mode != chineseconv.ModeSimplified && input.Mode != chineseconv.ModeTraditionalTaiwan) {
		writeError(w, http.StatusBadRequest, "invalid Chinese conversion request")
		return
	}
	var bytes int
	for _, text := range input.Texts {
		bytes += len(text)
		if bytes > maxConversionBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "Chinese conversion request is too large")
			return
		}
	}

	converted, err := s.chineseConversion.Convert(r.Context(), input.Mode, input.Texts)
	if err != nil {
		status := http.StatusInternalServerError
		code := "chinese_conversion_failed"
		if errors.Is(err, chineseconv.ErrUnavailable) {
			status = http.StatusServiceUnavailable
			code = "chinese_conversion_unavailable"
		}
		writeJSON(w, status, map[string]any{"code": code, "error": strings.TrimSpace(err.Error())})
		return
	}
	writeJSON(w, http.StatusOK, chineseConversionResponse{Texts: converted})
}
