// Conformance tests for source-scoped cookies, variables, and memory state.
package sourceexec

import (
	"testing"
)

func TestSourceSessionPersistsCookiesAndVariablesWithinOneSession(t *testing.T) {
	session := NewSourceSession()
	const sourceURL = "https://example.test/search"

	if err := session.SetCookie(sourceURL, "token", "abc"); err != nil {
		t.Fatal(err)
	}
	if got := session.GetCookie(sourceURL, "token"); got != "abc" {
		t.Fatalf("cookie = %q, want abc", got)
	}
	if got := session.CookieHeader(sourceURL); got != "token=abc" {
		t.Fatalf("cookie header = %q, want token=abc", got)
	}

	session.PutVariable("csrf", "token-value")
	if got := session.GetVariable("csrf"); got != "token-value" {
		t.Fatalf("source variable = %q, want token-value", got)
	}
	session.PutMemory("temporary", "value")
	if got := session.GetMemory("temporary"); got != "value" {
		t.Fatalf("memory value = %v, want value", got)
	}
}

func TestSourceSessionCopiesRequestHeaders(t *testing.T) {
	session := NewSourceSession()
	sourceHeaders := map[string]string{"X-Source": "one"}
	session.SetRequestHeaders(sourceHeaders)
	sourceHeaders["X-Source"] = "mutated"

	if got := session.RequestHeaders()["X-Source"]; got != "one" {
		t.Fatalf("stored header = %q, want one", got)
	}
	headers := session.RequestHeaders()
	headers["X-Source"] = "changed"
	if got := session.RequestHeaders()["X-Source"]; got != "one" {
		t.Fatalf("returned header mutated session: %q", got)
	}
}

func TestSourceSessionLoginHeadersOverrideRequestHeadersAndSyncCookie(t *testing.T) {
	session := NewSourceSession()
	session.SetRequestHeaders(map[string]string{"Authorization": "source", "X-Source": "yes"})
	if err := session.SetCookie("https://example.test/", "old", "stale"); err != nil {
		t.Fatal(err)
	}
	session.SetLoginHeader(`{authorization:'login',Cookie:"token=abc",'X-Login':'yes'}`)

	headers := session.RequestHeaders()
	if headers["authorization"] != "login" || headers["X-Source"] != "yes" || headers["X-Login"] != "yes" {
		t.Fatalf("merged headers=%v", headers)
	}
	if got := session.CookieHeader("https://example.test/"); got != "token=abc" {
		t.Fatalf("cookie=%q, want login cookie to replace stale jar cookie", got)
	}
}

func TestSourceSessionsDoNotShareState(t *testing.T) {
	first := NewSourceSession()
	second := NewSourceSession()
	const sourceURL = "https://example.test/"

	_ = first.SetCookie(sourceURL, "session", "one")
	first.PutVariable("key", "one")

	if got := second.GetCookie(sourceURL, "session"); got != "" {
		t.Fatalf("cookie leaked between sessions: %q", got)
	}
	if got := second.GetVariable("key"); got != "" {
		t.Fatalf("variable leaked between sessions: %q", got)
	}
}
