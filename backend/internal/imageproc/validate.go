// Package imageproc validates and optimizes bounded static raster images.
// Callers own input acquisition, policy, storage, and lifecycle.
package imageproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

var (
	ErrUnsupported = errors.New("imageproc: image format unsupported")
	ErrInvalid     = errors.New("imageproc: invalid image")
	ErrLimit       = errors.New("imageproc: image limit exceeded")
)

// Limits are caller-owned policy. All fields must be positive; zero permits no
// images. Encoded and pixel bounds are checked before allocating a bitmap.
type Limits struct {
	MaxBytes     int64
	MaxDimension int
	MaxPixels    int64
}

// Info describes display axes, including EXIF orientation. Encoded matrix
// dimensions are checked independently against the pixel budget.
type Info struct {
	MediaType string
	Width     int
	Height    int
}

func Validate(ctx context.Context, data []byte, declaredType string, limits Limits) (Info, error) {
	img, err := decodeValidated(ctx, data, declaredType, limits)
	if err != nil {
		return Info{}, err
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if imageSwapsAxes(data, declaredType) {
		width, height = height, width
	}
	return Info{MediaType: declaredType, Width: width, Height: height}, nil
}

// Both public operations use this boundary; optimization must not decode twice.
func decodeValidated(ctx context.Context, data []byte, declaredType string, limits Limits) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(data)) > limits.MaxBytes {
		return nil, ErrLimit
	}
	var config func(io.Reader) (image.Config, error)
	var decode func(io.Reader) (image.Image, error)
	switch declaredType {
	case "image/jpeg":
		config, decode = jpeg.DecodeConfig, jpeg.Decode
	case "image/png":
		config, decode = png.DecodeConfig, png.Decode
	default:
		return nil, ErrUnsupported
	}
	reader := func() io.Reader { return contextReader{ctx: ctx, source: bytes.NewReader(data)} }
	info, err := config(reader())
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		return nil, ErrInvalid
	}
	if info.Width > limits.MaxDimension || info.Height > limits.MaxDimension || int64(info.Width)*int64(info.Height) > limits.MaxPixels {
		return nil, ErrLimit
	}
	if declaredType == "image/png" {
		if err := checkStaticPNG(data); err != nil {
			return nil, err
		}
	}
	img, err := decode(reader())
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return img, nil
}

// PNG's standard decoder validates only the default frame. Reject APNG rather
// than accepting other unvalidated frames. The decoder validates pixel/CRC data.
func checkStaticPNG(data []byte) error {
	for offset := 8; offset < len(data); {
		if len(data)-offset < 12 {
			return ErrInvalid
		}
		length := uint64(binary.BigEndian.Uint32(data[offset:]))
		if length > uint64(len(data)-offset-12) {
			return ErrInvalid
		}
		kind := string(data[offset+4 : offset+8])
		if kind == "acTL" {
			return ErrUnsupported
		}
		if kind == "IEND" {
			return nil
		}
		offset += int(length) + 12
	}
	return ErrInvalid
}

type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}
