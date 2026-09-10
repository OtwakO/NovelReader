package txt

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	textunicode "golang.org/x/text/encoding/unicode"
)

func TestAnalyzeEncodings(t *testing.T) {
	const text = "前言\r\n第一章 開始\r\n山水。\r\n第二章 歸來\n終。"
	for _, tc := range []struct {
		name      string
		codec     encoding.Encoding
		requested Encoding
		resolved  Encoding
		bom       int64
	}{
		{"utf8", nil, "", UTF8, 0},
		{"utf8-bom", textunicode.UTF8BOM, "", UTF8, 3},
		{"utf16le", textunicode.UTF16(textunicode.LittleEndian, textunicode.UseBOM), "", UTF16LE, 2},
		{"utf16be", textunicode.UTF16(textunicode.BigEndian, textunicode.UseBOM), "", UTF16BE, 2},
		{"gb18030", simplifiedchinese.GB18030, GB18030, GB18030, 0},
		{"big5", traditionalchinese.Big5, Big5, Big5, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(text)
			if tc.codec != nil {
				var err error
				raw, err = tc.codec.NewEncoder().Bytes(raw)
				if err != nil {
					t.Fatal(err)
				}
			}
			analysis, err := Analyze(t.Context(), bytes.NewReader(raw), Options{Encoding: tc.requested})
			if err != nil {
				t.Fatal(err)
			}
			if analysis.Encoding != tc.resolved || analysis.Preset != ChineseChapters || len(analysis.Sections) != 3 || len(analysis.ReviewReasons) != 0 {
				t.Fatalf("unexpected analysis: %+v", analysis)
			}
			if analysis.Sections[0].Start != tc.bom || analysis.Sections[1].Title != "第一章 開始" {
				t.Fatalf("lost BOM offset or chapter title: %+v", analysis.Sections)
			}
			if got := readAllSections(t, raw, analysis); got != text {
				t.Fatalf("content changed: %q", got)
			}
		})
	}
}

func TestAnalyzeGeneratedSections(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("山", maxSectionBytes), // One unusually long paragraph; split only at character boundaries.
		strings.Repeat(strings.Repeat("x", targetSectionBytes)+"\n\n", 3),
		"Chapter 1 Start\n" + strings.Repeat("text\n", maxSectionBytes) + "Chapter 2 End\nfinish",
	} {
		analysis, err := Analyze(t.Context(), strings.NewReader(text), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(analysis.Sections) < 3 {
			t.Fatalf("unbounded sections: %+v", analysis)
		}
		if got := readAllSections(t, []byte(text), analysis); got != text {
			t.Fatal("generated divisions lost or duplicated content")
		}
		if !analysis.Sections[0].Generated {
			t.Fatal("generated divisions must be disclosed")
		}
	}
	text := strings.Repeat("x", targetSectionBytes) + "\n\n" + strings.Repeat("y", targetSectionBytes)
	analysis, err := Analyze(t.Context(), strings.NewReader(text), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Sections[0].End != int64(targetSectionBytes+2) {
		t.Fatal("did not prefer the paragraph boundary")
	}
}

func TestAnalyzeReviewAndPreset(t *testing.T) {
	for _, tc := range []struct {
		text   string
		preset Preset
		reason ReviewReason
	}{
		{"plain text", "", NoHeadings},
		{"Chapter 1 Only\ntext", "", FewHeadings},
		{"第一章 甲\ntext\nChapter 1 One\ntext\n第二章 乙\ntext\nChapter 2 Two\ntext", "", AmbiguousHeadings},
		{"Chapter 1 One\ntext\nChapter 2 Two\ntext", ChineseChapters, NoHeadings},
	} {
		analysis, err := Analyze(t.Context(), strings.NewReader(tc.text), Options{Preset: tc.preset})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, reason := range analysis.ReviewReasons {
			found = found || reason == tc.reason
		}
		if !found {
			t.Fatalf("missing %s: %+v", tc.reason, analysis)
		}
	}
}

func TestAnalyzeRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		raw     []byte
		options Options
	}{
		{nil, Options{}},
		{[]byte(" \n\t"), Options{}},
		{[]byte("text\x00more"), Options{}},
		{[]byte{0xff, 0xfe, 0, 0xd8}, Options{}}, // Unpaired UTF-16 surrogate.
		{[]byte{0xff, 0xfe, 'a'}, Options{}},
		{[]byte{'a', 0x81}, Options{Encoding: GB18030}},
		{[]byte{'a', 0x81}, Options{Encoding: Big5}},
		{[]byte{0xff, 0xfe, 'a', 0}, Options{Encoding: UTF8}},
		{[]byte{'a', 0}, Options{Encoding: UTF16LE}}, // No BOM.
		{[]byte("text"), Options{Encoding: "unsupported"}},
		{[]byte("text"), Options{Preset: "unsupported"}},
	} {
		analysis, err := Analyze(t.Context(), bytes.NewReader(tc.raw), tc.options)
		if err == nil || len(analysis.Sections) != 0 {
			t.Fatalf("accepted invalid input: %x: %+v, %v", tc.raw[:min(len(tc.raw), 8)], analysis, err)
		}
	}
	raw, err := traditionalchinese.Big5.NewEncoder().Bytes([]byte("山水"))
	if err != nil {
		t.Fatal(err)
	}
	for _, uncertain := range [][]byte{raw, append(bytes.Repeat([]byte("a"), encodingSampleBytes+8), raw...)} {
		if analysis, err := Analyze(t.Context(), bytes.NewReader(uncertain), Options{}); !errors.Is(err, ErrEncodingRequired) || len(analysis.Sections) != 0 {
			t.Fatalf("ambiguous encoding must request selection, even after an ASCII sample: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Analyze(ctx, strings.NewReader("text"), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	failure := errors.New("read failed")
	if _, err := Analyze(t.Context(), failingReader{failure}, Options{}); !errors.Is(err, failure) {
		t.Fatalf("lost I/O failure: %v", err)
	}
}

func TestReadSectionBounds(t *testing.T) {
	raw := []byte(strings.Repeat("before", 10000) + "target" + strings.Repeat("after", 10000))
	reader := &countedReaderAt{Reader: bytes.NewReader(raw)}
	section := Section{Start: 60000, End: 60006}
	text, err := ReadSection(t.Context(), reader, UTF8, section)
	if err != nil || text != "target" || reader.bytes != 6 {
		t.Fatalf("range read: %q, bytes=%d, error=%v", text, reader.bytes, err)
	}
	for _, invalid := range []Section{{Start: -1, End: 2}, {Start: 1, End: 1}, {End: maxSectionSourceBytes + 1}, {Start: int64(len(raw)), End: int64(len(raw)) + 1}} {
		if _, err := ReadSection(t.Context(), reader, UTF8, invalid); err == nil {
			t.Fatalf("accepted range: %+v", invalid)
		}
	}
}

func readAllSections(t *testing.T, raw []byte, analysis Analysis) string {
	t.Helper()
	var result strings.Builder
	for i, section := range analysis.Sections {
		if i > 0 && section.Start != analysis.Sections[i-1].End {
			t.Fatal("non-contiguous ranges")
		}
		text, err := ReadSection(t.Context(), bytes.NewReader(raw), analysis.Encoding, section)
		if err != nil {
			t.Fatal(err)
		}
		if len(text) > maxSectionBytes {
			t.Fatalf("oversized section: %d", len(text))
		}
		result.WriteString(text)
	}
	if analysis.Sections[len(analysis.Sections)-1].End != int64(len(raw)) {
		t.Fatal("lost EOF")
	}
	return result.String()
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type countedReaderAt struct {
	*bytes.Reader
	bytes int
}

func (r *countedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.Reader.ReadAt(p, off)
	r.bytes += n
	return n, err
}
