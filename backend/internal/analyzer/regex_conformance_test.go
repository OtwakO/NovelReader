// Conformance tests for Legado standalone regex rules.
package analyzer

import "testing"

func TestJavaRegexEscapesWorkThroughAnalyzer(t *testing.T) {
	for _, test := range []struct {
		content string
		rule    string
		want    string
	}{
		{content: "章\u00a012:30\n下一章", rule: `##\h[\d:]+\d\n##▪`, want: "章▪下一章"},
		{content: "X\t \u00a0\u1680\u180e\u2000\u200a\u202f\u205f\u3000X", rule: `##\h##`, want: "XX"},
		{content: "word space", rule: `##[^\h]+##`, want: " "},
		{content: `《剑来》 作者：烽火`, rule: `##\《|\》|作者.*|\s##`, want: "剑来"},
		{content: `分类：玄幻 最新章节：第一章`, rule: `##[\u4e00-\u9fa5]+：##`, want: "玄幻 第一章"},
	} {
		analyzer := New(test.content, "https://example.test/", NewJSVM(), nil)
		value, err := analyzer.GetStringStrict(test.rule)
		if err != nil || value != test.want {
			t.Errorf("rule %q value=%q err=%v, want %q", test.rule, value, err, test.want)
		}
	}

	analyzer := New("text", "https://example.test/", NewJSVM(), nil)
	if _, err := analyzer.GetStringStrict(`##\q##`); err == nil {
		t.Fatal("unsupported alphabetic escape was accepted")
	}
	if _, err := analyzer.GetStringStrict(`##\u12##`); err == nil {
		t.Fatal("malformed Unicode escape was accepted")
	}
}

func TestInlineReplacementNormalizesJavaIdentityEscapes(t *testing.T) {
	analyzer := New(`<p class="intro">简介：现言</p>`, "https://example.test/", NewJSVM(), nil)
	value, err := analyzer.GetStringStrict(`class.intro@text##\简介：`)
	if err != nil || value != "现言" {
		t.Fatalf("value=%q err=%v, want 现言", value, err)
	}
}

func TestStandaloneRegexRuleUsesOnlyFirstMatchedReplacement(t *testing.T) {
	analyzer := New(`<meta author="忘语"><meta author="烽火">`, "https://example.test/", NewJSVM(), nil)
	value, err := analyzer.GetStringStrict(`##author="([^"]+)"##$1###`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "忘语" {
		t.Fatalf("regex value = %q, want only the first matched replacement", value)
	}
}

func TestStandaloneRegexFirstMatchReturnsEmptyWhenNothingMatches(t *testing.T) {
	analyzer := New(`<meta title="凡人修仙传">`, "https://example.test/", NewJSVM(), nil)
	value, err := analyzer.GetString(`##author="([^"]+)"##$1###`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("regex value = %q, want empty string", value)
	}
}
