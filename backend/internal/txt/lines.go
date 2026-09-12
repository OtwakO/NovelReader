package txt

import (
	"fmt"
	"io"
)

// A line larger than a reading section is yielded in character-aligned fragments.
// Only a complete, non-continuation line can be a heading. The buffer is reused.
type textLine struct {
	start, end          int64
	text                []byte
	complete, continued bool
}

type lineReader struct {
	decoder    *decoder
	buffer     []byte
	offset     int64
	pending    int
	continuing bool
}

func (r *lineReader) next() (textLine, error) {
	r.buffer = r.buffer[:0]
	line := textLine{start: r.offset, continued: r.continuing}
	for {
		var unit []byte
		if r.pending != 0 {
			unit = r.decoder.text[:r.pending]
			r.pending = 0
		} else {
			var err error
			unit, err = r.decoder.next()
			if err != nil {
				if err != io.EOF || len(r.buffer) == 0 {
					return textLine{}, err
				}
				line.complete = true
				break
			}
		}
		if r.decoder.offset > MaxInputBytes {
			return textLine{}, fmt.Errorf("txt: input exceeds %d original bytes", MaxInputBytes)
		}
		if len(r.buffer)+len(unit) > maxSectionBytes {
			r.pending = len(unit)
			r.continuing = true
			break
		}
		r.buffer = append(r.buffer, unit...)
		r.offset = r.decoder.offset
		if len(unit) == 1 && unit[0] == '\n' {
			line.complete = true
			r.continuing = false
			break
		}
	}
	line.end, line.text = r.offset, r.buffer
	return line, nil
}
