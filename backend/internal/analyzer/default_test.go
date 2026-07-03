package analyzer

import (
	"fmt"
	"strings"
	"testing"
)

func TestDefaultRuleDetection(t *testing.T) {
	tests := []struct {
		expr      string
		isJSON    bool
		expectCSS bool
	}{
		{"class.librarylist@tag.li", false, false},
		{"id.jieqi_page_contents@class.c_row", false, false},
		{"class.info@tag.span.0@tag.a@text", false, false},
		{"td.2@text", false, false},
		{"td.-1@text", false, false},
		{"tbody>tr", false, true},      // CSS: has >
		{".librarylist li", false, true}, // CSS
		{"a@href", false, true},        // CSS: tag@attr
		{"/xpath/expr", false, false},  // XPath
		{"$.json.path", false, false},  // JSON
		{"tag.td", false, true},        // ambiguous: CSS tag.class or Default tag.name
		{"tag.td.2@text", false, false},
	}

	for _, tt := range tests {
		mode := detectMode(tt.expr, tt.isJSON)
		isCSS := mode == ModeCSS
		if isCSS != tt.expectCSS {
			t.Errorf("detectMode(%q) = %d (CSS=%v, Default=%d), expected CSS=%v",
				tt.expr, mode, isCSS, ModeDefault, tt.expectCSS)
		}
	}
}

func TestDefaultRuleParsing(t *testing.T) {
	html := `<html><body>
		<div class="librarylist">
			<li><a href="/book1">Book 1</a></li>
			<li><a href="/book2">Book 2</a></li>
		</div>
	</body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "http://example.com", jsVM, cache)

	elements, err := an.GetElements("class.librarylist@tag.li")
	if err != nil {
		t.Fatalf("GetElements error: %v", err)
	}
	if len(elements) != 2 {
		t.Errorf("expected 2 elements, got %d", len(elements))
	}
	for i, el := range elements {
		s := ToString(el)
		if !strings.Contains(s, fmt.Sprintf("book%d", i+1)) {
			t.Errorf("element %d doesn't contain book%d: %s", i, i+1, s[:min(50, len(s))])
		}
	}

	// Test name extraction from first element
	firstHTML := ToString(elements[0])
	elAn := New(firstHTML, "http://example.com", jsVM, cache)
	name, err := elAn.GetString("tag.a@text")
	if err != nil {
		t.Fatalf("GetString error: %v", err)
	}
	if name != "Book 1" {
		t.Errorf("expected 'Book 1', got %q", name)
	}
	href, err := elAn.GetString("tag.a@href")
	if err != nil {
		t.Fatalf("GetString href error: %v", err)
	}
	if href != "/book1" {
		t.Errorf("expected '/book1', got %q", href)
	}
}

func TestDefaultPosition(t *testing.T) {
	html := `<html><body><table><tbody>
		<tr><td>1</td><td>A</td><td>Auth1</td></tr>
	</tbody></table></body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "", jsVM, cache)

	author, err := an.GetString("td.2@text")
	if err != nil {
		t.Fatalf("GetString td.2@text error: %v", err)
	}
	if author != "Auth1" {
		t.Errorf("expected 'Auth1', got %q", author)
	}

	last, err := an.GetString("td.-1@text")
	if err != nil {
		t.Fatalf("GetString td.-1@text error: %v", err)
	}
	if last != "Auth1" {
		t.Errorf("expected 'Auth1' (last), got %q", last)
	}
}

func TestCSSFallback(t *testing.T) {
	html := `<html><body>
		<div class="content"><p>Hello World</p></div>
	</body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "", jsVM, cache)

	result, err := an.GetString(".content p@text")
	if err != nil {
		t.Fatalf("GetString error: %v", err)
	}
	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result)
	}
}

func TestDefaultIDRule(t *testing.T) {
	html := `<html><body>
		<div id="jieqi_page_contents">
			<div class="c_row">Row 1</div>
			<div class="c_row">Row 2</div>
			<div class="c_row">Row 3</div>
		</div>
	</body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "", jsVM, cache)

	elements, err := an.GetElements("id.jieqi_page_contents@class.c_row")
	if err != nil {
		t.Fatalf("GetElements error: %v", err)
	}
	if len(elements) != 3 {
		t.Errorf("expected 3 elements, got %d", len(elements))
	}

	name, err := an.GetString("id.jieqi_page_contents@class.c_row.0@text")
	if err != nil {
		t.Fatalf("GetString error: %v", err)
	}
	if name != "Row 1" {
		t.Errorf("expected 'Row 1', got %q", name)
	}
}

// TestTOCDirectLink verifies that a chapterList rule selecting <a> elements
// directly (without a container) preserves the element tag and attributes
// so field rules like @href and text can extract them.
func TestTOCDirectLink(t *testing.T) {
	html := `<html><body>
		<div class="chapter-list">
			<a href="/ch/1">第一章 起点</a>
			<a href="/ch/2">第二章 修炼</a>
			<a href="/ch/3">第三章 突破</a>
		</div>
	</body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "http://example.com", jsVM, cache)

	// This simulates the TOC path: class.chapter-list@tag.a selects <a> elements directly
	elements, err := an.GetElements("class.chapter-list@tag.a")
	if err != nil {
		t.Fatalf("GetElements error: %v", err)
	}
	if len(elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elements))
	}

	// Each element must be outer HTML so field rules can match the <a> tag and its attributes
	for i, el := range elements {
		elHTML := ToString(el)
		elAn := New(elHTML, "http://example.com", jsVM, cache)

		// chapterName: "text" — standalone getter on current element
		title, err := elAn.GetString("text")
		if err != nil {
			t.Errorf("element %d GetString(text): %v", i, err)
		}
		if title == "" {
			t.Errorf("element %d: title is empty — outer HTML may not preserve <a> tag", i)
		}

		// chapterUrl: "@href" — attribute getter on current element
		chURL, err := elAn.GetString("@href")
		if err != nil {
			t.Errorf("element %d GetString(@href): %v", i, err)
		}
		if chURL == "" {
			t.Errorf("element %d: URL is empty — @href on <a> should return href attr", i)
		}
		if i == 0 && chURL != "/ch/1" {
			t.Errorf("element 0: expected href='/ch/1', got %q", chURL)
		}

		t.Logf("element %d: title=%q url=%q (elHTML=%s)", i, title, chURL, elHTML[:min(len(elHTML), 80)])
	}
}

// TestTOCContainerWithStandaloneGetter verifies the container + shorthand getter pattern:
// chapterList selects a container (e.g. <li>), then chapterName="text" extracts text
// and chapterUrl="tag.a@href" finds the href from a child <a>.
func TestTOCContainerWithStandaloneGetter(t *testing.T) {
	html := `<html><body>
		<ul class="chapter-list">
			<li><a href="/ch/1">第一章 起点</a></li>
			<li><a href="/ch/2">第二章 修炼</a></li>
		</ul>
	</body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "http://example.com", jsVM, cache)

	elements, err := an.GetElements("class.chapter-list@tag.li")
	if err != nil {
		t.Fatalf("GetElements error: %v", err)
	}
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	for i, el := range elements {
		elHTML := ToString(el)
		elAn := New(elHTML, "http://example.com", jsVM, cache)

		// "text" is standalone getter — should return the element's text content
		title, err := elAn.GetString("text")
		if err != nil {
			t.Errorf("element %d GetString(text): %v", i, err)
		}
		if title == "" {
			t.Errorf("element %d: title is empty", i)
		}

		// "a@href" is CSS: find <a>, get href attr
		chURL, err := elAn.GetString("a@href")
		if err != nil {
			t.Errorf("element %d GetString(a@href): %v", i, err)
		}
		if chURL == "" {
			t.Errorf("element %d: URL is empty", i)
		}

		t.Logf("element %d: title=%q url=%q", i, title, chURL)
	}
}

// TestDefaultStandaloneGetter tests bare getter keywords directly on document root.
func TestDefaultStandaloneGetter(t *testing.T) {
	html := `<html><body><a href="/test">Hello</a></body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "http://example.com", jsVM, cache)

	tests := []struct {
		rule string
		want string
		desc string
	}{
		{"text", "Hello", "standalone text getter"},
		{"@href", "/test", "standalone @href getter"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := an.GetString(tt.rule)
			if err != nil {
				t.Fatalf("GetString(%q): %v", tt.rule, err)
			}
			if !strings.Contains(result, tt.want) {
				t.Errorf("GetString(%q) = %q, want containing %q", tt.rule, result, tt.want)
			}
		})
	}
}

// TestTOCFieldRulesOnDirectLink runs the FULL chapter extraction pipeline on
// the most common legado TOC patterns: direct <a> selection with standalone
// getters. This is what BLOCKERs 1+2 were breaking.
func TestTOCFieldRulesOnDirectLink(t *testing.T) {
	html := `<html><body>
		<div class="chapter-list">
			<a href="/ch/1">第一章 起点</a>
			<a href="/ch/2">第二章 修炼</a>
		</div>
	</body></html>`

	jsVM := NewJSVM()
	cache := NewCacheManager()
	an := New(html, "http://example.com", jsVM, cache)

	// Step 1: Select all <a> elements (the chapterList selector)
	elements, err := an.GetElements("class.chapter-list@tag.a")
	if err != nil {
		t.Fatalf("GetElements error: %v", err)
	}

	// Step 2: For each element, extract chapterName and chapterUrl
	type entry struct{ title, url string }
	var chapters []entry
	for _, el := range elements {
		elHTML := ToString(el)
		elAn := New(elHTML, "http://example.com", jsVM, cache)
		title, _ := elAn.GetString("text")
		url, _ := elAn.GetString("@href")
		if title != "" {
			chapters = append(chapters, entry{title, url})
		}
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d — chapterName/text or chapterUrl/@href likely returning empty", len(chapters))
	}
	if chapters[0].title != "第一章 起点" {
		t.Errorf("chapter 0 title: got %q, want %q", chapters[0].title, "第一章 起点")
	}
	if chapters[0].url != "/ch/1" {
		t.Errorf("chapter 0 url: got %q, want %q", chapters[0].url, "/ch/1")
	}
	if chapters[1].title != "第二章 修炼" {
		t.Errorf("chapter 1 title: got %q, want %q", chapters[1].title, "第二章 修炼")
	}
	if chapters[1].url != "/ch/2" {
		t.Errorf("chapter 1 url: got %q, want %q", chapters[1].url, "/ch/2")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
