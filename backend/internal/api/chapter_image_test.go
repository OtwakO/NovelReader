package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/processor"
)

func TestStoredChapterImageUsesIndexedURLHeadersAndDecodeScript(t *testing.T) {
	encoded := []byte{0x13, 0x17, 0x1b, 0x1d, 0x1f}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/novel/chapter/1":
			_, _ = fmt.Fprint(w, `<article class="content"><p>before</p><img src="../image.bin,{&quot;headers&quot;:{&quot;Referer&quot;:&quot;https://reader.test/chapter&quot;}}" alt="Route map"><p>after</p></article>`)
		case "/novel/image.bin":
			if r.Header.Get("X-Image-Token") != "fixture-token" || r.Header.Get("Referer") != "https://reader.test/chapter" {
				http.Error(w, "missing image headers", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(encoded)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	ruleContent, _ := json.Marshal(map[string]string{
		"content":     ".content@html",
		"imageDecode": `if (src.indexOf('/novel/image.bin') < 0) throw new Error('wrong src: ' + src); result.map(function(value) { return value ^ 90; })`,
	})
	source := booksource.BookSource{
		BookSourceURL: upstream.URL, BookSourceName: "chapter image fixture",
		Header: `{"X-Image-Token":"fixture-token"}`, RuleContent: string(ruleContent),
	}
	if err := server.standalone.sourceStore.Upsert(&source); err != nil {
		t.Fatal(err)
	}
	storedBook := book.Book{ID: "image-book", Name: "Image", SourceID: source.ID, SourceURL: source.BookSourceURL, BookURL: upstream.URL + "/novel/book"}
	if err := server.standalone.bookStore.AddBook(&storedBook); err != nil {
		t.Fatal(err)
	}
	if err := server.standalone.bookStore.SaveChapters(storedBook.ID, []book.Chapter{{Index: 0, Title: "Chapter", URL: upstream.URL + "/novel/chapter/1"}}); err != nil {
		t.Fatal(err)
	}

	contentResponse := performAPIRequest(server, http.MethodGet, "/api/books/image-book/chapters/0/content", nil)
	if contentResponse.Code != http.StatusOK || bytes.Contains(contentResponse.Body.Bytes(), []byte("image.bin")) {
		t.Fatalf("content status=%d body=%s", contentResponse.Code, contentResponse.Body.String())
	}
	var content chapterContentResponse
	if err := json.Unmarshal(contentResponse.Body.Bytes(), &content); err != nil || content.Version != proseDocumentVersion || content.Document.Kind != "prose" || len(content.Document.Blocks) != 3 {
		t.Fatalf("content=%+v err=%v body=%s", content, err, contentResponse.Body.String())
	}
	imageBlock := content.Document.Blocks[1]
	if imageBlock.Kind != processor.ProseBlockImage || imageBlock.Resource == nil || imageBlock.Resource.Href != "/api/books/image-book/chapters/0/images/0" || imageBlock.Alt != "Route map" {
		t.Fatalf("image block=%+v body=%s", imageBlock, contentResponse.Body.String())
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books/image-book/chapters/0/images/0?url=http://127.0.0.1/private", nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), []byte("IMAGE")) {
		t.Fatalf("status=%d body=%v", response.Code, response.Body.Bytes())
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Content-Security-Policy") != "sandbox; default-src 'none'" {
		t.Fatalf("security headers=%v", response.Header())
	}
}

func TestStoredChapterImageRejectsAndroidBitmapDecoder(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	source := booksource.BookSource{
		BookSourceURL: "https://source.test", BookSourceName: "bitmap fixture",
		RuleContent: `{"content":"body@html","imageDecode":"Packages.android.graphics.BitmapFactory.decodeByteArray(result, 0, result.length)"}`,
	}
	if err := server.standalone.sourceStore.Upsert(&source); err != nil {
		t.Fatal(err)
	}
	storedBook := book.Book{ID: "bitmap-book", Name: "Bitmap", SourceID: source.ID, SourceURL: source.BookSourceURL, BookURL: "https://source.test/book"}
	if err := server.standalone.bookStore.AddBook(&storedBook); err != nil {
		t.Fatal(err)
	}
	chapter := book.Chapter{Index: 0, Title: "Chapter", URL: "https://source.test/chapter"}
	if err := server.standalone.bookStore.SaveChapters(storedBook.ID, []book.Chapter{chapter}); err != nil {
		t.Fatal(err)
	}
	if err := server.standalone.bookStore.SaveChapterCache(book.CachedChapter{
		BookID: storedBook.ID, SourceID: source.ID, ChapterIndex: chapter.Index, ChapterURL: chapter.URL,
		Blocks: []processor.ProseBlock{{Kind: processor.ProseBlockImage, Src: "https://source.test/image"}},
	}); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books/bitmap-book/chapters/0/images/0", nil)
	if response.Code != http.StatusNotImplemented || !bytes.Contains(response.Body.Bytes(), []byte("chapter_image_decoder_unsupported")) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
