package fingerprint

import "testing"

func TestNormalizeURLEscapesUnicodeQueryValues(t *testing.T) {
	got := normalizeURL("https://example.test/search?keyboard=凡人修仙传")
	want := "https://example.test/search?keyboard=%E5%87%A1%E4%BA%BA%E4%BF%AE%E4%BB%99%E4%BC%A0"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
