// Conformance test for session-backed URL template JavaScript.
package sourceexec

import (
	"context"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
)

func TestExecutorUsesSourceSessionInURLTemplates(t *testing.T) {
	session := NewSourceSession()
	if err := session.SetCookie("https://example.test/", "token", "fixture"); err != nil {
		t.Fatal(err)
	}
	transport := &captureTransport{}
	executor := NewExecutorWithSession(analyzer.NewJSVM(), transport, session)

	_, err := executor.Execute(context.Background(), "https://example.test/{{cookie.getKey(source.key, 'token')}}", "", 1, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	if transport.spec.URL != "https://example.test/fixture" {
		t.Fatalf("session-backed URL = %q", transport.spec.URL)
	}
}
