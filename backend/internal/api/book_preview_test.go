package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otwako/novelreader/internal/book"
)

func TestHandlePreviewBookReturnsDetailAndTOCWithoutPersistence(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/book":
			_, _ = fmt.Fprint(w, `<h1 class="name">Fixture Novel</h1><span class="author">Fixture Author</span><div class="intro">First&nbsp;line<br>Second line</div><span class="latest">Chapter 2</span><time class="updated">2026-08-13</time><span class="words">20万字</span><a class="toc" href="/toc">目录</a>`)
		case "/toc":
			_, _ = fmt.Fprint(w, `<a class="chapter" href="/chapter/1">Chapter 1</a><a class="chapter" href="/chapter/2">Chapter 2</a>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer sourceServer.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	importWorkflowSource(t, server, sourceServer.URL)

	requestBody, _ := json.Marshal(map[string]any{
		"name": "Search Name", "author": "Fixture Author", "sourceName": "Fixture Source",
		"sourceUrl": sourceServer.URL, "bookUrl": sourceServer.URL + "/book",
		"alternateSources": []map[string]string{{"sourceUrl": "https://alternate.test", "bookUrl": "/book", "sourceName": "Alternate"}},
	})
	response := performAPIRequest(server, http.MethodPost, "/api/books/preview", requestBody)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", response.Code, response.Body.String())
	}
	var preview struct {
		Book     book.PreviewBook `json:"book"`
		Chapters []book.Chapter   `json:"chapters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Book.Name != "Search Name" || preview.Book.Intro != "First line\nSecond line" || preview.Book.LastChapter != "Chapter 2" || preview.Book.UpdateTime != "2026-08-13" || preview.Book.WordCount != "20万字" {
		t.Fatalf("preview book=%+v", preview.Book)
	}
	if len(preview.Book.AlternateSources) != 1 || len(preview.Chapters) != 2 || preview.Chapters[0].BookID != "" {
		t.Fatalf("preview=%+v", preview)
	}
	books, err := server.bookStore.ListBooks()
	if err != nil || len(books) != 0 {
		t.Fatalf("preview persisted books=%+v err=%v", books, err)
	}
}

func TestHandleShelveBookPersistsInfoAndTOCWithoutContentValidation(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/book":
			_, _ = fmt.Fprint(w, `<h1 class="name">Fixture Novel</h1><span class="author">Fixture Author</span><a class="toc" href="/toc">目录</a>`)
		case "/toc":
			_, _ = fmt.Fprint(w, `<a class="chapter" href="/chapter/1">Chapter 1</a>`)
		case "/chapter/1":
			http.Error(w, "content unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer sourceServer.Close()
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	importWorkflowSource(t, server, sourceServer.URL)
	requestBody, _ := json.Marshal(map[string]any{"id": "book-1", "name": "Fixture Novel", "author": "Fixture Author", "sourceUrl": sourceServer.URL, "bookUrl": sourceServer.URL + "/book"})
	response := performAPIRequest(server, http.MethodPost, "/api/books/shelve", requestBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	stored, err := server.bookStore.GetBook("book-1")
	if err != nil || stored == nil || stored.TotalChapterNum != 1 {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	chapters, err := server.bookStore.GetChapters("book-1")
	if err != nil || len(chapters) != 1 {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
}

func TestHandleAddReadableBookPreservesExistingProgress(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/book":
			_, _ = fmt.Fprint(w, `<h1 class="name">Fixture Novel</h1><span class="author">Fixture Author</span><a class="toc" href="/toc">目录</a>`)
		case "/toc":
			_, _ = fmt.Fprint(w, `<a class="chapter" href="/chapter/1">Chapter 1</a><a class="chapter" href="/chapter/2">Chapter 2</a>`)
		case "/chapter/1":
			_, _ = fmt.Fprint(w, `<article class="content">readable</article>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer sourceServer.Close()
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	importWorkflowSource(t, server, sourceServer.URL)
	existing := &book.Book{ID: "existing", Name: "Fixture Novel", Author: "Fixture Author", SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", Origin: "Fixture Source", DurChapterIndex: 9, DurChapterPos: .4}
	if err := server.bookStore.AddBook(existing); err != nil {
		t.Fatal(err)
	}
	requestBody, _ := json.Marshal(map[string]any{"id": "candidate", "name": existing.Name, "author": existing.Author, "sourceUrl": sourceServer.URL, "bookUrl": existing.BookURL})
	response := performAPIRequest(server, http.MethodPost, "/api/books/readable", requestBody)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Book                 book.Book `json:"book"`
		FirstReadableChapter int       `json:"firstReadableChapter"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Book.ID != existing.ID || result.FirstReadableChapter != 9 || result.Book.DurChapterPos != .4 {
		t.Fatalf("result=%+v", result)
	}
}

func TestHandleAddReadableBookValidatesBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name           string
		content        string
		wantStatus     int
		wantBookCount  int
		wantFirstIndex int
	}{
		{name: "readable", content: "real chapter content", wantStatus: http.StatusCreated, wantBookCount: 1, wantFirstIndex: 1},
		{name: "empty", content: "", wantStatus: http.StatusBadGateway, wantBookCount: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/book":
					_, _ = fmt.Fprint(w, `<h1 class="name">Fixture Novel</h1><span class="author">Fixture Author</span><a class="toc" href="/toc">目录</a>`)
				case "/toc":
					_, _ = fmt.Fprint(w, `<div class="volume">Volume One</div><a class="chapter" href="/chapter/1">Chapter 1</a>`)
				case "/chapter/1":
					_, _ = fmt.Fprintf(w, `<article class="content">%s</article>`, test.content)
				default:
					http.NotFound(w, r)
				}
			}))
			defer sourceServer.Close()

			server, closeDB := newWorkflowAPIServer(t)
			defer closeDB()
			importWorkflowSourceWithVolume(t, server, sourceServer.URL)
			requestBody, _ := json.Marshal(map[string]any{
				"id": "book-1", "name": "Fixture Novel", "author": "Fixture Author",
				"sourceUrl": sourceServer.URL, "bookUrl": sourceServer.URL + "/book",
			})
			response := performAPIRequest(server, http.MethodPost, "/api/books/readable", requestBody)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			books, err := server.bookStore.ListBooks()
			if err != nil || len(books) != test.wantBookCount {
				t.Fatalf("books=%+v err=%v", books, err)
			}
			if test.wantBookCount == 0 {
				return
			}
			var result struct {
				Book                 book.Book `json:"book"`
				FirstReadableChapter int       `json:"firstReadableChapter"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.FirstReadableChapter != test.wantFirstIndex {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			chapters, err := server.bookStore.GetChapters(result.Book.ID)
			if err != nil || len(chapters) != 2 {
				t.Fatalf("chapters=%+v err=%v", chapters, err)
			}
		})
	}
}

func importWorkflowSource(t *testing.T, server *Server, sourceURL string) {
	t.Helper()
	rawSource, _ := json.Marshal([]map[string]any{{
		"bookSourceUrl": sourceURL, "bookSourceName": "Fixture Source", "bookSourceType": 0, "enabled": true,
		"ruleBookInfo": map[string]string{"name": ".name@text", "author": ".author@text", "intro": ".intro@html", "lastChapter": ".latest@text", "updateTime": ".updated@text", "wordCount": ".words@text", "tocUrl": ".toc@href"},
		"ruleToc":      map[string]string{"chapterList": ".chapter", "chapterName": "text", "chapterUrl": "href"},
		"ruleContent":  map[string]string{"content": ".content@text"},
	}})
	response := performAPIRequest(server, http.MethodPost, "/api/sources", rawSource)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
}

func importWorkflowSourceWithVolume(t *testing.T, server *Server, sourceURL string) {
	t.Helper()
	rawSource, _ := json.Marshal([]map[string]any{{
		"bookSourceUrl": sourceURL, "bookSourceName": "Fixture Source", "bookSourceType": 0, "enabled": true,
		"ruleBookInfo": map[string]string{"name": ".name@text", "author": ".author@text", "tocUrl": ".toc@href"},
		"ruleToc":      map[string]string{"chapterList": ".volume, .chapter", "chapterName": "text", "chapterUrl": "href", "isVolume": "class.volume"},
		"ruleContent":  map[string]string{"content": ".content@text"},
	}})
	response := performAPIRequest(server, http.MethodPost, "/api/sources", rawSource)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
}
