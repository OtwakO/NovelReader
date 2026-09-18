package epubstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/otwako/novelreader/internal/epub"
)

const sectionStreamFile = "sections.jsonl"

// SectionSpan indexes one complete JSON record, including its trailing newline.
// Offsets are byte offsets, not character positions. Order is the section ordinal.
type SectionSpan struct{ Offset, Length int64 }

func (span SectionSpan) validFor(size int64) bool {
	return size >= 0 && size <= maxSectionTotalBytes && span.Offset >= 0 &&
		span.Length > 0 && span.Length <= maxSectionBytes &&
		span.Offset <= size && span.Length <= size-span.Offset
}

var errInvalidSectionSpan = errors.New("epubstore: invalid section span")

func appendSection(ctx context.Context, output io.Writer, section epub.PreparedSection, offset int64) (SectionSpan, error) {
	w := &outputWriter{ctx: ctx, target: output, remaining: min(maxSectionBytes, maxSectionTotalBytes-offset)}
	if err := json.NewEncoder(w).Encode(section); err != nil {
		return SectionSpan{}, err
	}
	return SectionSpan{Offset: offset, Length: w.written}, nil
}

// readSection bounds allocation and I/O to a single indexed record. ReaderAt
// avoids a shared seek cursor, allowing concurrent reads without a book cache.
// This checks the record/span contract, not portable semantic/resource validity.
func readSection(ctx context.Context, input io.ReaderAt, size int64, ordinal int, span SectionSpan) (epub.PreparedSection, error) {
	if err := ctx.Err(); err != nil {
		return epub.PreparedSection{}, err
	}
	if ordinal < 0 || !span.validFor(size) {
		return epub.PreparedSection{}, errInvalidSectionSpan
	}
	data := make([]byte, int(span.Length))
	_, err := io.ReadFull(io.NewSectionReader(contextReaderAt{ctx: ctx, input: input}, span.Offset, span.Length), data)
	if err != nil {
		return epub.PreparedSection{}, fmt.Errorf("epubstore: read section: %w", err)
	}
	var section epub.PreparedSection
	// Unmarshal rejects a second record or truncated JSON in the indexed span.
	if err = json.Unmarshal(data, &section); err != nil {
		return epub.PreparedSection{}, fmt.Errorf("epubstore: decode section: %w", err)
	}
	if section.Ordinal != ordinal {
		return epub.PreparedSection{}, errInvalidSectionSpan
	}
	return section, ctx.Err()
}

type contextReaderAt struct {
	ctx   context.Context
	input io.ReaderAt
}

func (r contextReaderAt) ReadAt(data []byte, offset int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.input.ReadAt(data, offset)
}
