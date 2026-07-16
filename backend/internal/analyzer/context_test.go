// Tests for complete book, chapter, and next-chapter JavaScript bindings.
package analyzer

import "testing"

func TestJavaScriptBindsNullNextChapterURL(t *testing.T) {
	an := New(`<div>content</div>`, "https://example.test/read/1", NewJSVM(), nil)
	value, err := an.GetString(`<js>nextChapterUrl === null ? 'none' : 'unexpected'</js>`)
	if err != nil || value != "none" {
		t.Fatalf("nextChapterUrl = %q, err=%v", value, err)
	}
}

func TestJavaScriptReceivesCompleteCrawlContext(t *testing.T) {
	an := New(`<div>content</div>`, "https://example.test/read/1", NewJSVM(), nil)
	an.SetBookDataValues(map[string]interface{}{
		"bookUrl": "https://example.test/book", "name": "Context Book", "author": "Context Author",
		"origin": "https://example.test", "originName": "Fixture", "durChapterIndex": 2,
	})
	an.SetChapterDataValues(map[string]interface{}{
		"url": "https://example.test/read/1", "title": "Chapter One", "index": 2,
		"bookUrl": "https://example.test/book", "baseUrl": "https://example.test/toc",
		"isVip": true, "isVolume": false,
	})
	an.SetNextChapterDataValues(map[string]interface{}{
		"url": "https://example.test/read/2", "title": "Chapter Two", "index": 3,
	})

	value, err := an.GetString(`<js>[book.name,book.author,book.bookUrl,book.origin,book.originName,book.durChapterIndex,chapter.title,chapter.index,chapter.bookUrl,chapter.baseUrl,chapter.isVip,nextChapter.title,nextChapter.url,nextChapter.index].join("|")</js>`)
	if err != nil {
		t.Fatal(err)
	}
	want := "Context Book|Context Author|https://example.test/book|https://example.test|Fixture|2|Chapter One|2|https://example.test/book|https://example.test/toc|true|Chapter Two|https://example.test/read/2|3"
	if value != want {
		t.Fatalf("context = %q, want %q", value, want)
	}
}
