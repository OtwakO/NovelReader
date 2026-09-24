package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

func TestTXTSelectedSectionPreview(t *testing.T) {
	server, sessions, readers, alice, closeStores := newOwnershipServer(t)
	t.Cleanup(closeStores)
	if err := server.fileImports.Quiesce(t.Context(), alice); err != nil {
		t.Fatal(err)
	}
	home, err := readers.Open(t.Context(), alice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	store := txtstore.NewStore(home.DB(), home.Files())
	text := "Chapter 1\n" + strings.Repeat("Literal <script>text</script>.\n", 250) + "End of first chapter.\n"
	receipt, err := store.Receive(t.Context(), rand.Text(), "Example.txt", strings.NewReader(text+"Chapter 2\nSecond chapter.\n"))
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := store.Analyze(t.Context(), receipt.ID, txt.Options{Preset: txt.Preset("english-chapters")})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := sessions.Create(t.Context(), alice, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	get := func(path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	path := fmt.Sprintf("/api/imports/txt/receipts/%s/sections/0?generation=%d", receipt.ID, analysis.Version)
	response := get(path, credential.Token)
	var section txtReviewSectionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &section); err != nil || response.Code != 200 || section.Generation != analysis.Version || section.Index != 0 || section.Text != text {
		t.Fatalf("full section: status=%d bytes=%d err=%v", response.Code, len(section.Text), err)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preview was cacheable")
	}
	summary, err := store.Review(t.Context(), receipt.ID, analysis.Version, 0, 1)
	if err != nil || !summary.SampleTruncated || len(summary.Sample) > 4096 {
		t.Fatalf("summary changed: %v", err)
	}
	if response := get(path, ""); response.Code != 401 {
		t.Fatalf("anonymous: %d", response.Code)
	}
	other, err := sessions.Create(t.Context(), ownershipBob, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if response := get(path, other.Token); response.Code != 404 {
		t.Fatalf("cross-reader preview: %d", response.Code)
	}
	if response := get(strings.Replace(path, "/sections/0", "/sections/99", 1), credential.Token); response.Code != 404 {
		t.Fatalf("missing section: %d", response.Code)
	}
	if err := store.QueueAnalysis(t.Context(), receipt.ID, analysis.Version, txt.Options{}); err != nil {
		t.Fatal(err)
	}
	if response := get(path, credential.Token); response.Code != 409 {
		t.Fatalf("stale generation: %d", response.Code)
	}
}
