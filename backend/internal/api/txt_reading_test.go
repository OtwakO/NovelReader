package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/reading"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

type txtReadingFixture struct {
	store   *txtstore.Store
	files   readerstore.FileStore
	receipt txtstore.Receipt
	item    library.Item
	request func(string, string, string) *httptest.ResponseRecorder
}

func newTXTReadingFixture(t *testing.T) txtReadingFixture {
	t.Helper()
	server, sessions, readers, alice, closeStores := newOwnershipServer(t)
	t.Cleanup(closeStores)
	if err := server.fileImports.Quiesce(t.Context(), alice); err != nil {
		t.Fatal(err)
	}
	home, err := readers.Open(t.Context(), alice)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { home.Close() })
	store := txtstore.NewStore(home.DB(), home.Files())
	receipt, err := store.Receive(t.Context(), rand.Text(), "Example.txt", strings.NewReader("第一章 起点\nLiteral <img src=\"https://invalid.test/private\"> & <script>text</script>.\n第二章 继续\nLast line.\n"))
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := store.Analyze(t.Context(), receipt.ID, txt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Accept(t.Context(), receipt.ID, analysis.Version, "Example", "Author")
	if err != nil {
		t.Fatal(err)
	}
	credential, err := sessions.Create(t.Context(), alice, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: credential.Token})
		response := httptest.NewRecorder()
		server.ServeHTTP(response, r)
		return response
	}
	return txtReadingFixture{store, home.Files(), receipt, item, request}
}

func TestTXTReadingThroughCommonHTTPRoutes(t *testing.T) {
	f := newTXTReadingFixture(t)
	base := "/api/books/" + f.item.ID
	response := f.request(http.MethodGet, base+"/chapters", "")
	var catalog reading.Catalog
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != 200 || len(catalog.Chapters) != 2 || catalog.ContentRevision != f.item.ContentRevision {
		t.Fatalf("catalog: %d %s %v", response.Code, response.Body.String(), err)
	}
	var shape struct {
		Chapters []map[string]any `json:"chapters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &shape); err != nil {
		t.Fatal(err)
	}
	if len(shape.Chapters[0]) != 3 {
		t.Fatalf("native fields leaked: %v", shape.Chapters[0])
	}
	response = f.request(http.MethodGet, fmt.Sprintf("%s/chapters/0/content?contentRevision=%d", base, catalog.ContentRevision), "")
	var content reading.Content
	if err := json.Unmarshal(response.Body.Bytes(), &content); err != nil || response.Code != 200 || content.Version != reading.DocumentVersion || content.OfflineCopy || content.ContentRevision != catalog.ContentRevision {
		t.Fatalf("content: %d %s %v", response.Code, response.Body.String(), err)
	}
	literalFound := false
	for _, block := range content.Document.Blocks {
		if block.Kind != "paragraph" || block.Resource != nil {
			t.Fatalf("TXT treated as HTML: %+v", block)
		}
		literalFound = literalFound || strings.Contains(block.Text, "<img src=")
	}
	if !literalFound {
		t.Fatal("literal markup was removed")
	}
	response = f.request(http.MethodGet, base, "")
	var unread library.Item
	if err := json.Unmarshal(response.Body.Bytes(), &unread); err != nil || unread.LastReadAt != 0 {
		t.Fatalf("admission/content read marked reading: %s %v", response.Body.String(), err)
	}
	guards := fmt.Sprintf(`"contentRevision":%d,"stateVersion":%d`, f.item.ContentRevision, f.item.StateVersion)
	response = f.request(http.MethodPut, base+"/progress", `{`+guards+`,"chapterIndex":1,"position":0.4}`)
	if response.Code != 200 {
		t.Fatalf("progress: %s", response.Body.String())
	}
	response = f.request(http.MethodPost, base+"/bookmarks", fmt.Sprintf(`{"id":"mark","contentRevision":%d,"stateVersion":%d,"chapterIndex":1,"position":0.4}`, f.item.ContentRevision, f.item.StateVersion+1))
	if response.Code != 201 {
		t.Fatalf("bookmark: %s", response.Body.String())
	}
	var mark library.Bookmark
	if err := json.Unmarshal(response.Body.Bytes(), &mark); err != nil || mark.ChapterTitle != catalog.Chapters[1].Title {
		t.Fatalf("mark=%+v %v", mark, err)
	}
	response = f.request(http.MethodGet, base, "")
	var item library.Item
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil || item.CurrentChapterTitle != mark.ChapterTitle || item.DurChapterPos != 0.4 || item.StateVersion != f.item.StateVersion+2 || item.LastReadAt <= 0 {
		t.Fatalf("shared state: %s %v", response.Body.String(), err)
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, base + "/booksource", "", 404},
		{http.MethodGet, base + "/chapters/0/content", "", 400},
		{http.MethodGet, fmt.Sprintf("%s/chapters/0/content?contentRevision=%d", base, f.item.ContentRevision+1), "", 409},
		{http.MethodGet, fmt.Sprintf("%s/chapters/9/content?contentRevision=%d", base, f.item.ContentRevision), "", 404},
		{http.MethodPut, base + "/progress", `{` + guards + `,"chapterIndex":0,"position":0}`, 409},
		{http.MethodPut, base + "/progress", fmt.Sprintf(`{"contentRevision":%d,"stateVersion":%d,"chapterIndex":9,"position":0}`, item.ContentRevision, item.StateVersion), 400},
	} {
		if got := f.request(tc.method, tc.path, tc.body); got.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, got.Code, got.Body.String())
		}
	}
	for i := 0; i < 2; i++ {
		if got := f.request(http.MethodDelete, "/api/books?id="+f.item.ID, ""); got.Code != 200 {
			t.Fatalf("remove: %s", got.Body.String())
		}
	}
	if got := f.request(http.MethodGet, base, ""); got.Code != 404 {
		t.Fatalf("publication survived: %s", got.Body.String())
	}
	if _, err := f.store.Get(t.Context(), f.receipt.ID); !errors.Is(err, txtstore.ErrNotFound) {
		t.Fatalf("receipt survived: %v", err)
	}
	root, err := f.files.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.Stat(f.receipt.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bytes survived: %v", err)
	}
}

func TestTXTRemovalWarningAndRetryProtectPendingFiles(t *testing.T) {
	f := newTXTReadingFixture(t)
	root, err := f.files.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	extra := filepath.Join(filepath.Dir(f.receipt.Path), "keep.txt")
	if err := root.WriteFile(extra, []byte("not owned by TXT"), 0600); err != nil {
		t.Fatal(err)
	}
	response := f.request(http.MethodDelete, "/api/books?id="+f.item.ID, "")
	var result struct {
		Status   string   `json:"status"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || result.Status != "removed" || len(result.Warnings) != 1 || result.Warnings[0] != "txt_cleanup_pending" {
		t.Fatalf("warning: %d %s %v", response.Code, response.Body.String(), err)
	}
	if got := f.request(http.MethodGet, "/api/books/"+f.item.ID, ""); got.Code != 404 {
		t.Fatal("removal did not hide publication")
	}
	value, err := f.store.Get(t.Context(), f.receipt.ID)
	if err != nil || value.State != txtstore.Removing {
		t.Fatalf("retry record: %+v %v", value, err)
	}
	if _, err := root.Stat(extra); err != nil {
		t.Fatalf("unowned file lost: %v", err)
	}
	if err := root.Remove(extra); err != nil {
		t.Fatal(err)
	}
	response = f.request(http.MethodDelete, "/api/books?id="+f.item.ID, "")
	if response.Code != 200 || strings.Contains(response.Body.String(), "warnings") {
		t.Fatalf("retry: %s", response.Body.String())
	}
	pending, err := f.store.Receive(t.Context(), rand.Text(), "Pending.txt", strings.NewReader("original"))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.request(http.MethodDelete, "/api/books?id="+pending.ID, ""); got.Code != 404 {
		t.Fatalf("pending receipt accepted as book: %s", got.Body.String())
	}
	if _, err := root.Stat(pending.Path); err != nil {
		t.Fatalf("pending original lost: %v", err)
	}
}
