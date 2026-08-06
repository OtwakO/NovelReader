package book

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestActiveChapterImageDecodersHavePortableOrExplicitBitmapBoundary(t *testing.T) {
	vm := analyzer.NewJSVM()
	portable := 0
	bitmap := 0
	for _, name := range []string{"test_booksource3.json", "test_booksource4.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		var sources []booksource.BookSource
		if err := json.Unmarshal(raw, &sources); err != nil {
			t.Fatal(err)
		}
		for _, source := range sources {
			script := parseRuleJSON(source.RuleContent)["imageDecode"]
			if script == "" {
				continue
			}
			if usesAndroidBitmapDecoder(script) {
				bitmap++
				continue
			}
			portable++
			value, err := vm.EvalContext(t.Context(), decodeScript(source.JSLib, script), []byte("fixture-image"), "https://image.test/file.jpg", map[string]interface{}{
				"source": sourceContext(source),
				"book":   bookContext(&Book{Name: "fixture", SourceURL: source.BookSourceURL}, source),
				"src":    "https://image.test/file.jpg",
			})
			if err != nil && !fixtureDependentImageDecoder(script, err) {
				t.Fatalf("%s portable decoder initialization: %v", source.BookSourceName, err)
			}
			if err == nil && value != nil {
				if _, bytesErr := analyzer.ToBytes(value); bytesErr != nil {
					t.Fatalf("%s returned %T: %v", source.BookSourceName, value, bytesErr)
				}
			}
		}
	}
	if portable == 0 || bitmap == 0 {
		t.Fatalf("portable=%d bitmap=%d", portable, bitmap)
	}
}

func TestDecodeScriptSkipsRemoteLibraryMap(t *testing.T) {
	script := decodeScript(`{"library":"https://cdn.test/lib.js"}`, "result")
	if script != "result" {
		t.Fatalf("script=%q", script)
	}
}

func fixtureDependentImageDecoder(script string, err error) bool {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "syntaxerror") || strings.Contains(message, "referenceerror") || strings.Contains(message, "is not defined") {
		return false
	}
	return strings.Contains(message, "ciphertext is not block aligned") ||
		strings.Contains(message, "invalid padding") ||
		(strings.Contains(script, "cpx=") && strings.Contains(message, "cannot read"))
}
