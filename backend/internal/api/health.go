// SQLite-backed readiness for container orchestration.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := s.health
	if health == nil {
		writeError(w, http.StatusServiceUnavailable, "service not ready")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := health.PingContext(ctx); err != nil {
		slog.Error("readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "service not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
