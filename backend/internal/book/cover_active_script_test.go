package book

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestActiveCoverDecodeScriptRunsWithByteInput(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test_booksource4.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	var src *booksource.BookSource
	for index := range sources {
		if sources[index].CoverDecodeJS != "" {
			src = &sources[index]
			break
		}
	}
	if src == nil {
		t.Fatal("active cover decoder source not found")
	}
	value, err := analyzer.NewJSVM().EvalContext(t.Context(), src.JSLib+"\n"+src.CoverDecodeJS, []byte("fixture-cover"), src.BookSourceURL, map[string]interface{}{
		"source": sourceContext(*src),
		"book":   bookContext(&Book{Name: "fixture", SourceURL: src.BookSourceURL}, *src),
	})
	if err != nil {
		t.Fatalf("active cover decoder: %v", err)
	}
	if value != nil {
		if decoded, err := analyzer.ToBytes(value); err != nil || len(decoded) == 0 {
			t.Fatalf("decoded=%v err=%v", decoded, err)
		}
	}
}
