// Explore category parsing tests pin Legado's lenient array and legacy formats.
package book

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
)

func TestParseExploreKindsAcceptsLenientJSONArray(t *testing.T) {
	raw := "[{\"title\":\"排行\",\"url\":\"/rank/{{\nvar p = page;\np\n}}\"}, {'title':'分类','url':'/books','style':{layout_flexGrow:-1}},]"

	kinds, err := parseExploreKinds(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 2 || kinds[0].Title != "排行" || kinds[1].URL != "/books" || string(kinds[1].Style) != `{"layout_flexGrow":-1}` {
		t.Fatalf("kinds=%+v", kinds)
	}
}

func TestParseExploreKindsUsesLegacySeparators(t *testing.T) {
	kinds, err := parseExploreKinds("分类::/books/{{page}}\n排行::/rank&&分组")
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 3 || kinds[0].Title != "分类" || kinds[0].URL != "/books/{{page}}" || kinds[2].Title != "分组" || kinds[2].URL != "" {
		t.Fatalf("kinds=%+v", kinds)
	}
}

func TestParseExploreKindsRejectsExecutableArrayValues(t *testing.T) {
	for _, raw := range []string{
		`[{title: function () { return "hidden"; }()}]`,
		`[{title: !false}]`,
	} {
		if _, err := parseExploreKinds(raw); err == nil {
			t.Fatalf("expected executable value to be rejected: %s", raw)
		}
	}
}

func TestParseExploreKindsLoadsPinnedRawLenientFixture(t *testing.T) {
	source := pinnedExploreSource(t, 1)
	kinds, err := parseExploreKinds(source.ExploreURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 15 || kinds[0].Title != "分类" || kinds[14].Title != "完本小说" {
		t.Fatalf("raw fixture kinds=%d first=%q last=%q", len(kinds), kinds[0].Title, kinds[len(kinds)-1].Title)
	}
}

func pinnedExploreSource(t *testing.T, rawIndex int) booksource.BookSource {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "..", "testdata", "booksource", "explore-sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Fixtures []struct {
			RawIndex int             `json:"rawIndex"`
			Source   json.RawMessage `json:"source"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Fixtures {
		if fixture.RawIndex == rawIndex {
			source, err := booksource.NewFromJSON(fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			return *source
		}
	}
	t.Fatalf("raw fixture index %d missing", rawIndex)
	return booksource.BookSource{}
}
