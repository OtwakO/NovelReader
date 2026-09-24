package txt

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
)

func TestAnalyzeCharacterBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		codec     encoding.Encoding
		requested Encoding
		unit      string
	}{
		{"utf8-sample-boundary", nil, "", "🌿"},
		{"utf16le-surrogates", textunicode.UTF16(textunicode.LittleEndian, textunicode.UseBOM), "", "🌿"},
		{"utf16be-surrogates", textunicode.UTF16(textunicode.BigEndian, textunicode.UseBOM), "", "🌿"},
		{"gb18030-four-bytes", simplifiedchinese.GB18030, GB18030, "🌿"},
		{"big5-two-runes", nil, Big5, "\u00ca\u0304"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The first character crosses the encoding sample boundary and later
			// characters cross generated-section boundaries. Keep indivisible
			// original units intact, including Big5's two-rune mappings.
			prefix := strings.Repeat("x", encodingSampleBytes-1)
			text := prefix + strings.Repeat(tc.unit, maxSectionBytes/2)
			raw := []byte(text)
			if tc.codec != nil {
				var err error
				raw, err = tc.codec.NewEncoder().Bytes(raw)
				if err != nil {
					t.Fatal(err)
				}
			} else if tc.requested == Big5 {
				raw = append([]byte(prefix), bytes.Repeat([]byte{0x88, 0x62}, maxSectionBytes/2)...)
			}
			analysis, err := Analyze(t.Context(), bytes.NewReader(raw), Options{Encoding: tc.requested})
			if err != nil {
				t.Fatal(err)
			}
			if len(analysis.Sections) < 2 {
				t.Fatal("expected bounded sections")
			}
			if got := readAllSections(t, raw, analysis); got != text {
				t.Fatal("character boundary changed decoded content")
			}
		})
	}
	// A literal replacement character is data, unlike a decoder substitution.
	raw, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("a\ufffdb"))
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := Analyze(t.Context(), bytes.NewReader(raw), Options{Encoding: GB18030})
	if err != nil {
		t.Fatal(err)
	}
	if got := readAllSections(t, raw, analysis); got != "a\ufffdb" {
		t.Fatalf("literal replacement character: %q", got)
	}
}

func BenchmarkAnalyze(b *testing.B) {
	text := []byte(strings.Repeat("第一章 山水\n"+strings.Repeat("山水文字。\n", 1000)+"第二章 歸來\n", 64))
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := Analyze(b.Context(), bytes.NewReader(text), Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
