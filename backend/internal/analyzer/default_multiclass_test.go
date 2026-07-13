// Conformance test for Legado Default multi-class selectors.
package analyzer

import "testing"

func TestDefaultRuleMatchesSpaceSeparatedClasses(t *testing.T) {
	an := New(`<div class="ptm-list-view-cell ptm-img ptm-col-xs-4"><a>凡人修仙传</a></div>`, "https://example.com", NewJSVM(), NewCacheManager())
	elements, err := an.GetElements("class.ptm-list-view-cell ptm-img ptm-col-xs-4")
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 1 {
		t.Fatalf("elements=%d, want 1", len(elements))
	}
}
