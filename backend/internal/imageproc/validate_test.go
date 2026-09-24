package imageproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

const (
	maxRasterBytes    int64 = 16 << 20
	maxImageDimension       = 16384
	maxImagePixels          = 32 << 20
)

var fixtureLimits = Limits{MaxBytes: maxRasterBytes, MaxDimension: maxImageDimension, MaxPixels: maxImagePixels}

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

func pngChunk(kind string, data []byte) []byte {
	chunk := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(data)))
	copy(chunk[4:], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	return chunk
}

func TestValidateImage(t *testing.T) {
	for _, mediaType := range []string{"image/jpeg", "image/png"} {
		data := rasterFixture(t, mediaType)
		info, err := Validate(context.Background(), data, mediaType, fixtureLimits)
		if err != nil || info != (Info{MediaType: mediaType, Width: 3, Height: 2}) {
			t.Fatalf("%s: %+v %v", mediaType, info, err)
		}
	}
	if _, err := Validate(context.Background(), rasterFixture(t, "image/png"), "image/jpeg", fixtureLimits); !errors.Is(err, ErrInvalid) {
		t.Fatalf("false declaration: %v", err)
	}
	if _, err := Validate(context.Background(), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/png", fixtureLimits); !errors.Is(err, ErrInvalid) {
		t.Fatalf("active content: %v", err)
	}
	if _, err := Validate(context.Background(), nil, "image/webp", fixtureLimits); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported media: %v", err)
	}
}

func TestValidateImageBoundaries(t *testing.T) {
	data := rasterFixture(t, "image/png")
	for _, size := range [][2]uint32{{maxImageDimension + 1, 1}, {8192, 8192}} {
		oversized := append([]byte(nil), data...)
		binary.BigEndian.PutUint32(oversized[16:], size[0])
		binary.BigEndian.PutUint32(oversized[20:], size[1])
		binary.BigEndian.PutUint32(oversized[29:], crc32.ChecksumIEEE(oversized[12:29]))
		if _, err := Validate(context.Background(), oversized, "image/png", fixtureLimits); !errors.Is(err, ErrLimit) {
			t.Fatalf("decoded bounds: %v", err)
		}
	}
	badPixels := append(append([]byte(nil), data[:33]...), pngChunk("IDAT", []byte("not a zlib stream"))...)
	badPixels = append(badPixels, pngChunk("IEND", nil)...)
	if _, err := Validate(context.Background(), badPixels, "image/png", fixtureLimits); !errors.Is(err, ErrInvalid) {
		t.Fatalf("pixel corruption: %v", err)
	}
	animated := append(append([]byte(nil), data[:33]...), pngChunk("acTL", []byte{0, 0, 0, 2, 0, 0, 0, 0})...)
	animated = append(animated, data[33:]...)
	if _, err := Validate(context.Background(), animated, "image/png", fixtureLimits); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unvalidated animation: %v", err)
	}
	if _, err := Validate(context.Background(), make([]byte, maxRasterBytes+1), "image/png", fixtureLimits); !errors.Is(err, ErrLimit) {
		t.Fatalf("encoded bounds: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Validate(ctx, data, "image/png", fixtureLimits); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
