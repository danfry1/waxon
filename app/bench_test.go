package app

import (
	"image"
	"image/color"
	"testing"
)

// newTestImage creates a w x h RGBA image filled with a gradient pattern
// to exercise realistic pixel sampling in benchmarks.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 255) / max(1, w-1)),
				G: uint8((y * 255) / max(1, h-1)),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

func BenchmarkRenderHalfBlocks(b *testing.B) {
	img := newTestImage(100, 100)
	b.ResetTimer()
	for b.Loop() {
		renderHalfBlocks(img, 24, 12)
	}
}

func BenchmarkScaleBilinear(b *testing.B) {
	img := newTestImage(100, 100)
	b.ResetTimer()
	for b.Loop() {
		scaleBilinear(img, 24, 24)
	}
}

func BenchmarkRgbHex(b *testing.B) {
	for b.Loop() {
		rgbHex(0x1d, 0xb9, 0x54)
	}
}
