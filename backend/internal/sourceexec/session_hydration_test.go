package sourceexec

import (
	"errors"
	"testing"
)

func TestSourceSessionHydrateOnceRetriesFailureAndPreservesMutations(t *testing.T) {
	session := NewSourceSession()
	calls := 0
	hydrate := func() error {
		calls++
		if calls == 1 {
			return errors.New("unavailable")
		}
		session.PutVariable("source", "durable")
		return nil
	}
	if err := session.HydrateOnce(hydrate); err == nil {
		t.Fatal("expected first hydration to fail")
	}
	if err := session.HydrateOnce(hydrate); err != nil {
		t.Fatal(err)
	}
	session.PutVariable("source", "workflow")
	if err := session.HydrateOnce(hydrate); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || session.GetVariable("source") != "workflow" {
		t.Fatalf("calls=%d variable=%q", calls, session.GetVariable("source"))
	}
}
