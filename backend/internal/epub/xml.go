package epub

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/unicode"
)

const (
	maxXMLDepth  = 128
	maxXMLTokens = 200000
)

// Decode through a bounded token reader, before constructing application
// records. encoding/xml does not retrieve external DTDs or expand DTD entities.
// The fixed HTML entity table accommodates ordinary EPUB 2 XHTML/NCX entities.
func decodeXML(ctx context.Context, data []byte, target any) error {
	converted := false
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		var err error
		data, err = unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder().Bytes(data)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrPackage, err)
		}
		converted = true
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		if converted && (strings.EqualFold(label, "utf-16") || strings.EqualFold(label, "utf-16le") || strings.EqualFold(label, "utf-16be")) {
			return input, nil
		}
		return charset.NewReaderLabel(label, input)
	}
	bounded := &xmlTokens{ctx: ctx, decoder: d}
	decoder := xml.NewTokenDecoder(bounded)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %w", ErrPackage, err)
	}
	// Decode alone accepts a second document after the root. Consume the tail so
	// malformed or over-budget trailing input cannot bypass the boundary.
	for {
		_, err := bounded.Token()
		if err == io.EOF {
			return ctx.Err()
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrPackage, err)
		}
	}
}

type xmlTokens struct {
	ctx                 context.Context
	decoder             *xml.Decoder
	depth, count, roots int
}

func (r *xmlTokens) Token() (xml.Token, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	t, err := r.decoder.Token()
	if err != nil {
		return nil, err
	}
	r.count++
	if r.count > maxXMLTokens {
		return nil, ErrLimit
	}
	switch token := t.(type) {
	case xml.StartElement:
		if r.depth == 0 {
			r.roots++
		}
		r.depth++
		if r.depth > maxXMLDepth {
			return nil, ErrLimit
		}
		if r.roots > 1 {
			return nil, ErrPackage
		}
	case xml.EndElement:
		r.depth--
	case xml.CharData:
		if r.depth == 0 && strings.TrimSpace(string(token)) != "" {
			return nil, ErrPackage
		}
	}
	return t, nil
}
