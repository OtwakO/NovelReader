package txt

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

var errInvalidUTF8 = errors.New("invalid UTF-8")

func legacyEncoding(enc Encoding) encoding.Encoding {
	switch enc {
	case GB18030:
		return simplifiedchinese.GB18030
	case Big5:
		return traditionalchinese.Big5
	default:
		return nil
	}
}

func validEncoding(enc Encoding) bool {
	return enc == UTF8 || enc == UTF16LE || enc == UTF16BE || legacyEncoding(enc) != nil
}

func resolveEncoding(reader *bufio.Reader, requested Encoding) (Encoding, int64, error) {
	if requested != "" && !validEncoding(requested) {
		return "", 0, fmt.Errorf("txt: unsupported encoding %q", requested)
	}
	prefix, err := reader.Peek(4)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", 0, fmt.Errorf("txt: read encoding prefix: %w", err)
	}
	var bom Encoding
	var skip int
	switch {
	case bytes.HasPrefix(prefix, []byte{0xff, 0xfe, 0, 0}), bytes.HasPrefix(prefix, []byte{0, 0, 0xfe, 0xff}):
		return "", 0, fmt.Errorf("txt: UTF-32 is not supported")
	case bytes.HasPrefix(prefix, []byte{0xef, 0xbb, 0xbf}):
		bom, skip = UTF8, 3
	case bytes.HasPrefix(prefix, []byte{0xff, 0xfe}):
		bom, skip = UTF16LE, 2
	case bytes.HasPrefix(prefix, []byte{0xfe, 0xff}):
		bom, skip = UTF16BE, 2
	}
	if bom != "" {
		if requested != "" && requested != bom {
			return "", 0, fmt.Errorf("txt: encoding %s conflicts with %s BOM", requested, bom)
		}
		if _, err := reader.Discard(skip); err != nil {
			return "", 0, fmt.Errorf("txt: skip BOM: %w", err)
		}
		return bom, int64(skip), nil
	}
	if requested == UTF16LE || requested == UTF16BE {
		return "", 0, fmt.Errorf("txt: UTF-16 requires a BOM")
	}
	if requested != "" {
		return requested, 0, nil
	}

	// Do not guess between legacy encodings from their overlapping byte ranges.
	// A sample suggests UTF-8; the complete streaming pass still validates it.
	sample, err := reader.Peek(encodingSampleBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", 0, fmt.Errorf("txt: sample encoding: %w", err)
	}
	for len(sample) > 0 {
		// A full sample may end inside a character, unlike the actual EOF.
		if err == nil && !utf8.FullRune(sample) {
			break
		}
		r, width := utf8.DecodeRune(sample)
		if r == utf8.RuneError && width == 1 {
			return "", 0, ErrEncodingRequired
		}
		sample = sample[width:]
	}
	return UTF8, 0, nil
}
