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
