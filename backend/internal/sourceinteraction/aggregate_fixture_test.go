package sourceinteraction

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func TestDescribeUnmodifiedAggregateFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../test-booksources/test_光遇聚合_aggregated_booksource.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	source := sources[0]
	source.ID = "aggregate-fixture"
	describer := NewDescriber(describerSourceStore{&source}, describerProfileStore{sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`)}}, analyzer.NewJSVM())
	view, err := describer.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Controls) != 34 {
		t.Fatalf("controls=%d", len(view.Controls))
	}
	for _, control := range view.Controls {
		if control.Unsupported != "" {
			t.Fatalf("unsupported control=%+v", control)
		}
	}
}
