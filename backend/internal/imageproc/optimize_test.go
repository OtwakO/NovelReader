package imageproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"golang.org/x/image/webp"
)

var testProfile = Profile{Limits: fixtureLimits, MaxEdge: 2048, Quality: 92, MaxOutputBytes: 1 << 20}

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOptimizeDimensionsAndOwnership(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ width, height, edge, wantWidth, wantHeight int }{
		{3, 2, 8, 3, 2}, {6, 4, 3, 3, 2}, {4, 6, 3, 2, 3}, {10, 1, 2, 2, 1},
	} {
		source := image.NewNRGBA64(image.Rect(0, 0, tc.width, tc.height))
		for y := 0; y < tc.height; y++ {
			for x := 0; x < tc.width; x++ {
				source.SetNRGBA64(x, y, color.NRGBA64{R: 0xcccc, G: 0x7777, B: 0x3333, A: 0x8080})
			}
		}
		data := encodePNG(t, source)
		original := bytes.Clone(data)
		profile := testProfile
		profile.MaxEdge = tc.edge
		result, err := Optimize(context.Background(), data, "image/png", profile)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := webp.Decode(bytes.NewReader(result.Data))
		if err != nil {
			t.Fatal(err)
		}
		if result.Info != (Info{MediaType: "image/webp", Width: tc.wantWidth, Height: tc.wantHeight}) || decoded.Bounds().Dx() != result.Width || decoded.Bounds().Dy() != result.Height {
			t.Fatalf("dimensions: %+v %v", result.Info, decoded.Bounds())
		}
		if color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA).A != 128 {
			t.Fatal("16-bit alpha not preserved as 8-bit")
		}
		if result.Backend != EncoderBackend() || !bytes.Equal(data, original) {
			t.Fatal("backend or input ownership")
		}
	}
}

func TestOptimizeOrientationAndAlpha(t *testing.T) {
	t.Parallel()
	source := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i, alpha := range []uint8{0, 40, 80, 120, 160, 200} {
		source.SetNRGBA(i%3, i/3, color.NRGBA{R: 80, G: 100, B: 120, A: alpha})
	}
	original := encodePNG(t, source)
	expected := [][]uint8{
		{0, 40, 80, 120, 160, 200}, {80, 40, 0, 200, 160, 120},
		{200, 160, 120, 80, 40, 0}, {120, 160, 200, 0, 40, 80},
		{0, 120, 40, 160, 80, 200}, {120, 0, 160, 40, 200, 80},
		{200, 80, 160, 40, 120, 0}, {80, 200, 40, 160, 0, 120},
	}
	for i, want := range expected {
		exif := pngChunk("eXIf", orientationFixture(binary.LittleEndian, uint16(i+1)))
		data := append(append(bytes.Clone(original[:33]), exif...), original[33:]...)
		result, err := Optimize(context.Background(), data, "image/png", testProfile)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := webp.Decode(bytes.NewReader(result.Data))
		if err != nil {
			t.Fatal(err)
		}
		var got []uint8
		for y := 0; y < decoded.Bounds().Dy(); y++ {
			for x := 0; x < decoded.Bounds().Dx(); x++ {
				got = append(got, color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA).A)
			}
		}
		if !bytes.Equal(got, want) || bytes.Contains(result.Data, []byte("EXIF")) {
			t.Fatalf("orientation %d: %v want %v", i+1, got, want)
		}
	}
}

func TestOptimizeFailureBoundaries(t *testing.T) {
	data := rasterFixture(t, "image/png")
	profile := testProfile
	profile.MaxOutputBytes = 1
	result, err := Optimize(context.Background(), data, "image/png", profile)
	if !errors.Is(err, ErrLimit) || len(result.Data) != 0 {
		t.Fatalf("partial encoded output: %v", err)
	}
	profile = testProfile
	profile.Limits.MaxPixels = 1
	if _, err := Optimize(context.Background(), data, "image/png", profile); !errors.Is(err, ErrLimit) {
		t.Fatalf("source limit bypass: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Optimize(ctx, data, "image/png", testProfile); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	writer := boundedOutput{ctx: ctx, limit: 1024}
	if _, err := writer.Write([]byte("x")); !errors.Is(err, context.Canceled) || writer.Len() != 0 {
		t.Fatal("canceled writer retained output")
	}
	profile = testProfile
	profile.Quality = 0
	if _, err := Optimize(context.Background(), data, "image/png", profile); !errors.Is(err, ErrProfile) {
		t.Fatal(err)
	}
}

// Used by the final container stage, where only runtime libraries are installed.
func TestNativeEncoderRequirement(t *testing.T) {
	if os.Getenv("NOVELREADER_TEST_REQUIRE_NATIVE_WEBP") == "1" && EncoderBackend() != "native" {
		t.Fatal("release image must provide native WebP")
	}
}
