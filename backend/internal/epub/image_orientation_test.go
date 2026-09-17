package epub

import (
	"context"
	"encoding/binary"
	"testing"
)

func orientationFixture(order binary.ByteOrder, orientation uint16) []byte {
	data := make([]byte, 26)
	copy(data, "II")
	if order == binary.BigEndian {
		copy(data, "MM")
	}
	order.PutUint16(data[2:], 42)
	order.PutUint32(data[4:], 8)
	order.PutUint16(data[8:], 1)
	order.PutUint16(data[10:], 0x0112)
	order.PutUint16(data[12:], 3)
	order.PutUint32(data[14:], 1)
	order.PutUint16(data[18:], orientation)
	return data
}

func TestValidateImageDisplayDimensions(t *testing.T) {
	for _, mediaType := range []string{"image/jpeg", "image/png"} {
		for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
			for orientation := uint16(1); orientation <= 8; orientation++ {
				original := rasterFixture(t, mediaType)
				exif := orientationFixture(order, orientation)
				var data []byte
				if mediaType == "image/jpeg" {
					segment := []byte{0xff, 0xe1, 0, byte(2 + 6 + len(exif)), 'E', 'x', 'i', 'f', 0, 0}
					data = append(append(append([]byte{}, original[:2]...), segment...), exif...)
					data = append(data, original[2:]...)
				} else {
					data = append(append([]byte{}, original[:33]...), pngChunk("eXIf", exif)...)
					data = append(data, original[33:]...)
				}
				info, err := ValidateImage(context.Background(), data, mediaType)
				width, height := 3, 2
				if orientation >= 5 {
					width, height = height, width
				}
				if err != nil || info.Width != width || info.Height != height {
					t.Fatalf("%s orientation %d: %+v, %v", mediaType, orientation, info, err)
				}
			}
		}
	}
}

func TestOrientationMetadataBounds(t *testing.T) {
	data := orientationFixture(binary.LittleEndian, 6)
	for _, malformed := range [][]byte{nil, data[:8], data[:19]} {
		if exifOrientation(malformed) != 1 {
			t.Fatal("truncated metadata should have no orientation")
		}
	}
	binary.LittleEndian.PutUint32(data[4:], 0xffffffff)
	if exifOrientation(data) != 1 {
		t.Fatal("out-of-range directory should have no orientation")
	}
}
