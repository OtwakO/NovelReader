package epub

import (
	"context"

	"github.com/otwako/novelreader/internal/imageproc"
)

// ValidatePreparedImage checks portable delivery evidence without trusting the
// database's MIME/dimensions or exposing archive paths to presentation code.
func ValidatePreparedImage(image PreparedImage, mode ImageMode) error {
	if !validEntryName(image.Reference.Path) || image.Reference.Fragment != "" {
		return ErrReference
	}
	info := image.Info
	if info.Width <= 0 || info.Height <= 0 || info.Width > maxImageDimension || info.Height > maxImageDimension || int64(info.Width)*int64(info.Height) > maxImagePixels {
		return ErrImageInvalid
	}
	switch mode {
	case OriginalImages:
		if image.DerivativeID != "" || (info.MediaType != "image/jpeg" && info.MediaType != "image/png") {
			return ErrImagePolicy
		}
	case OptimizedImages:
		if image.DerivativeID == "" || info.MediaType != "image/webp" || info.Width > optimizedImageMaxEdge || info.Height > optimizedImageMaxEdge {
			return ErrImagePolicy
		}
	default:
		return ErrImagePolicy
	}
	return nil
}

// ValidatePreparedImageBytes verifies actual pixels once at the portable boundary,
// never on ordinary reading. Storage owns acquisition and derivative byte budgets.
func ValidatePreparedImageBytes(ctx context.Context, data []byte, image PreparedImage, mode ImageMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidatePreparedImage(image, mode); err != nil {
		return err
	}
	var info ImageInfo
	var err error
	if mode == OriginalImages {
		info, err = ValidateImage(ctx, data, image.Info.MediaType)
	} else {
		info, err = imageproc.ValidateWebP(ctx, data, imageproc.Limits{MaxBytes: maxRasterBytes, MaxDimension: optimizedImageMaxEdge, MaxPixels: int64(optimizedImageMaxEdge) * optimizedImageMaxEdge})
		err = imageError(err)
	}
	if err != nil {
		return err
	}
	if info != image.Info {
		return ErrImageInvalid
	}
	return nil
}
