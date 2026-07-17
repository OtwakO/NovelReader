// Element-list errors distinguish empty matches from invalid rule execution.
package analyzer

import (
	"errors"
	"strings"
	"testing"
)

func TestGetElementsPreservesJavaScriptFailure(t *testing.T) {
	an := New(`<div></div>`, "https://fixture.test", NewJSVM(), nil)
	_, err := an.GetElements(`<js>throw new Error('fixture failure')</js>`)
	if err == nil || errors.Is(err, ErrNoElements) || !strings.Contains(err.Error(), "fixture failure") {
		t.Fatalf("error=%v, want JavaScript failure", err)
	}
}

func TestGetElementsPreservesMixedBranchFailure(t *testing.T) {
	an := New(`<div></div>`, "https://fixture.test", NewJSVM(), nil)
	for _, rule := range []string{
		`.missing || <js>throw new Error('mixed failure')</js>`,
		`.missing && <js>throw new Error('mixed failure')</js>`,
	} {
		_, err := an.GetElements(rule)
		if err == nil || errors.Is(err, ErrNoElements) || !strings.Contains(err.Error(), "mixed failure") {
			t.Fatalf("rule=%q error=%v, want mixed branch failure", rule, err)
		}
	}
}

func TestGetElementsReportsValidEmptySelector(t *testing.T) {
	an := New(`<div></div>`, "https://fixture.test", NewJSVM(), nil)
	_, err := an.GetElements(`.missing`)
	if !errors.Is(err, ErrNoElements) {
		t.Fatalf("error=%v, want ErrNoElements", err)
	}
}

func TestGetElementsRejectsEmptyRule(t *testing.T) {
	an := New(`<div></div>`, "https://fixture.test", NewJSVM(), nil)
	for _, rule := range []string{" ", "||", "&&"} {
		_, err := an.GetElements(rule)
		if err == nil || errors.Is(err, ErrNoElements) {
			t.Fatalf("rule=%q error=%v, want invalid rule", rule, err)
		}
	}
}
