package imageproc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

var (
	ErrProfile = errors.New("imageproc: invalid optimization profile")
	ErrEncode  = errors.New("imageproc: encoding failed")
)

// Profile is caller-owned policy, not a codec selector. Output is lossy 8-bit
// WebP. Limits apply to the original even when it will be downscaled.
type Profile struct {
	Limits         Limits
	MaxEdge        int
	Quality        int // 1..100
	MaxOutputBytes int64
}

// Result owns its encoded bytes; it retains neither source bytes nor pixels.
// Backend must accompany preparation evidence so portable fallback is visible.
type Result struct {
	Info
	Data    []byte
	Backend string // "native" or "portable"
}

// EncoderBackend reports the codec's immutable process-level selection. Missing
// native libraries permit portable encoding, not silent native-encode retries.
func EncoderBackend() string {
	if webp.Dynamic() == nil {
		return "native"
	}
	return "portable"
}

// Optimize validates/decodes once, never upscales, and strips source metadata.
// Standard Go decoding is not ICC/gamma color management; original mode remains
// the fidelity-preserving choice. Cancellation is observed between stages and
// on output writes, not guaranteed to interrupt a codec call immediately.
func Optimize(ctx context.Context, data []byte, mediaType string, profile Profile) (Result, error) {
	if profile.MaxEdge <= 0 || profile.Quality < 1 || profile.Quality > 100 || profile.MaxOutputBytes <= 0 {
		return Result{}, ErrProfile
	}
	img, err := decodeValidated(ctx, data, mediaType, profile.Limits)
	if err != nil {
		return Result{}, err
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if edge := max(width, height); edge > profile.MaxEdge {
		width = max(1, int(int64(width)*int64(profile.MaxEdge)/int64(edge)))
		height = max(1, int(int64(height)*int64(profile.MaxEdge)/int64(edge)))
	}
	// Resize before orientation so rotating a huge source never creates a second
	// source-sized bitmap. NRGBA avoids passing premultiplied alpha to the codec.
	pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
	if width == img.Bounds().Dx() && height == img.Bounds().Dy() {
		draw.Draw(pixels, pixels.Bounds(), img, img.Bounds().Min, draw.Src)
	} else {
		draw.CatmullRom.Scale(pixels, pixels.Bounds(), img, img.Bounds(), draw.Src, nil)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	pixels, err = orientPixels(ctx, pixels, imageOrientation(data, mediaType))
	if err != nil {
		return Result{}, err
	}
	out := boundedOutput{ctx: ctx, limit: profile.MaxOutputBytes}
	err = webp.Encode(&out, pixels, webp.Options{Quality: profile.Quality, Method: 4})
	// The binding replaces writer errors with ErrEncode. Preserve our boundary's
	// limit/cancellation cause; partial output must never escape as a success.
	if out.err != nil {
		return Result{}, out.err
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrEncode, err)
	}
	return Result{Info: Info{MediaType: "image/webp", Width: pixels.Bounds().Dx(), Height: pixels.Bounds().Dy()}, Data: out.Bytes(), Backend: EncoderBackend()}, nil
}

type boundedOutput struct {
	bytes.Buffer
	ctx   context.Context
	limit int64
	err   error
}

func (w *boundedOutput) Write(p []byte) (int, error) {
	if w.err == nil {
		w.err = w.ctx.Err()
	}
	if w.err == nil && int64(len(p)) > w.limit-int64(w.Len()) {
		w.err = ErrLimit
	}
	if w.err != nil {
		return 0, w.err
	}
	return w.Buffer.Write(p)
}
