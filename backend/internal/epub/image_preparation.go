package epub

import (
	"context"
	"errors"
	"fmt"

	"github.com/otwako/novelreader/internal/imageproc"
)

type ImageMode string

const (
	OriginalImages        ImageMode = "original"
	OptimizedImages       ImageMode = "optimized"
	optimizedImageProfile           = "webp-q92-edge2048-v1"
	optimizedImageMaxEdge           = 2048
)

var (
	ErrImagePolicy = errors.New("epub: invalid image preparation policy")
	ErrImageOutput = errors.New("epub: derivative output failed")
)

// ImageOptions is copied per preparation. Zero means original images. Storage
// owns output budgets and identifiers; no filesystem path enters preparation.
// Emit must synchronously stage/copy bytes and return a nonempty attempt-local
// resource ID. On ANY Prepare error the caller must discard ALL staged output.
type ImageOptions struct {
	Mode                    ImageMode
	MaxDerivativeBytes      int64
	MaxTotalDerivativeBytes int64
	Emit                    func(context.Context, Reference, ImageInfo, []byte) (string, error)
}

// ImageProcessing is private preparation evidence, separate from content-review
// diagnostics. Backend is empty until an image is actually optimized; portable
// encoding requires a performance notice at the eventual import UI boundary.
type ImageProcessing struct {
	Mode            ImageMode
	Profile         string
	Backend         string
	DerivativeCount int
	DerivativeBytes int64
}

func validateImageOptions(options ImageOptions) error {
	switch options.Mode {
	case "", OriginalImages:
		return nil
	case OptimizedImages:
		if options.Emit != nil && options.MaxDerivativeBytes > 0 && options.MaxTotalDerivativeBytes > 0 {
			return nil
		}
	}
	return ErrImagePolicy
}

func (r *imageResolver) prepareImage(ctx context.Context, ref Reference, data []byte, mediaType string) (imageResult, error) {
	if r.options.Mode != OptimizedImages {
		info, err := ValidateImage(ctx, data, mediaType)
		return imageResult{info: info}, err
	}
	remaining := r.options.MaxTotalDerivativeBytes - r.processing.DerivativeBytes
	if remaining <= 0 {
		return imageResult{}, ErrLimit
	}
	result, err := imageproc.Optimize(ctx, data, mediaType, imageproc.Profile{
		Limits: rasterLimits(), MaxEdge: optimizedImageMaxEdge, Quality: 92,
		MaxOutputBytes: min(r.options.MaxDerivativeBytes, remaining),
	})
	if err != nil {
		return imageResult{}, imageError(err)
	}
	id, err := r.options.Emit(ctx, ref, result.Info, result.Data)
	if err != nil {
		return imageResult{}, fmt.Errorf("%w: %w", ErrImageOutput, err)
	}
	if err := ctx.Err(); err != nil {
		return imageResult{}, err
	}
	if id == "" {
		return imageResult{}, ErrImageOutput
	}
	r.processing.Backend = result.Backend
	r.processing.DerivativeCount++
	r.processing.DerivativeBytes += int64(len(result.Data))
	return imageResult{info: result.Info, derivativeID: id}, nil
}

func (r *imageResolver) preparedImage(ref Reference) PreparedImage {
	result := r.cache[ref.Path]
	return PreparedImage{Reference: ref, Info: result.info, DerivativeID: result.derivativeID}
}
