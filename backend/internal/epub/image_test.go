package epub

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func rasterFixture(t testing.TB, mediaType string) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	var err error
	if mediaType == "image/png" {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestValidateImagePreservesEPUBContract(t *testing.T) {
	data := rasterFixture(t, "image/png")
	original := bytes.Clone(data)
	info, err := ValidateImage(context.Background(), data, "image/png")
	if err != nil || info != (ImageInfo{MediaType: "image/png", Width: 3, Height: 2}) || !bytes.Equal(data, original) {
		t.Fatalf("original validation: %+v %v", info, err)
	}
	for _, tc := range []struct {
		name  string
		data  []byte
		media string
		want  error
	}{
		{"invalid", data, "image/jpeg", ErrImageInvalid},
		{"unsupported", data, "image/webp", ErrImageUnsupported},
		{"limit", make([]byte, maxRasterBytes+1), "image/png", ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateImage(context.Background(), tc.data, tc.media); !errors.Is(err, tc.want) {
				t.Fatalf("error classification: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ValidateImage(ctx, data, "image/png"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
