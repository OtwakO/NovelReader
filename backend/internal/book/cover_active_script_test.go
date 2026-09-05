package book

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestCoverDecodeScriptRunsWithByteInput(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://fixture.test",
		CoverDecodeJS: `result.slice(0, 7);`,
	}
	value, err := analyzer.NewJSVM().EvalContext(
		t.Context(),
		source.CoverDecodeJS,
		[]byte("fixture-cover"),
		source.BookSourceURL,
		map[string]interface{}{
			"source": source.ScriptData(),
			"book":   bookContext(&Book{Name: "fixture", SourceURL: source.BookSourceURL}, source),
		},
	)
	if err != nil {
		t.Fatalf("cover decoder: %v", err)
	}
	decoded, err := analyzer.ToBytes(value)
	if err != nil {
		t.Fatalf("cover decoder returned %T: %v", value, err)
	}
	if string(decoded) != "fixture" {
		t.Fatalf("decoded=%q", decoded)
	}
}
