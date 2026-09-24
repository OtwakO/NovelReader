package imageproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/image/webp"
)

// ValidateWebP validates metadata-free, static output of Optimize, not arbitrary
// publisher WebP. The portable Go decoder avoids requiring a native codec on
// restore. Dimensions are checked before allocating pixels; nothing is retained.
func ValidateWebP(ctx context.Context, data []byte, limits Limits) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	if int64(len(data)) > limits.MaxBytes {
		return Info{}, ErrLimit
	}
	if err := checkOptimizedWebP(data); err != nil {
		return Info{}, err
	}
	reader := func() io.Reader { return contextReader{ctx: ctx, source: bytes.NewReader(data)} }
	config, err := webp.DecodeConfig(reader())
	if err != nil {
		return Info{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return Info{}, ErrInvalid
	}
	if config.Width > limits.MaxDimension || config.Height > limits.MaxDimension || int64(config.Width)*int64(config.Height) > limits.MaxPixels {
		return Info{}, ErrLimit
	}
	if _, err = webp.Decode(reader()); err != nil {
		return Info{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err = ctx.Err(); err != nil {
		return Info{}, err
	}
	return Info{MediaType: "image/webp", Width: config.Width, Height: config.Height}, nil
}

func checkOptimizedWebP(data []byte) error {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return ErrInvalid
	}
	images := 0
	alpha := false
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return ErrInvalid
		}
		size := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		padded := size + (size & 1)
		if padded > uint64(len(data)-offset-8) {
			return ErrInvalid
		}
		switch string(data[offset : offset+4]) {
		case "VP8 ", "VP8L":
			images++
			if images != 1 {
				return ErrInvalid
			}
		case "ALPH":
			if alpha || images != 0 {
				return ErrInvalid
			}
			alpha = true
		case "VP8X":
			// Only transparency is produced by Optimize: no animation, EXIF, XMP, ICC.
			if offset != 12 || size != 10 || data[offset+8]&^byte(0x10) != 0 {
				return ErrUnsupported
			}
		default:
			return ErrUnsupported
		}
		offset += 8 + int(padded)
	}
	if images != 1 {
		return ErrInvalid
	}
	return nil
}
