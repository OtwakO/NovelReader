// Readiness contract coverage for container orchestration.
package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestHealthReportsSQLiteReadinessWithoutLeakingFailures(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{db: db, mux: http.NewServeMux()}
	server.registerRoutes()

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthy response: status=%d body=%s", response.Code, response.Body.String())
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed database status=%d, want 503; body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "database is closed") {
		t.Fatalf("readiness leaked database failure: %s", response.Body.String())
	}
}
