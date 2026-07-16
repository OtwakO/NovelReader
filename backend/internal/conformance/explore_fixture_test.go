// Guards raw Explore fixture identity and imported compatibility fields.
package conformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
)

const exploreSourceFileSHA256 = "23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c"

type exploreFixtureIdentity struct {
	url string
	sha string
}

var expectedExploreFixtures = map[int]exploreFixtureIdentity{
	1:   {url: "https://bcshuku.com/", sha: "fe69c1a83eed3cffea1354cb8bb93378a650d272aefc47f870267269767cc24c"},
	3:   {url: "https://api-x.shrtxs.cn/qd/", sha: "c04c232446f707d3162a028ed1a8047dd7c958e18c4d3bf8fd4f2f795aa8fad4"},
	7:   {url: "http://www.shukuge.com/", sha: "e10e83cf5e75b91085bb2b4857f0598f30cc40e5337304c3bf83f0673826acb8"},
	12:  {url: "https://www.sudugu.org", sha: "e18676914c5b742b2b5ccfcf6818bab33537678f76413734600e7714b0ac98d0"},
	160: {url: "https://www.missevan.com#乃星", sha: "5d2d3793f458c90b05d3794f5822de0fc4630f9c82f092828c46905be1685772"},
	184: {url: "https://newsmiss.lofter.com", sha: "dafc22fb49e562cea52314ff8e6f36bc66132797f04e552eac5cf4d310dd0da7"},
	916: {url: "https://www.bxwx.co/", sha: "3f63c01c1ab04fc5735919190e6cb19581cfc1e8ce4b2d9bdb418d36aad986f1"},
}

type exploreFixtureSet struct {
	SourceFileSHA256 string `json:"sourceFileSha256"`
	Fixtures         []struct {
		RawIndex     int             `json:"rawIndex"`
		SourceURL    string          `json:"sourceUrl"`
		SourceSHA256 string          `json:"sourceSha256"`
		Features     []string        `json:"features"`
		Source       json.RawMessage `json:"source"`
	} `json:"fixtures"`
}

func TestRawExploreFixturesKeepStableIdentityAndCoverage(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "..", "testdata", "booksource", "explore-sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtureSet exploreFixtureSet
	if err := json.Unmarshal(data, &fixtureSet); err != nil {
		t.Fatal(err)
	}
	if fixtureSet.SourceFileSHA256 != exploreSourceFileSHA256 {
		t.Fatalf("source file SHA=%q", fixtureSet.SourceFileSHA256)
	}

	required := map[string]bool{
		"html": false, "jsonpath": false, "xpath": false, "regex": false,
		"post": false, "charset": false, "javascript-categories": false,
		"webview": false, "pagination": false, "page-selector": false,
		"interactive-select": false,
	}
	if len(fixtureSet.Fixtures) != len(expectedExploreFixtures) {
		t.Fatalf("fixture count=%d want=%d", len(fixtureSet.Fixtures), len(expectedExploreFixtures))
	}
	seenIndices := make(map[int]bool, len(fixtureSet.Fixtures))
	for _, fixture := range fixtureSet.Fixtures {
		if seenIndices[fixture.RawIndex] {
			t.Errorf("duplicate raw index %d", fixture.RawIndex)
		}
		seenIndices[fixture.RawIndex] = true
		expected, ok := expectedExploreFixtures[fixture.RawIndex]
		if !ok {
			t.Errorf("unexpected raw index %d", fixture.RawIndex)
			continue
		}
		if fixture.SourceURL != expected.url || fixture.SourceSHA256 != expected.sha {
			t.Errorf("index %d metadata changed: url=%q sha=%q", fixture.RawIndex, fixture.SourceURL, fixture.SourceSHA256)
		}

		var compact bytes.Buffer
		if err := json.Compact(&compact, fixture.Source); err != nil {
			t.Errorf("index %d source JSON: %v", fixture.RawIndex, err)
			continue
		}
		hash := sha256.Sum256(compact.Bytes())
		if hex.EncodeToString(hash[:]) != expected.sha {
			t.Errorf("index %d source SHA changed", fixture.RawIndex)
		}

		source, err := booksource.NewFromJSON(fixture.Source)
		if err != nil {
			t.Errorf("index %d import: %v", fixture.RawIndex, err)
			continue
		}
		if source.BookSourceURL != fixture.SourceURL || !source.EnabledExplore || source.ExploreURL == "" {
			t.Errorf("index %d identity/eligibility mismatch: url=%q enabled=%v explore=%t", fixture.RawIndex, source.BookSourceURL, source.EnabledExplore, source.ExploreURL != "")
		}
		for _, feature := range fixture.Features {
			if _, tracked := required[feature]; tracked {
				required[feature] = true
			}
		}
	}
	for index := range expectedExploreFixtures {
		if !seenIndices[index] {
			t.Errorf("missing raw index %d", index)
		}
	}
	for feature, covered := range required {
		if !covered {
			t.Errorf("missing Explore fixture feature %q", feature)
		}
	}
}
