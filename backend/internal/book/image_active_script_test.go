package book

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestChapterImageDecodersHavePortableOrExplicitBitmapBoundary(t *testing.T) {
	cases := []struct {
		name   string
		script string
		bitmap bool
	}{
		{
			name:   "portable byte decoder",
			script: `result.slice(0, 7);`,
		},
		{
			name:   "Android bitmap decoder",
			script: `Packages.android.graphics.BitmapFactory.decodeByteArray(result, 0, result.length);`,
			bitmap: true,
		},
	}

	vm := analyzer.NewJSVM()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usesAndroidBitmapDecoder(tc.script); got != tc.bitmap {
				t.Fatalf("usesAndroidBitmapDecoder()=%v want=%v", got, tc.bitmap)
			}
			if tc.bitmap {
				return
			}

			value, err := vm.EvalContext(
				t.Context(),
				decodeScript("", tc.script),
				[]byte("fixture-image"),
				"https://image.test/file.jpg",
				nil,
			)
			if err != nil {
				t.Fatalf("portable decoder: %v", err)
			}
			decoded, err := analyzer.ToBytes(value)
			if err != nil {
				t.Fatalf("portable decoder returned %T: %v", value, err)
			}
			if string(decoded) != "fixture" {
				t.Fatalf("decoded=%q", decoded)
			}
		})
	}
}

func TestDecodeScriptSkipsRemoteLibraryMap(t *testing.T) {
	script := decodeScript(`{"library":"https://cdn.test/lib.js"}`, "result")
	if script != "result" {
		t.Fatalf("script=%q", script)
	}
}

func TestGetChapterImageDecodesInlineDataResourceWithoutFetcher(t *testing.T) {
	searcher := &Searcher{}
	source := booksource.BookSource{ID: "source", BookSourceURL: "https://source.test"}
	storedBook := &Book{BookURL: "data:;base64,Ym9vaw==,{\"type\":\"book\"}"}
	chapter := &Chapter{URL: "data:;base64,Y2hhcHRlcg==,{\"type\":\"chapter\"}"}

	cases := []struct {
		name        string
		rawURL      string
		wantData    string
		contentType string
	}{
		{name: "base64 with malformed option fragment", rawURL: "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=,{", wantData: "<svg></svg>", contentType: "image/svg+xml"},
		{name: "escaped text payload", rawURL: "data:image/svg+xml,%3Csvg%3E%3C%2Fsvg%3E", wantData: "<svg></svg>", contentType: "image/svg+xml"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, contentType, err := searcher.GetChapterImage(t.Context(), source, storedBook, chapter, test.rawURL)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != test.wantData || contentType != test.contentType {
				t.Fatalf("contentType=%q data=%q", contentType, data)
			}
		})
	}
}
