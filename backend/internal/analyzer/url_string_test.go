package analyzer

import "testing"

const twoLinkHTML = `<div><a class="detail" href="/book/1">Detail</a><a class="chapter" href="/chapter/1">Chapter</a></div>`

func TestGetURLStringStrictUsesFirstJSoupValueOnly(t *testing.T) {
	an := New(twoLinkHTML, "https://example.test/search", nil, nil)

	value, err := an.GetURLStringStrict("a@href")
	if err != nil {
		t.Fatal(err)
	}
	if value != "/book/1" {
		t.Fatalf("value = %q, want first JSoup value", value)
	}
}

func TestGetURLStringStrictSelectsFirstValueBeforeReplacement(t *testing.T) {
	an := New(twoLinkHTML, "", nil, nil)

	value, err := an.GetURLStringStrict(`a@href##^/book/1$##/resolved`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "/resolved" {
		t.Fatalf("value = %q, want replacement applied after selecting first value", value)
	}
}

func TestGetURLStringStrictUsesFirstCombinedJSoupConnectorValue(t *testing.T) {
	an := New(twoLinkHTML, "", nil, nil)

	value, err := an.GetURLStringStrict(`a.detail@href&&a.chapter@href`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "/book/1" {
		t.Fatalf("value = %q, want first combined JSoup value", value)
	}
}

func TestGetURLStringStrictUsesFirstNonEmptyJSoupAlternative(t *testing.T) {
	an := New(twoLinkHTML, "", nil, nil)

	value, err := an.GetURLStringStrict(`a.missing@href||a.chapter@href`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "/chapter/1" {
		t.Fatalf("value = %q, want first non-empty JSoup alternative", value)
	}
}

func TestGetURLStringStrictPreservesChainedJSResult(t *testing.T) {
	an := New(twoLinkHTML, "", NewJSVM(), nil)

	value, err := an.GetURLStringStrict(`a@href<js>result + "?from=js"</js>`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "/book/1?from=js" {
		t.Fatalf("value = %q, want JS chained from first JSoup value", value)
	}
}

func TestGetURLStringStrictDoesNotChangeOtherModes(t *testing.T) {
	xpath := New(twoLinkHTML, "", nil, nil)
	gotXPath, err := xpath.GetURLStringStrict(`//a/@href`)
	if err != nil {
		t.Fatal(err)
	}
	wantXPath, err := xpath.GetStringStrict(`//a/@href`)
	if err != nil {
		t.Fatal(err)
	}
	if gotXPath != wantXPath {
		t.Fatalf("XPath URL value = %q, ordinary value = %q", gotXPath, wantXPath)
	}

	json := New(`{"urls":["/book/1","/chapter/1"]}`, "", nil, nil)
	gotJSON, err := json.GetURLStringStrict(`$.urls[*]`)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.GetStringStrict(`$.urls[*]`)
	if err != nil {
		t.Fatal(err)
	}
	if gotJSON != wantJSON {
		t.Fatalf("JSONPath URL value = %q, ordinary value = %q", gotJSON, wantJSON)
	}
}
