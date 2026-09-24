package imageproc

import (
	"context"
	"image"
)

// Input is already capped and 8-bit. No full-source-sized rotated copy, image
// cache or pool is retained. Orientation 1 needs no additional pixel buffer.
func orientPixels(ctx context.Context, src *image.NRGBA, orientation uint16) (*image.NRGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if orientation == 1 {
		return src, nil
	}
	width, height := src.Bounds().Dx(), src.Bounds().Dy()
	bounds := src.Bounds()
	if orientation >= 5 {
		bounds = image.Rect(0, 0, height, width)
	}
	dst := image.NewNRGBA(bounds)
	for y := 0; y < height; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < width; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = width - 1 - x
			case 3:
				dx, dy = width-1-x, height-1-y
			case 4:
				dy = height - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = height-1-y, x
			case 7:
				dx, dy = height-1-y, width-1-x
			case 8:
				dx, dy = y, width-1-x
			}
			dst.SetNRGBA(dx, dy, src.NRGBAAt(x, y))
		}
	}
	return dst, nil
}
