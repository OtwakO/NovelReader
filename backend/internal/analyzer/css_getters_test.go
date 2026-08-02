package analyzer

import (
	"slices"
	"strings"
	"testing"
)

func TestExplicitCSSJsoupPositionalSelectors(t *testing.T) {
	html := `<ul class="items"><b>skip</b><li><a>one</a></li><li><a>two</a></li><li><a>three</a></li></ul>`
	an := New(html, "", NewJSVM(), nil)

	value, err := an.GetStringStrict(`@css:.items li:eq(1) a@text`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "one" {
		t.Fatalf("eq value=%q, want sibling-indexed descendant value one", value)
	}

	values, err := an.GetStringList(`@css:.items > *:lt(2)@text`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"skip", "one"}; !slices.Equal(values, want) {
		t.Fatalf("lt values=%v, want %v", values, want)
	}
	values, err = an.GetStringList(`@css:.items > *:gt(1)@text`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"two", "three"}; !slices.Equal(values, want) {
		t.Fatalf("gt values=%v, want %v", values, want)
	}

	elements, err := an.GetElements(`@css:.items > *:eq(2)`)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 1 || !strings.Contains(ToString(elements[0]), ">two<") {
		t.Fatalf("eq elements=%v, want the sibling at zero-based index 2", elements)
	}

	quoted := New(`<i data-rule=":eq(1)">literal</i><i data-rule="other">other</i>`, "", NewJSVM(), nil)
	value, err = quoted.GetStringStrict(`@css:[data-rule=":eq(1)"]@text`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "literal" {
		t.Fatalf("quoted positional text value=%q, want literal", value)
	}
}

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
