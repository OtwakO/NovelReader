package txt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
)

// Decode one original character unit at a time so byte offsets are exact. Big5
// has units that expand to two Unicode runes; keep those units indivisible.
// Scratch buffers and the legacy decoder are reused, not allocated per rune.
type decoder struct {
	ctx      context.Context
	reader   *bufio.Reader
	encoding Encoding
	offset   int64
	legacy   *encoding.Decoder
	raw      [4]byte
	text     [8]byte
}

func newDecoder(ctx context.Context, reader *bufio.Reader, enc Encoding, offset int64) *decoder {
	d := &decoder{ctx: ctx, reader: reader, encoding: enc, offset: offset}
	if codec := legacyEncoding(enc); codec != nil {
		d.legacy = codec.NewDecoder()
	}
	return d
}

func (d *decoder) next() ([]byte, error) {
	if err := d.ctx.Err(); err != nil {
		return nil, err
	}
	text, width, err := d.character()
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("txt: %s decode at byte %d: %w", d.encoding, d.offset, err)
	}
	if len(text) == 1 && (text[0] < 32 && text[0] != '\n' && text[0] != '\r' && text[0] != '\t' && text[0] != '\f' || text[0] == 127) {
		return nil, fmt.Errorf("%w at byte %d", ErrNonText, d.offset)
	}
	d.offset += int64(width)
	return text, nil
}

func (d *decoder) character() ([]byte, int, error) {
	if d.encoding == UTF8 {
		r, width, err := d.reader.ReadRune()
		if err != nil {
			return nil, 0, err
		}
		if r == utf8.RuneError && width == 1 {
			return nil, 0, errInvalidUTF8
		}
		return utf8.AppendRune(d.text[:0], r), width, nil
	}
	if d.encoding == UTF16LE || d.encoding == UTF16BE {
		return d.utf16Character()
	}
	first, err := d.reader.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	d.raw[0] = first
	width := 1
	if first >= 0x81 && first <= 0xfe {
		width = 2
		if _, err := io.ReadFull(d.reader, d.raw[1:2]); err != nil {
			return nil, 0, ioFailure(err)
		}
		if d.encoding == GB18030 && d.raw[1] >= '0' && d.raw[1] <= '9' {
			width = 4
			if _, err := io.ReadFull(d.reader, d.raw[2:4]); err != nil {
				return nil, 0, ioFailure(err)
			}
		}
	}
	n, consumed, err := d.legacy.Transform(d.text[:], d.raw[:width], true)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrInvalidEncoding, err)
	}
	if consumed != width {
		return nil, 0, fmt.Errorf("%w: incomplete character", ErrInvalidEncoding)
	}
	text := d.text[:n]
	// x/text replaces malformed legacy sequences instead of returning an error.
	// Reject substitutions, but allow an actually encoded U+FFFD to round-trip.
	if bytes.Contains(text, []byte("\ufffd")) {
		roundtrip, err := legacyEncoding(d.encoding).NewEncoder().Bytes(text)
		if err != nil || !bytes.Equal(roundtrip, d.raw[:width]) {
			return nil, 0, ErrInvalidEncoding
		}
	}
	return text, width, nil
}

func (d *decoder) utf16Character() ([]byte, int, error) {
	if _, err := io.ReadFull(d.reader, d.raw[:2]); err != nil {
		if err == io.EOF {
			return nil, 0, err
		}
		return nil, 0, ioFailure(err)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if d.encoding == UTF16BE {
		order = binary.BigEndian
	}
	first := order.Uint16(d.raw[:2])
	r, width := rune(first), 2
	if first >= 0xd800 && first <= 0xdbff {
		if _, err := io.ReadFull(d.reader, d.raw[2:]); err != nil {
			return nil, 0, ioFailure(err)
		}
		second := order.Uint16(d.raw[2:])
		if second < 0xdc00 || second > 0xdfff {
			return nil, 0, fmt.Errorf("%w: unpaired UTF-16 surrogate", ErrInvalidEncoding)
		}
		r, width = utf16.DecodeRune(rune(first), rune(second)), 4
	} else if first >= 0xdc00 && first <= 0xdfff {
		return nil, 0, fmt.Errorf("%w: unpaired UTF-16 surrogate", ErrInvalidEncoding)
	}
	return utf8.AppendRune(d.text[:0], r), width, nil
}

func ioFailure(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return fmt.Errorf("%w: %w", ErrInvalidEncoding, io.ErrUnexpectedEOF)
	}
	return err
}
