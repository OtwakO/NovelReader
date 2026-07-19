// Rule templates interpolate against the current value before literal or JS evaluation.
package analyzer

import "testing"

func TestTemplateScannerIgnoresQuotedBracesAndPreservesUnclosedInput(t *testing.T) {
	expanded, err := replaceTemplateExpressions(`before{{"}"}}after`, func(expression string) (string, error) {
		if expression != `"}"` {
			t.Fatalf("expression=%q", expression)
		}
		return "}", nil
	})
	if err != nil || expanded != "before}after" {
		t.Fatalf("expanded=%q err=%v", expanded, err)
	}
	unclosed := `before{{"}"after`
	expanded, err = replaceTemplateExpressions(unclosed, func(string) (string, error) {
		t.Fatal("unclosed template was evaluated")
		return "", nil
	})
	if err != nil || expanded != unclosed {
		t.Fatalf("expanded=%q err=%v", expanded, err)
	}
}

func TestGetStringStrictAllowsMissingFallbackButPropagatesCompositeScriptErrors(t *testing.T) {
	an := New(`{"fallback":"value"}`, "https://fixture.test", NewJSVM(), nil)
	value, err := an.GetStringStrict(`$.missing||$.fallback`)
	if err != nil || value != "value" {
		t.Fatalf("fallback value=%q err=%v", value, err)
	}
	for _, rule := range []string{`@js:throw new Error('broken')||$.fallback`, `$.fallback&&@js:throw new Error('broken')`} {
		if _, err := an.GetStringStrict(rule); err == nil {
			t.Fatalf("rule %q swallowed script error", rule)
		}
	}
}

func TestTemplateConnectorsRemainTopLevel(t *testing.T) {
	an := New(`{"a":"left","b":"right","grade":"9.2"}`, "https://fixture.test", NewJSVM(), nil)

	value, err := an.GetStringStrict(`{{$.missing}}||$.grade`)
	if err != nil || value != "9.2" {
		t.Fatalf("template OR value=%q err=%v", value, err)
	}
	value, err = an.GetStringStrict(`{{$.missing||$.grade}}`)
	if err != nil || value != "9.2" {
		t.Fatalf("inner template OR value=%q err=%v", value, err)
	}
	value, err = an.GetStringStrict(`{{$.missing||$.grade##9\.2##rated}}`)
	if err != nil || value != "rated" {
		t.Fatalf("inner template replacement value=%q err=%v", value, err)
	}
	value, err = an.GetStringStrict(`{{$.a}}&&{{$.b}}`)
	if err != nil || value != "left right" {
		t.Fatalf("template AND value=%q err=%v", value, err)
	}
}

func TestSingleBraceJSONInterpolationUsesCurrentValue(t *testing.T) {
	an := New(`{"grade":"9.2","crazy_rating":"8.7"}`, "https://fixture.test", NewJSVM(), nil)

	for rule, want := range map[string]string{
		`{$.grade} / {{$.grade}}`:         "9.2 / 9.2",
		`{$.grade}分`:                      "9.2分",
		`评分 {$.grade} / {$.crazy_rating}`: "评分 9.2 / 8.7",
		`{$.grade}分##分## points`:          "9.2 points",
	} {
		value, err := an.GetStringStrict(rule)
		if err != nil || value != want {
			t.Errorf("rule %q value=%q err=%v, want %q", rule, value, err, want)
		}
	}
}

func TestDoubleAtPrefixForcesDefaultModeOnJSON(t *testing.T) {
	an := New(`{"kind":"fantasy"}`, "https://fixture.test", NewJSVM(), nil)
	value, err := an.GetStringStrict(`{{@@class.tag@text}},{{$.kind}}`)
	if err != nil || value != ",fantasy" {
		t.Fatalf("forced Default template value=%q err=%v", value, err)
	}
}

func TestEmbeddedJSONPathAgainstHTMLKeepsLiteralFallback(t *testing.T) {
	an := New(`<td class="cover"></td>`, "https://fixture.test", NewJSVM(), nil)

	value, err := an.GetStringStrict(`{{$.coverUrl}}http://u3v.cn/5zBiW8`)
	if err != nil || value != "http://u3v.cn/5zBiW8" {
		t.Fatalf("template value=%q err=%v", value, err)
	}
	if _, err := an.GetStringStrict(`$.coverUrl`); err == nil {
		t.Fatal("terminal JSONPath against HTML did not fail")
	}
	if _, err := an.GetStringStrict(`{{throw new Error('broken')}}fallback`); err == nil {
		t.Fatal("template JavaScript error was swallowed")
	}
}

func TestOptionalJSONPathCanFeedJavaScriptDefault(t *testing.T) {
	an := New(`<article class="articlegeneral">book</article>`, "https://fixture.test", NewJSVM(), nil)

	value, err := an.GetStringStrict("$.thumbnail\n@js:result || 'fallback.jpg'")
	if err != nil || value != "fallback.jpg" {
		t.Fatalf("pipeline value=%q err=%v", value, err)
	}
	if _, err := an.GetStringStrict(`$.thumbnail`); err == nil {
		t.Fatal("terminal JSONPath against HTML did not fail")
	}
}

func TestWrappedRegexRemovesMatchesWithImplicitEmptyReplacement(t *testing.T) {
	an := New("alpha|\nbeta|", "https://fixture.test", NewJSVM(), nil)
	value, err := an.GetStringStrict(`<js>##(?m)\|$</js>`)
	if err != nil || value != "alpha\nbeta" {
		t.Fatalf("wrapped regex value=%q err=%v", value, err)
	}
}

func TestRuleTemplatesAndNewlineJavaScriptUseCurrentJSONValue(t *testing.T) {
	an := New(`{"id":"42","fallback":"A"}`, "https://fixture.test", NewJSVM(), nil)
	for _, test := range []struct {
		rule string
		want string
	}{
		{rule: `https://fixture.test/book/{{$.id}}`, want: `https://fixture.test/book/42`},
		{rule: "$.id\n@js:`book-${result}`", want: "book-42"},
		{rule: "@js:const id='{{$.id||$.fallback}}'; id", want: "42"},
	} {
		got, err := an.GetString(test.rule)
		if err != nil {
			t.Fatalf("rule %q: %v", test.rule, err)
		}
		if got != test.want {
			t.Fatalf("rule %q=%q, want %q", test.rule, got, test.want)
		}
	}
}
