package reading

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// Shared with the frontend candidate parser: actual preparation and projection,
// not a separately handwritten approximation of the reading wire contract.
func TestPreparedEPUBWireContract(t *testing.T) {
	prepared, sections := prepareReadingEPUB(t)
	contents := make([]Content, len(sections))
	for index, section := range sections {
		content, err := epubContent(context.Background(), 9, prepared.Sections[index].Title, section, func(key string) string {
			return fmt.Sprintf("/api/books/proof/chapters/%d/resources/%s?contentRevision=9", index, key)
		})
		if err != nil {
			t.Fatal(err)
		}
		contents[index] = content
	}
	wire := struct {
		Catalog  Catalog   `json:"catalog"`
		Contents []Content `json:"contents"`
	}{epubCatalog(9, prepared), contents}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/epub-reading.json")
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prepared EPUB projection differs from shared wire fixture: %s", encoded)
	}
}
