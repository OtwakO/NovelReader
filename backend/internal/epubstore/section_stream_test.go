package epubstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func TestSectionStreamRandomAccess(t *testing.T) {
	sections := []epub.PreparedSection{
		{Ordinal: 0, Root: epub.Node{Kind: "paragraph", Text: "中文\nFirst section"}},
		{Ordinal: 1, Root: epub.Node{Kind: "paragraph", Text: "Second section"}, Diagnostics: []string{"synthetic"}},
	}
	var stream bytes.Buffer
	var spans []SectionSpan
	for _, section := range sections {
		span, err := appendSection(t.Context(), &stream, section, int64(stream.Len()))
		if err != nil {
			t.Fatal(err)
		}
		spans = append(spans, span)
	}
	for _, ordinal := range []int{1, 0, 1} {
		span := spans[ordinal]
		input := rangeReader{input: bytes.NewReader(stream.Bytes()), span: span}
		got, err := readSection(t.Context(), input, int64(stream.Len()), ordinal, span)
		if err != nil || !reflect.DeepEqual(got, sections[ordinal]) {
			t.Fatalf("section %d: %+v %v", ordinal, got, err)
		}
	}
}

// Fail if the reader touches any bytes outside the requested indexed record.
type rangeReader struct {
	input io.ReaderAt
	span  SectionSpan
}

func (r rangeReader) ReadAt(p []byte, offset int64) (int, error) {
	if offset < r.span.Offset || int64(len(p)) > r.span.Offset+r.span.Length-offset {
		return 0, errors.New("read outside section")
	}
	return r.input.ReadAt(p, offset)
}

func TestSectionStreamRejectsInvalidSpans(t *testing.T) {
	var stream bytes.Buffer
	span, err := appendSection(t.Context(), &stream, epub.PreparedSection{Ordinal: 0}, 0)
	if err != nil {
		t.Fatal(err)
	}
	originalSize := int64(stream.Len())
	if _, err = appendSection(t.Context(), &stream, epub.PreparedSection{Ordinal: 1}, originalSize); err != nil {
		t.Fatal(err)
	}
	size := int64(stream.Len())
	for _, bad := range []SectionSpan{
		{Offset: -1, Length: span.Length},
		{Offset: 0, Length: 0},
		{Offset: size, Length: 1},
		{Offset: 0, Length: maxSectionBytes + 1},
	} {
		if _, err = readSection(t.Context(), bytes.NewReader(stream.Bytes()), size, 0, bad); !errors.Is(err, errInvalidSectionSpan) {
			t.Fatalf("span %+v: %v", bad, err)
		}
	}
	if _, err = readSection(t.Context(), bytes.NewReader(stream.Bytes()), size, 1, span); !errors.Is(err, errInvalidSectionSpan) {
		t.Fatal("ordinal mismatch", err)
	}
	if _, err = readSection(t.Context(), bytes.NewReader(stream.Bytes()), size, 0, SectionSpan{Length: size}); err == nil {
		t.Fatal("accepted two records in one span")
	}
	if _, err = readSection(t.Context(), bytes.NewReader(stream.Bytes()[:originalSize-2]), originalSize, 0, span); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("truncated file", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = readSection(ctx, bytes.NewReader(stream.Bytes()), size, 0, span); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// Both failed writes publish no span; the owning stage discards partial bytes.
	if got, err := appendSection(t.Context(), io.Discard, epub.PreparedSection{}, maxSectionTotalBytes-1); !errors.Is(err, epub.ErrLimit) || got != (SectionSpan{}) {
		t.Fatal("aggregate limit", err)
	}
	if got, err := appendSection(t.Context(), shortWriter{}, epub.PreparedSection{}, 0); !errors.Is(err, io.ErrShortWrite) || got != (SectionSpan{}) {
		t.Fatal("partial output", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
