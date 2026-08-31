package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
	"github.com/otwako/novelreader/internal/webview"
)

type startSourceBrowserRequest struct {
	BrowserRequestID  string  `json:"browserRequestId"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor"`
}

func (s *Server) handleStartSourceBrowser(w http.ResponseWriter, r *http.Request) {
	if s.browserSessions == nil || s.sourceProfiles == nil || s.sourceStore == nil {
		writeError(w, http.StatusNotImplemented, "interactive browser is unavailable")
		return
	}
	var request startSourceBrowserRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid browser request")
		return
	}
	sourceID := r.PathValue("id")
	source, err := s.sourceStore.GetByID(sourceID)
	if err != nil || source == nil {
		writeError(w, http.StatusNotFound, "book source not found")
		return
	}
	profile, err := s.sourceProfiles.Load(r.Context(), sourceID)
	if err != nil {
		writeSourceBrowserError(w, err)
		return
	}
	session := sourceexec.NewSourceSession()
	sourceprofile.ApplyAuthentication(session, sourceprofile.DecodeAuthentication(profile.Authentication))
	frame, err := s.browserSessions.Start(r.Context(), sourceID, request.BrowserRequestID, webview.InteractiveViewport{Width: request.Width, Height: request.Height, DeviceScaleFactor: request.DeviceScaleFactor}, session)
	if err != nil {
		writeSourceBrowserError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, frame)
}

func (s *Server) handleSourceBrowserFrame(w http.ResponseWriter, r *http.Request) {
	if s.browserSessions == nil {
		writeError(w, http.StatusNotImplemented, "interactive browser is unavailable")
		return
	}
	frame, err := s.browserSessions.Frame(r.Context(), r.PathValue("id"), r.PathValue("sessionID"))
	if err != nil {
		writeSourceBrowserError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, frame)
}

func (s *Server) handleSourceBrowserInput(w http.ResponseWriter, r *http.Request) {
	if s.browserSessions == nil {
		writeError(w, http.StatusNotImplemented, "interactive browser is unavailable")
		return
	}
	var input webview.InteractiveInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid browser input")
		return
	}
	frame, err := s.browserSessions.Input(r.Context(), r.PathValue("id"), r.PathValue("sessionID"), input)
	if err != nil {
		writeSourceBrowserError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, frame)
}

func (s *Server) handleCloseSourceBrowser(w http.ResponseWriter, r *http.Request) {
	if s.browserSessions == nil {
		writeError(w, http.StatusNotImplemented, "interactive browser is unavailable")
		return
	}
	sourceID := r.PathValue("id")
	session, err := s.browserSessions.Close(r.Context(), sourceID, r.PathValue("sessionID"), r.URL.Query().Get("save") == "true")
	if err != nil {
		writeSourceBrowserError(w, err)
		return
	}
	if r.URL.Query().Get("save") == "true" {
		profile, err := s.sourceProfiles.Load(r.Context(), sourceID)
		if err != nil {
			writeSourceBrowserError(w, err)
			return
		}
		authentication := sourceprofile.CaptureAuthentication(session, sourceprofile.DecodeAuthentication(profile.Authentication))
		document, err := json.Marshal(authentication)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encode browser authentication")
			return
		}
		if err := s.sourceProfiles.SaveAuthentication(r.Context(), sourceID, document); err != nil {
			writeError(w, http.StatusInternalServerError, "save browser authentication: "+err.Error())
			return
		}
		s.deleteSourceSession(sourceID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"closed": true})
}

func writeSourceBrowserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sourceinteraction.ErrBrowserSessionNotFound):
		writeError(w, http.StatusGone, "browser session expired")
	case errors.Is(err, sourceprofile.ErrSourceNotInstalled):
		writeError(w, http.StatusNotFound, "book source not found")
	default:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	}
}
