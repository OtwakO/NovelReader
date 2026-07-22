package analyzer

import (
	"strings"
	"testing"
)

func TestLiveDefaultListRuleShapes(t *testing.T) {
	html := `<div id="j"><li>one</li><li>two</li></div>
<div id="articlelist"><ul>first</ul><ul>second</ul><li>header</li><li>book</li></div>
<div class="right-book-list"><li>book one</li><li>book two</li></div>
<div class="box"><div class="col-12">card one</div><div class="col-12">card two</div></div>
<div class="item">alpha</div><div class="item">beta</div>`
	an := New(html, "https://example.com/", NewJSVM(), NewCacheManager())

	for rule, want := range map[string]int{
		"#j@li":                   2,
		"#articlelist li!0":       1,
		"id.articlelist@tag.ul!0": 1,
		".right-book-list@tag.li": 2,
		".box@.col-12":            2,
		"class.item":              2,
		"id.articlelist":          1,
	} {
		if mode := detectMode(rule, false); mode != ModeDefault {
			t.Errorf("detectMode(%q) = %d, want Default", rule, mode)
			continue
		}
		elements, err := an.GetElements(rule)
		if err != nil {
			t.Errorf("GetElements(%q): %v", rule, err)
		} else if len(elements) != want {
			t.Errorf("GetElements(%q) = %d elements, want %d", rule, len(elements), want)
		}
	}
}

func TestDefaultBareElementTraversalUsesEveryParent(t *testing.T) {
	html := `<table><tbody><tr><td>A</td></tr></tbody><tbody><tr><td>B</td></tr><tr><td>C</td></tr></tbody></table><div id="identity" class="kind">Text</div>`
	an := New(html, "https://example.com/", NewJSVM(), NewCacheManager())

	elements, err := an.GetElements(`tbody@tag.tr||class.list-group-item`)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 3 {
		t.Fatalf("elements = %d, want all 3 rows across tbody parents", len(elements))
	}
	elements, err = an.GetElements(`tbody@tr!0`)
	if err != nil || len(elements) != 1 || !strings.Contains(ToString(elements[0]), ">C<") {
		t.Fatalf("excluded bare traversal=%#v err=%v, want row C", elements, err)
	}
	for rule, want := range map[string]string{"div@id": "identity", "div@class": "kind"} {
		value, err := an.GetStringStrict(rule)
		if err != nil || value != want {
			t.Errorf("CSS getter %q = %q, err=%v, want %q", rule, value, err, want)
		}
	}
}

func TestJSONWildcardFilterSelectsObjectValues(t *testing.T) {
	an := New(`{"first":[{"novelName":"A","novelId":1}],"second":[{"novelName":"B","novelId":2}]}`, "https://example.com/", NewJSVM(), NewCacheManager())
	elements, err := an.GetElements(`@Json:$.*[?(@.novelName)]`)
	if err != nil || len(elements) != 2 {
		t.Fatalf("GetElements()=%#v err=%v", elements, err)
	}
	for index, want := range []string{"A", "B"} {
		value, err := New(ToString(elements[index]), "https://example.com/", NewJSVM(), nil).GetString(`$.novelName`)
		if err != nil || value != want {
			t.Fatalf("index=%d novelName=%q err=%v", index, value, err)
		}
	}
}

func TestJSONWildcardFilterRejectsMalformedInput(t *testing.T) {
	for _, content := range []string{
		`{"first":[{"novelName":"A"}]`,
		`{"first":[{"novelName":"A"}]} trailing`,
	} {
		an := New(content, "https://example.com/", NewJSVM(), NewCacheManager())
		if _, err := an.GetElements(`@Json:$.*[?(@.novelName)]`); err == nil {
			t.Errorf("content %q did not fail", content)
		}
	}
}

func TestUnpositionedBareElementTraversesChild(t *testing.T) {
	an := New(`<table><tbody><tr><td>A</td></tr><tr><td>B</td></tr></tbody></table><div id="identity" class="kind" data-role="card">Text</div>`, "https://example.com/", NewJSVM(), NewCacheManager())

	for rule, want := range map[string]int{"tbody@tr": 2, "tr@td": 2} {
		elements, err := an.GetElements(rule)
		if err != nil || len(elements) != want {
			t.Errorf("GetElements(%q) = %#v, err=%v, want %d elements", rule, elements, err, want)
		}
	}
	for rule, want := range map[string]string{"div@id": "identity", "div@class": "kind", "div@data-role": "card"} {
		value, err := an.GetStringStrict(rule)
		if err != nil || value != want {
			t.Errorf("CSS getter %q = %q, err=%v, want %q", rule, value, err, want)
		}
	}
}

func TestSelectedTableRowKeepsContextForFieldRules(t *testing.T) {
	an := New(`<table><tbody><tr data-row="book"><td>kind</td><td><a href="/book">Book</a></td></tr></tbody></table>`, "https://example.com/", NewJSVM(), NewCacheManager())

	elements, err := an.GetElements(`tbody@tag.tr`)
	if err != nil || len(elements) != 1 {
		t.Fatalf("GetElements() = %#v, err=%v", elements, err)
	}
	row := New(ToString(elements[0]), "https://example.com/", NewJSVM(), NewCacheManager())
	value, err := row.GetStringStrict(`td.1@a@text`)
	if err != nil || value != "Book" {
		t.Fatalf("GetStringStrict() = %q, err=%v, want Book", value, err)
	}
	value, err = row.GetStringStrict(`@data-row`)
	if err != nil || value != "book" {
		t.Fatalf("current row attribute = %q, err=%v, want book", value, err)
	}
}

func TestDefaultPositionedElementReadsAttribute(t *testing.T) {
	an := New(`<li><a><img title="First"><img title="Second"></a></li>`, "https://example.com/", NewJSVM(), NewCacheManager())

	value, err := an.GetStringStrict(`img.0@title`)
	if err != nil || value != "First" {
		t.Fatalf("GetStringStrict() = %q, err=%v, want First", value, err)
	}
}

func TestPositionedParentStillTraversesBareChild(t *testing.T) {
	an := New(`<ol><li>Prefix <a href="/first">First</a></li><li><a href="/second">Second</a></li></ol>`, "https://example.com/", NewJSVM(), NewCacheManager())

	value, err := an.GetStringStrict(`li.0@a`)
	if err != nil || value != "First" {
		t.Fatalf("GetStringStrict() = %q, err=%v, want First", value, err)
	}
}

func TestDottedDefaultExclusionStillRoutesToDefault(t *testing.T) {
	an := New(`<div class="items">skip</div><div class="items">keep</div>`, "https://example.com/", NewJSVM(), NewCacheManager())

	values, err := an.GetStringList(`class.items.!0@text`)
	if err != nil || len(values) != 1 || values[0] != "keep" {
		t.Fatalf("GetStringList() = %#v, err=%v, want keep", values, err)
	}
}

func TestDefaultIndexAppliesPerParent(t *testing.T) {
	html := `<div class="group"><a>first A</a><a>second A</a></div>
<div class="group"><a>first B</a><a>second B</a></div>`
	an := New(html, "https://example.com/", NewJSVM(), NewCacheManager())

	values, err := an.GetStringList(`.group@tag.a.0@text`)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != "first A" || values[1] != "first B" {
		t.Fatalf("values = %#v, want first child from each parent", values)
	}
}

func TestDefaultExclusionSupportsNegativeIndex(t *testing.T) {
	an := New(`<ol><li>one</li><li>two</li><li>three</li></ol>`, "https://example.com/", NewJSVM(), NewCacheManager())

	values, err := an.GetStringList(`ol@li!-1@text`)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("values = %#v, want last item excluded", values)
	}
}
