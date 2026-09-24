package imageproc

import "encoding/binary"

// Image bytes remain unchanged. Browsers orient JPEG APP1 and PNG eXIf images;
// report the same display axes instead of reserving the encoded matrix's shape.
// Pixel decoding/limits belong to Validate, not this metadata reader.
func imageSwapsAxes(data []byte, mediaType string) bool {
	orientation := imageOrientation(data, mediaType)
	return orientation >= 5 && orientation <= 8
}

func imageOrientation(data []byte, mediaType string) uint16 {
	var exif []byte
	if mediaType == "image/jpeg" {
		for offset := 2; offset < len(data); {
			if data[offset] != 0xff {
				break
			}
			for offset < len(data) && data[offset] == 0xff {
				offset++
			}
			if offset >= len(data) {
				break
			}
			marker := data[offset]
			offset++
			if marker == 0xda || marker == 0xd9 || len(data)-offset < 2 {
				break
			}
			length := int(binary.BigEndian.Uint16(data[offset:]))
			if length < 2 || length > len(data)-offset {
				break
			}
			payload := data[offset+2 : offset+length]
			if marker == 0xe1 && len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
				exif = payload[6:]
				break
			}
			offset += length
		}
	} else if mediaType == "image/png" {
		for offset := 8; len(data)-offset >= 12; {
			length := uint64(binary.BigEndian.Uint32(data[offset:]))
			if length > uint64(len(data)-offset-12) {
				break
			}
			if string(data[offset+4:offset+8]) == "eXIf" {
				exif = data[offset+8 : offset+8+int(length)]
				break
			}
			if string(data[offset+4:offset+8]) == "IEND" {
				break
			}
			offset += int(length) + 12
		}
	}
	orientation := exifOrientation(exif)
	if orientation < 1 || orientation > 8 {
		return 1
	}
	return orientation
}

// Only IFD0's inline SHORT orientation is relevant; do not traverse thumbnail
// directories or other EXIF values. Missing/malformed metadata has no orientation.
func exifOrientation(data []byte) uint16 {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}
	offset := uint64(order.Uint32(data[4:8]))
	if offset < 8 || offset+2 > uint64(len(data)) {
		return 1
	}
	count := uint64(order.Uint16(data[offset:]))
	offset += 2
	if count > (uint64(len(data))-offset)/12 {
		return 1
	}
	for i := uint64(0); i < count; i++ {
		entry := data[offset+i*12 : offset+(i+1)*12]
		if order.Uint16(entry[:2]) == 0x0112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
			return order.Uint16(entry[8:10])
		}
	}
	return 1
}
