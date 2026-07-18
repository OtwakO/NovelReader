package analyzer

import "testing"

func TestLiveDefaultListRuleShapes(t *testing.T) {
	html := `<div id="j"><li>one</li><li>two</li></div>
<div id="articlelist"><ul>first</ul><ul>second</ul></div>
<div class="item">alpha</div><div class="item">beta</div>`
	an := New(html, "https://example.com/", NewJSVM(), NewCacheManager())

	for rule, want := range map[string]int{
		"#j@li":          2,
		"class.item":     2,
		"id.articlelist": 1,
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
