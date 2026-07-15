// Tests for Legado @put/@get variables evaluated in parsed-rule order.
package analyzer

import "testing"

func TestRulePutGetVariables(t *testing.T) {
	an := New(`<h1 class="name">Rule Book</h1><span class="author">Rule Author</span><span class="word">Book</span>`, "https://example.test/", NewJSVM(), nil)

	content, err := an.GetElement(`@put:{n:".name@text",a:'.author@text',word:".word@text"}`)
	if err != nil {
		t.Fatal(err)
	}
	an.SetContent(content)

	name, err := an.GetString(`@get:{n}##@get:{word}##Novel`)
	if err != nil || name != "Rule Novel" {
		t.Fatalf("name = %q, err=%v", name, err)
	}
	author, err := an.GetString(`@get:{a}`)
	if err != nil || author != "Rule Author" {
		t.Fatalf("author = %q, err=%v", author, err)
	}
	dynamic, err := an.GetString(`.name@text##@get:{word}##Novel`)
	if err != nil || dynamic != "Rule Novel" {
		t.Fatalf("dynamic replacement = %q, err=%v", dynamic, err)
	}
	missing, err := an.GetString(`@get:{missing}`)
	if err != nil || missing != "" {
		t.Fatalf("missing = %q, err=%v", missing, err)
	}
}

func TestRulePutSuffixAndJavaScriptSubstitution(t *testing.T) {
	an := New(`{"title":"Stored Book","id":"42","novelId":"43"}`, "https://example.test/", NewJSVM(), nil)

	title, err := an.GetString(`$.title@put:{book:$.missing||$.novelId}`)
	if err != nil || title != "Stored Book" {
		t.Fatalf("title = %q, err=%v", title, err)
	}
	value, err := an.GetString(`<js>"@get:{book}" + "!"</js>`)
	if err != nil || value != "43!" {
		t.Fatalf("JS value = %q, err=%v", value, err)
	}
}

func TestRuleVariablesPreserveEmptyAndMalformedSemantics(t *testing.T) {
	an := New(`<h1 class="name">Book</h1>`, "https://example.test/", NewJSVM(), nil)

	if value, err := an.GetString(""); err != nil || value != "" {
		t.Fatalf("empty rule = %q, err=%v", value, err)
	}
	if _, err := an.GetElement(`@put:{"strict":".name@text",empty:""}`); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{`@get:{empty}`, `@get:{missing}`} {
		if value, err := an.GetString(rule); err != nil || value != "" {
			t.Fatalf("%s = %q, err=%v", rule, value, err)
		}
	}
	if value, err := an.GetString(`.name@text@put:{bad:}`); err != nil || value != "Book" {
		t.Fatalf("malformed map value = %q, err=%v", value, err)
	}
}

func TestRuleVariablesPersistThroughSourceState(t *testing.T) {
	state := &testSourceState{vars: map[string]string{}, memory: map[string]interface{}{}}
	first := New(`{"id":"persisted"}`, "https://example.test/", NewJSVM(), nil)
	first.SetSourceState(state)
	if _, err := first.GetString(`$.id@put:{saved:$.id}`); err != nil {
		t.Fatal(err)
	}

	second := New(`{}`, "https://example.test/", NewJSVM(), nil)
	second.SetSourceState(state)
	value, err := second.GetString(`@get:{saved}`)
	if err != nil || value != "persisted" {
		t.Fatalf("persisted value = %q, err=%v", value, err)
	}
}

func TestRulePutRunsInSelectedSegmentOrder(t *testing.T) {
	an := New(`<h1 class="name">Before</h1><span class="author">Author</span>`, "https://example.test/", NewJSVM(), nil)

	content, err := an.GetElement(`<js>result.replace("Before", "After")</js>@put:{n:".name@text"}`)
	if err != nil {
		t.Fatal(err)
	}
	an.SetContent(content)
	name, _ := an.GetString(`@get:{n}`)
	if name != "Before" {
		t.Fatalf("ordered put = %q, want Before from analyzer root content", name)
	}

	value, err := an.GetString(`.name@text||.missing@put:{unused:".author@text"}`)
	if err != nil || value != "After" {
		t.Fatalf("OR value = %q, err=%v", value, err)
	}
	unused, _ := an.GetString(`@get:{unused}`)
	if unused != "" {
		t.Fatalf("unselected branch stored %q", unused)
	}
}
