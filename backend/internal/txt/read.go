package txt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// ReadSection decodes only an indexed range. The caller supplies the immutable
// original and resolved encoding belonging to that index; this function does not
// locate publications, validate revisions, or convert text to a Reading Document.
func ReadSection(ctx context.Context, original io.ReaderAt, enc Encoding, section Section) (string, error) {
	if !validEncoding(enc) {
		return "", fmt.Errorf("txt: unsupported encoding %q", enc)
	}
	if section.Start < 0 || section.End <= section.Start || section.End-section.Start > maxSectionSourceBytes {
		return "", fmt.Errorf("txt: invalid section range [%d, %d)", section.Start, section.End)
	}
	length := section.End - section.Start
	reader := bufio.NewReader(io.NewSectionReader(original, section.Start, length))
	decoder := newDecoder(ctx, reader, enc, section.Start)
	var text strings.Builder
	for {
		unit, err := decoder.next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if text.Len()+len(unit) > maxSectionBytes {
			return "", fmt.Errorf("txt: section exceeds %d decoded bytes", maxSectionBytes)
		}
		text.Write(unit)
	}
	if decoder.offset != section.End {
		return "", fmt.Errorf("txt: truncated section: %w", io.ErrUnexpectedEOF)
	}
	return text.String(), nil
}
