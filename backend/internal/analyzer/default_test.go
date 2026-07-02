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
