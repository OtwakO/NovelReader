package booksource

import "testing"

func TestDefinitionIdentityPreservesUnknownFieldsButIgnoresApplicationTimestamps(t *testing.T) {
	first, err := NewFromJSON([]byte(`{"bookSourceUrl":"https://example.invalid","customScriptInput":{"a":1,"b":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFromJSON([]byte(`{"customScriptInput":{"b":2,"a":1},"bookSourceUrl":"https://example.invalid"}`))
	if err != nil {
		t.Fatal(err)
	}
	first.CreatedAt, first.UpdatedAt = 1, 2
	a, err := first.DefinitionIdentity()
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.DefinitionIdentity()
	if err != nil || a != b {
		t.Fatalf("format/timestamps changed identity: %v", err)
	}
	second.Header = `{"Referer":"https://other.invalid"}`
	b, err = second.DefinitionIdentity()
	if err != nil || a == b {
		t.Fatalf("execution edit did not change identity: %v", err)
	}
	third, err := NewFromJSON([]byte(`{"bookSourceUrl":"https://example.invalid","customScriptInput":{"a":9,"b":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := third.DefinitionIdentity()
	if err != nil || a == c {
		t.Fatalf("unknown field lost from identity: %v", err)
	}
}
