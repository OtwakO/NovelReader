package book

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
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
