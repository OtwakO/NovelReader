// Regression tests for JSONPath decoding and complete list extraction.
package analyzer

import (
	"encoding/json"
	"testing"
)

func TestJSONPathDecodesObjectsBeforeSelection(t *testing.T) {
	a := New(`{"books":[{"name":"凡人修仙传"}]}`, "https://example.test/", NewJSVM(), nil)
	value, err := a.GetString(`@Json:$.books[0].name`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "凡人修仙传" {
		t.Fatalf("value=%q", value)
	}
}

func TestJSONPathElementsAreNotCappedAtFifty(t *testing.T) {
	books := make([]map[string]int, 75)
	for i := range books {
		books[i] = map[string]int{"index": i}
	}
	content, err := json.Marshal(map[string]interface{}{"books": books})
	if err != nil {
		t.Fatal(err)
	}
	a := New(string(content), "https://example.test/", NewJSVM(), nil)
	elements, err := a.GetElements(`$.books[*]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != len(books) {
		t.Fatalf("elements=%d want=%d", len(elements), len(books))
	}
}
