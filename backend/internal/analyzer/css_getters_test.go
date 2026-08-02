package analyzer

import (
	"slices"
	"testing"
)

func TestExplicitCSSTextNodesKeepsOnlyDirectNodes(t *testing.T) {
	an := New(`<div class="item"> first <span>nested</span> tail </div><div class="item"><b>child only</b></div><div class="item"> second <i>ignored</i> end </div>`, "", NewJSVM(), nil)

	value, err := an.GetStringStrict(`@css:.item@textNodes`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "first\ntail\nsecond\nend" {
		t.Fatalf("GetString textNodes=%q, want direct nodes joined in element order", value)
	}

	values, err := an.GetStringList(`@css:.item@textNodes`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"first\ntail", "second\nend"}; !slices.Equal(values, want) {
		t.Fatalf("GetStringList textNodes=%v, want %v", values, want)
	}
}

func TestExplicitCSSOwnTextExcludesDescendantText(t *testing.T) {
	an := New(`<div class="item">First <span>nested</span> tail</div><div class="item"><b>child only</b></div><div class="item">Second</div>`, "", NewJSVM(), nil)

	value, err := an.GetStringStrict(`@css:.item@ownText`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "First tail\nSecond" {
		t.Fatalf("GetString ownText=%q, want ordered nonempty own text", value)
	}

	values, err := an.GetStringList(`@css:.item@ownText`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"First tail", "Second"}; !slices.Equal(values, want) {
		t.Fatalf("GetStringList ownText=%v, want %v", values, want)
	}
}
