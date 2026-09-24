package epubstore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/otwako/novelreader/internal/epub"
)

const (
	maxPreparedJSONBytes = maxSectionBytes
	// XML depth is at most 128; JSON adds a children array per semantic level.
	maxPreparedJSONDepth = 320
	// Count scalars too: null entries can allocate full zero-value Go structs.
	maxPreparedJSONTokens = 250_000
)

// checkPreparedJSON limits expansion before allocating typed slices/maps. Used
// at staging and portable boundaries, not on ordinary reads of trusted output.
// Syntax and field types remain encoding/json's responsibility; this is not a
// second parser. Tokens are discarded immediately, including unknown fields.
func checkPreparedJSON(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if int64(len(data)) > maxPreparedJSONBytes {
		return epub.ErrLimit
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	depth, tokens := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		tokens++
		if tokens > maxPreparedJSONTokens {
			return epub.ErrLimit
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{', '[':
				depth++
				if depth > maxPreparedJSONDepth {
					return epub.ErrLimit
				}
			case '}', ']':
				depth--
			}
		}
	}
}
