//go:build ignore

// Generate installable PWA icons from the canonical NovelReader logo.
// Run from frontend/: go run scripts/generate-pwa-icons.go
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const logoPath = "assets/branding/novelreader-logo.png"

func loadLogo() image.Image {
	file, err := os.Open(logoPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	logo, err := png.Decode(file)
	if err != nil {
		panic(err)
	}
	return logo
}

func rgba(value color.Color) (float64, float64, float64, float64) {
	r, g, b, a := value.RGBA()
	return float64(r), float64(g), float64(b), float64(a)
}

func blend(a, b float64, weight float64) float64 {
	return a + (b-a)*weight
}

func sample(source image.Image, x, y float64) color.RGBA {
	bounds := source.Bounds()
	x = math.Max(0, math.Min(x, float64(bounds.Dx()-1)))
	y = math.Max(0, math.Min(y, float64(bounds.Dy()-1)))
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, bounds.Dx()-1), min(y0+1, bounds.Dy()-1)
	tx, ty := x-float64(x0), y-float64(y0)

	r00, g00, b00, a00 := rgba(source.At(bounds.Min.X+x0, bounds.Min.Y+y0))
	r10, g10, b10, a10 := rgba(source.At(bounds.Min.X+x1, bounds.Min.Y+y0))
	r01, g01, b01, a01 := rgba(source.At(bounds.Min.X+x0, bounds.Min.Y+y1))
	r11, g11, b11, a11 := rgba(source.At(bounds.Min.X+x1, bounds.Min.Y+y1))

	channel := func(c00, c10, c01, c11 float64) uint8 {
		top := blend(c00, c10, tx)
		bottom := blend(c01, c11, tx)
		return uint8(math.Round(blend(top, bottom, ty) / 257))
	}
	return color.RGBA{
		R: channel(r00, r10, r01, r11),
		G: channel(g00, g10, g01, g11),
		B: channel(b00, b10, b01, b11),
		A: channel(a00, a10, a01, a11),
	}
}

func resize(source image.Image, size int) *image.RGBA {
	output := image.NewRGBA(image.Rect(0, 0, size, size))
	bounds := source.Bounds()
	for y := 0; y < size; y++ {
		sourceY := (float64(y)+0.5)*float64(bounds.Dy())/float64(size) - 0.5
		for x := 0; x < size; x++ {
			sourceX := (float64(x)+0.5)*float64(bounds.Dx())/float64(size) - 0.5
			output.SetRGBA(x, y, sample(source, sourceX, sourceY))
		}
	}
	return output
}

func maskable(source image.Image, size int) *image.RGBA {
	output := image.NewRGBA(image.Rect(0, 0, size, size))
	background := source.At(source.Bounds().Min.X, source.Bounds().Min.Y)
	draw.Draw(output, output.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)

	const safeScale = 0.8
	logoSize := int(math.Round(float64(size) * safeScale))
	logo := resize(source, logoSize)
	offset := (size - logoSize) / 2
	draw.Draw(output, image.Rect(offset, offset, offset+logoSize, offset+logoSize), logo, image.Point{}, draw.Src)
	return output
}

func writePNG(path string, img image.Image) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		panic(err)
	}
}

func main() {
	logo := loadLogo()
	writePNG("public/icons/icon-192.png", resize(logo, 192))
	writePNG("public/icons/icon-512.png", resize(logo, 512))
	writePNG("public/icons/icon-maskable-512.png", maskable(logo, 512))
}
