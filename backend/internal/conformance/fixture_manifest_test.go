// Verifies the checked-in deterministic fixture corpus stays complete.
package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
)

type fixtureManifest struct {
	Fixtures []struct {
		Name     string `json:"name"`
		File     string `json:"file"`
		Kind     string `json:"kind"`
		Rule     string `json:"rule"`
		Expected string `json:"expected"`
	} `json:"fixtures"`
}

func TestFixtureManifestCoversRequiredBooksourceContracts(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	fixtureDir := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "testdata", "booksource")
	manifestBytes, err := os.ReadFile(filepath.Join(fixtureDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}

	requiredKinds := []string{
		"search", "detail", "toc", "content", "json", "xpath", "regex", "js-post",
		"post-gbk", "cookie", "pagination", "webview",
	}
	present := make(map[string]bool, len(requiredKinds))
	for _, kind := range requiredKinds {
		present[kind] = false
	}
	names := make(map[string]bool, len(manifest.Fixtures))
	files := make(map[string]bool, len(manifest.Fixtures))
	for _, fixture := range manifest.Fixtures {
		if names[fixture.Name] {
			t.Errorf("duplicate fixture name %q", fixture.Name)
		}
		if files[fixture.File] {
			t.Errorf("duplicate fixture file %q", fixture.File)
		}
		names[fixture.Name], files[fixture.File] = true, true
		if _, exists := present[fixture.Kind]; exists {
			present[fixture.Kind] = true
		} else {
			t.Errorf("unknown fixture kind %q", fixture.Kind)
		}
		if fixture.Rule == "" {
			t.Errorf("fixture %q has no executable rule", fixture.Name)
		}
		if filepath.IsAbs(fixture.File) || strings.HasPrefix(filepath.Clean(fixture.File), "..") {
			t.Errorf("fixture %q escapes fixture directory", fixture.Name)
			continue
		}
		body, err := os.ReadFile(filepath.Join(fixtureDir, fixture.File))
		if err != nil {
			t.Errorf("fixture %q: %v", fixture.Name, err)
			continue
		}
		if len(body) == 0 {
			t.Errorf("fixture %q is empty", fixture.Name)
			continue
		}
		if fixture.Kind == "webview" {
			if fixture.Rule != "webView:true" || !strings.Contains(string(body), `id="app"`) || fixture.Expected != "" {
				t.Errorf("fixture %q has invalid WebView contract", fixture.Name)
			}
			continue
		}
		value, err := analyzer.New(string(body), "https://fixture.test/", analyzer.NewJSVM(), nil).GetString(fixture.Rule)
		if err != nil {
			t.Errorf("fixture %q rule %q: %v", fixture.Name, fixture.Rule, err)
			continue
		}
		if value != fixture.Expected {
			t.Errorf("fixture %q value=%q want=%q", fixture.Name, value, fixture.Expected)
		}
	}
	for _, kind := range requiredKinds {
		if !present[kind] {
			t.Errorf("missing required fixture kind %q", kind)
		}
	}
}
