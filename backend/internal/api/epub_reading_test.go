package api

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/reading"
)

func readingEPUBBytes(t *testing.T, replacements ...[2]string) []byte {
	t.Helper()
	var archive, picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(&archive)
	entries := [][2]string{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"book.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest><item id="main" href="main.xhtml" media-type="application/xhtml+xml"/><item id="notes" href="notes.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" properties="nav" media-type="application/xhtml+xml"/><item id="image" href="private/pic.png" properties="cover-image" media-type="image/png"/></manifest><spine><itemref idref="main"/><itemref idref="notes" linear="no"/></spine></package>`},
		{"main.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Main</title></head><body><p id="start">Publication prose <a epub:type="noteref" href="notes.xhtml#note">Note</a></p><img src="private/pic.png" alt="Illustration"/></body></html>`},
		{"notes.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Notes</title></head><body><aside id="note" epub:type="footnote"><p>Auxiliary note</p><a href="main.xhtml#start">Back</a></aside></body></html>`},
		{"nav.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="main.xhtml#start">Authored start</a></li><li><a href="notes.xhtml#note">Notes</a></li></ol></nav></body></html>`},
		{"private/pic.png", picture.String()},
	}
	for _, replacement := range replacements {
		for i := range entries {
			if entries[i][0] == replacement[0] {
				entries[i] = replacement
			}
		}
	}
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry[0], Method: zip.Deflate}
		if entry[0] == "mimetype" {
			h.Method = zip.Store
		}
		w, err := z.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(entry[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func TestEPUBPublicationThroughAuthorizedReadingRoutes(t *testing.T) {
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
	store := epubstore.NewStore(home.DB(), home.Files())
	receipt, err := store.Receive(t.Context(), rand.Text(), "Novel.epub", bytes.NewReader(readingEPUBBytes(t)), epub.OriginalImages)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := store.QueuePreparation(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Prepare(t.Context(), receipt.ID, attempt.Generation); err != nil {
		t.Fatal(err)
	}
	credential, err := sessions.Create(t.Context(), alice, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	request := func(token, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	get := func(path string) *httptest.ResponseRecorder { return request(credential.Token, "GET", path, "") }
	base := "/api/books/" + receipt.ID
	if got := get(base + "/chapters"); got.Code != 404 {
		t.Fatal("draft readable", got.Code)
	}
	item, err := store.Accept(t.Context(), receipt.ID, attempt.Generation, "Novel", "Author")
	if err != nil {
		t.Fatal(err)
	}
	var catalog reading.Catalog
	response := get(base + "/chapters")
	if err = json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != 200 || len(catalog.Chapters) != 2 || !catalog.Chapters[1].Auxiliary || catalog.Navigation == nil || len(catalog.Navigation.Entries) != 2 {
		t.Fatal(response.Code, response.Body.String(), err)
	}
	var content reading.Content
	response = get(base + "/chapters/0/content?contentRevision=1")
	if err = json.Unmarshal(response.Body.Bytes(), &content); err != nil || response.Code != 200 || content.Version != reading.StructuredDocumentVersion {
		t.Fatal(response.Code, response.Body.String(), err)
	}
	var imageHref string
	noteFound := false
	var walk func([]reading.Block)
	walk = func(blocks []reading.Block) {
		for _, b := range blocks {
			if b.Resource != nil {
				imageHref = b.Resource.Href
			}
			if b.Target != nil && b.Target.ChapterIndex == 1 && b.Role == "noteref" {
				noteFound = true
			}
			walk(b.Children)
		}
	}
	walk(content.Document.Blocks)
	if imageHref == "" || !noteFound || strings.Contains(response.Body.String(), "private/pic.png") {
		t.Fatal("projection/bindings", response.Body.String())
	}
	imageResponse := get(imageHref)
	if imageResponse.Code != 200 || imageResponse.Header().Get("Content-Type") != "image/png" || imageResponse.Header().Get("X-Content-Type-Options") != "nosniff" || imageResponse.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("image", imageResponse.Code, imageResponse.Header())
	}
	var dto libraryBookResponse
	response = get(base)
	if err = json.Unmarshal(response.Body.Bytes(), &dto); err != nil || !strings.HasPrefix(dto.CoverDisplayURL, base+"/cover?v=") || dto.CoverURL != "" {
		t.Fatal("cover", response.Body.String(), err)
	}
	cover := get(dto.CoverDisplayURL)
	if cover.Code != 200 || !bytes.Equal(cover.Body.Bytes(), imageResponse.Body.Bytes()) || cover.Header().Get("Content-Type") != "image/png" || cover.Header().Get("Cache-Control") != coverCacheControl || cover.Header().Get("Vary") != "Cookie" {
		t.Fatal("original cover", cover.Code, cover.Header())
	}
	for _, invalid := range []string{base + "/cover", base + "/cover?v=stale"} {
		if got := get(invalid); got.Code != 404 || got.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("unqualified cover", got.Code, got.Header())
		}
	}
	if got := request("", "GET", dto.CoverDisplayURL, ""); got.Code != 401 {
		t.Fatal("unauthenticated cover", got.Code)
	}
	if got := request("", "GET", imageHref, ""); got.Code != 401 {
		t.Fatal("unauthenticated", got.Code)
	}
	bob := readerstore.UserID("22222222-2222-4222-8222-222222222222")
	if err = readers.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}
	other, err := sessions.Create(t.Context(), bob, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if got := request(other.Token, "GET", imageHref, ""); got.Code != 404 {
		t.Fatal("wrong reader", got.Code, got.Body.String())
	}
	if got := request(other.Token, "GET", dto.CoverDisplayURL, ""); got.Code != 404 {
		t.Fatal("wrong reader cover", got.Code)
	}
	stale, err := url.Parse(imageHref)
	if err != nil {
		t.Fatal(err)
	}
	query := stale.Query()
	query.Set("revision", "2")
	stale.RawQuery = query.Encode()
	if got := get(stale.String()); got.Code != 409 {
		t.Fatal("stale resource", got.Code)
	}
	if got := get(base + "/chapters/1/content?contentRevision=1"); got.Code != 200 || !strings.Contains(got.Body.String(), "Auxiliary note") {
		t.Fatal("auxiliary", got.Code, got.Body.String())
	}
	guards := `"contentRevision":1,"stateVersion":0`
	if got := request(credential.Token, "PUT", base+"/progress", `{`+guards+`,"chapterIndex":1,"position":0.4}`); got.Code != 400 {
		t.Fatal("auxiliary progress", got.Code, got.Body.String())
	}
	if got := request(credential.Token, "POST", base+"/bookmarks", `{`+guards+`,"id":"note","chapterIndex":1,"position":0.4}`); got.Code != 201 {
		t.Fatal("auxiliary bookmark", got.Code, got.Body.String())
	}
	if got := request(credential.Token, "PUT", base+"/progress", `{"contentRevision":1,"stateVersion":1,"chapterIndex":0,"position":0.4}`); got.Code != 200 {
		t.Fatal("main progress", got.Code, got.Body.String())
	}
	if _, err := home.DB().ExecContext(t.Context(), `UPDATE library_items SET content_revision=content_revision+1 WHERE id=?`, item.ID); err != nil {
		t.Fatal(err)
	}
	var renewed libraryBookResponse
	response = get(base)
	if err := json.Unmarshal(response.Body.Bytes(), &renewed); err != nil || renewed.CoverDisplayURL == dto.CoverDisplayURL {
		t.Fatal("publication cover identity", response.Body.String(), err)
	}
	if got := get(dto.CoverDisplayURL); got.Code != 404 || got.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("stale publication cover", got.Code, got.Header())
	}
	if got := get(renewed.CoverDisplayURL); got.Code != 200 || !bytes.Equal(got.Body.Bytes(), cover.Body.Bytes()) {
		t.Fatal("renewed cover", got.Code)
	}
	if got := request(credential.Token, "DELETE", "/api/books?id="+item.ID, ""); got.Code != 200 {
		t.Fatal("remove", got.Code, got.Body.String())
	}
	if got := get(dto.CoverDisplayURL); got.Code != 404 || got.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("removed cover", got.Code, got.Header())
	}
	if got := get(imageHref); got.Code != 404 {
		t.Fatal("removed image", got.Code)
	}
}
