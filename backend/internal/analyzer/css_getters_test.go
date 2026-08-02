package analyzer

import (
	"slices"
	"testing"
)

func TestExplicitCSSHTMLAndAllReturnAggregateOuterHTML(t *testing.T) {
	html := `<section class="item"><p>One</p><script>run()</script></section><section class="item"><style>.x{}</style><b>Two</b></section>`

	cleaned := New(html, "", NewJSVM(), nil)
	value, err := cleaned.GetStringStrict(`@css:.item@html`)
	if err != nil {
		t.Fatal(err)
	}
	wantCleaned := "<section class=\"item\"><p>One</p></section>\n<section class=\"item\"><b>Two</b></section>"
	if value != wantCleaned {
		t.Fatalf("GetString html=%q, want aggregate cleaned outer HTML %q", value, wantCleaned)
	}
	values, err := cleaned.GetStringList(`@css:.item@html`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{wantCleaned}; !slices.Equal(values, want) {
		t.Fatalf("GetStringList html=%v, want one aggregate value %v", values, want)
	}

	preserved := New(html, "", NewJSVM(), nil)
	value, err = preserved.GetStringStrict(`@css:.item@all`)
	if err != nil {
		t.Fatal(err)
	}
	wantAll := "<section class=\"item\"><p>One</p><script>run()</script></section>\n<section class=\"item\"><style>.x{}</style><b>Two</b></section>"
	if value != wantAll {
		t.Fatalf("GetString all=%q, want aggregate preserved outer HTML %q", value, wantAll)
	}
	values, err = preserved.GetStringList(`@css:.item@all`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{wantAll}; !slices.Equal(values, want) {
		t.Fatalf("GetStringList all=%v, want one aggregate value %v", values, want)
	}
}

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
