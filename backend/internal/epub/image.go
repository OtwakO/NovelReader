package epub

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
	ErrImageUnavailable = errors.New("epub: image resource unavailable")
	ErrImageUnsupported = errors.New("epub: image format unsupported")
	ErrImageInvalid     = errors.New("epub: invalid image")
)

// These are EPUB preparation budgets, independent of the BookSource network
// proxy's transfer limit. Pixel bounds apply before allocating a decoded bitmap.
const (
	maxRasterBytes    int64 = 16 << 20
	maxImageDimension       = 16384
	maxImagePixels          = 32 << 20
)

// ImageInfo dimensions describe display axes, including EXIF orientation.
// Encoded matrix dimensions are checked independently against the pixel budget.
type ImageInfo struct {
	MediaType string
	Width     int
	Height    int
}

// ValidateImage checks the declared type against a real decoder, bounds decoded
// dimensions, then validates the complete static image. It does not transform,
// cache or serve bytes. Only this fixed raster allowlist is eligible for future
// authorized image routes; archive names and HTTP sniffing are not evidence.
func ValidateImage(ctx context.Context, data []byte, declaredType string) (ImageInfo, error) {
	if err := ctx.Err(); err != nil {
		return ImageInfo{}, err
	}
	if int64(len(data)) > maxRasterBytes {
		return ImageInfo{}, ErrLimit
	}
	var config func(io.Reader) (image.Config, error)
	var decode func(io.Reader) (image.Image, error)
	switch declaredType {
	case "image/jpeg":
		config, decode = jpeg.DecodeConfig, jpeg.Decode
	case "image/png":
		config, decode = png.DecodeConfig, png.Decode
	default:
		return ImageInfo{}, ErrImageUnsupported
	}
	reader := func() io.Reader { return contextReader{ctx: ctx, source: bytes.NewReader(data)} }
	info, err := config(reader())
	if ctx.Err() != nil {
		return ImageInfo{}, ctx.Err()
	}
	if err != nil {
		return ImageInfo{}, fmt.Errorf("%w: %w", ErrImageInvalid, err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		return ImageInfo{}, ErrImageInvalid
	}
	if info.Width > maxImageDimension || info.Height > maxImageDimension || int64(info.Width)*int64(info.Height) > maxImagePixels {
		return ImageInfo{}, ErrLimit
	}
	if declaredType == "image/png" {
		if err := checkStaticPNG(data); err != nil {
			return ImageInfo{}, err
		}
	}
	// DecodeConfig alone accepts a header with corrupt or missing pixel data.
	// Do not use DecodeAll: animation is outside this static-image profile.
	if _, err = decode(reader()); err != nil {
		if ctx.Err() != nil {
			return ImageInfo{}, ctx.Err()
		}
		return ImageInfo{}, fmt.Errorf("%w: %w", ErrImageInvalid, err)
	}
	if err := ctx.Err(); err != nil {
		return ImageInfo{}, err
	}
	if imageSwapsAxes(data, declaredType) {
		info.Width, info.Height = info.Height, info.Width
	}
	return ImageInfo{MediaType: declaredType, Width: info.Width, Height: info.Height}, nil
}

// The standard PNG decoder reads the default image of APNG but does not validate
// its other frames. Reject declared animation rather than accidentally serving
// unvalidated frames. This scans framing only; png.Decode validates pixel/CRC data.
func checkStaticPNG(data []byte) error {
	for offset := 8; offset < len(data); {
		if len(data)-offset < 12 {
			return ErrImageInvalid
		}
		length := uint64(binary.BigEndian.Uint32(data[offset:]))
		if length > uint64(len(data)-offset-12) {
			return ErrImageInvalid
		}
		kind := string(data[offset+4 : offset+8])
		if kind == "acTL" {
			return ErrImageUnsupported
		}
		if kind == "IEND" {
			return nil
		}
		offset += int(length) + 12
	}
	return ErrImageInvalid
}
