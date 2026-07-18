package analyzer

import "testing"

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
