package epub

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
		info, err := ValidateImage(context.Background(), data, mediaType)
		if err != nil || info != (ImageInfo{MediaType: mediaType, Width: 3, Height: 2}) {
			t.Fatalf("%s: %+v %v", mediaType, info, err)
		}
	}
	if _, err := ValidateImage(context.Background(), rasterFixture(t, "image/png"), "image/jpeg"); !errors.Is(err, ErrImageInvalid) {
		t.Fatalf("false declaration: %v", err)
	}
	if _, err := ValidateImage(context.Background(), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/png"); !errors.Is(err, ErrImageInvalid) {
		t.Fatalf("active content: %v", err)
	}
	if _, err := ValidateImage(context.Background(), nil, "image/webp"); !errors.Is(err, ErrImageUnsupported) {
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
		if _, err := ValidateImage(context.Background(), oversized, "image/png"); !errors.Is(err, ErrLimit) {
			t.Fatalf("decoded bounds: %v", err)
		}
	}
	badPixels := append(append([]byte(nil), data[:33]...), pngChunk("IDAT", []byte("not a zlib stream"))...)
	badPixels = append(badPixels, pngChunk("IEND", nil)...)
	if _, err := ValidateImage(context.Background(), badPixels, "image/png"); !errors.Is(err, ErrImageInvalid) {
		t.Fatalf("pixel corruption: %v", err)
	}
	animated := append(append([]byte(nil), data[:33]...), pngChunk("acTL", []byte{0, 0, 0, 2, 0, 0, 0, 0})...)
	animated = append(animated, data[33:]...)
	if _, err := ValidateImage(context.Background(), animated, "image/png"); !errors.Is(err, ErrImageUnsupported) {
		t.Fatalf("unvalidated animation: %v", err)
	}
	if _, err := ValidateImage(context.Background(), make([]byte, maxRasterBytes+1), "image/png"); !errors.Is(err, ErrLimit) {
		t.Fatalf("encoded bounds: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ValidateImage(ctx, data, "image/png"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
