package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

const maxRuntimeCookieRequestBytes = 256 * 1024

type runtimeCookiePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
}

type replaceRuntimeCookiesRequest struct {
	CurrentPassword string                        `json:"currentPassword"`
	Cookies         []sourceprofile.RuntimeCookie `json:"cookies"`
}

type runtimeCookieMetadata struct {
	Scope string   `json:"scope"`
	Names []string `json:"names"`
}

func (s *Server) handleRuntimeCookieMetadata(w http.ResponseWriter, r *http.Request) {
	cookies, err := s.sourceProfiles.RuntimeCookies(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeRuntimeCookieError(w, err)
		return
	}
	metadata := make([]runtimeCookieMetadata, len(cookies))
	for index, cookie := range cookies {
		metadata[index] = runtimeCookieMetadata{Scope: cookie.Scope, Names: runtimeCookieNames(cookie.Header)}
	}
	preventCredentialCaching(w)
	writeJSON(w, http.StatusOK, map[string]any{"cookies": metadata})
}

func (s *Server) handleRevealRuntimeCookies(w http.ResponseWriter, r *http.Request) {
	var request runtimeCookiePasswordRequest
	if !decodeRuntimeCookieRequest(w, r, &request) {
		return
	}
	if !s.verifyRuntimeCookiePassword(w, r, request.CurrentPassword) {
		return
	}
	cookies, err := s.sourceProfiles.RuntimeCookies(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeRuntimeCookieError(w, err)
		return
	}
	preventCredentialCaching(w)
	writeJSON(w, http.StatusOK, map[string]any{"cookies": cookies})
}

func (s *Server) handleReplaceRuntimeCookies(w http.ResponseWriter, r *http.Request) {
	var request replaceRuntimeCookiesRequest
	if !decodeRuntimeCookieRequest(w, r, &request) {
		return
	}
	if !s.verifyRuntimeCookiePassword(w, r, request.CurrentPassword) {
		return
	}
	if err := s.sourceProfiles.ReplaceRuntimeCookies(r.Context(), r.PathValue("id"), request.Cookies); err != nil {
		s.writeRuntimeCookieError(w, err)
		return
	}
	s.closeSourceRuntime(r.PathValue("id"))
	cookies, err := s.sourceProfiles.RuntimeCookies(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeRuntimeCookieError(w, err)
		return
	}
	metadata := make([]runtimeCookieMetadata, len(cookies))
	for index, cookie := range cookies {
		metadata[index] = runtimeCookieMetadata{Scope: cookie.Scope, Names: runtimeCookieNames(cookie.Header)}
	}
	preventCredentialCaching(w)
	writeJSON(w, http.StatusOK, map[string]any{"cookies": metadata})
}

func (s *Server) verifyRuntimeCookiePassword(w http.ResponseWriter, r *http.Request, password string) bool {
	if s.auth == nil {
		writeError(w, http.StatusNotImplemented, "current-password verification is unavailable")
		return false
	}
	if err := s.auth.VerifyCurrentPassword(r.Context(), password); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "current password is incorrect")
			return false
		}
		writeError(w, http.StatusServiceUnavailable, "runtime cookie management unavailable")
		return false
	}
	return true
}

func (s *Server) writeRuntimeCookieError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sourceprofile.ErrSourceNotInstalled):
		writeError(w, http.StatusNotFound, "book source not found")
	case errors.Is(err, sourceprofile.ErrInvalidRuntimeCookies):
		writeError(w, http.StatusBadRequest, "invalid runtime cookies")
	default:
		writeError(w, http.StatusInternalServerError, "runtime cookie management unavailable")
	}
}

func decodeRuntimeCookieRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRuntimeCookieRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func runtimeCookieNames(header string) []string {
	names := make([]string, 0)
	for _, pair := range strings.Split(header, ";") {
		name, _, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok && name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func preventCredentialCaching(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
