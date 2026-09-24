package epub

import (
	"context"
	"errors"
	"fmt"

	"github.com/otwako/novelreader/internal/imageproc"
)

var (
	ErrImageUnavailable = errors.New("epub: image resource unavailable")
	ErrImageUnsupported = errors.New("epub: image format unsupported")
	ErrImageInvalid     = errors.New("epub: invalid image")
)

// EPUB owns admission budgets, not the shared pixel-processing module.
const (
	maxRasterBytes    int64 = 16 << 20
	maxImageDimension       = 16384
	maxImagePixels          = 32 << 20
)

type ImageInfo = imageproc.Info

// ValidateImage preserves EPUB's admission/error contract while the shared
// module owns raster validation and orientation. No image bytes are retained.
func ValidateImage(ctx context.Context, data []byte, declaredType string) (ImageInfo, error) {
	info, err := imageproc.Validate(ctx, data, declaredType, rasterLimits())
	return info, imageError(err)
}

func rasterLimits() imageproc.Limits {
	return imageproc.Limits{MaxBytes: maxRasterBytes, MaxDimension: maxImageDimension, MaxPixels: maxImagePixels}
}

func imageError(err error) error {
	switch {
	case errors.Is(err, imageproc.ErrLimit):
		return ErrLimit
	case errors.Is(err, imageproc.ErrUnsupported):
		return ErrImageUnsupported
	case errors.Is(err, imageproc.ErrInvalid):
		return fmt.Errorf("%w: %w", ErrImageInvalid, err)
	default:
		return err
	}
}
