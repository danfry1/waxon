//go:build darwin

package media

import (
	"image"
	"image/color"
	"math"
)

// recordImage draws a size×size black record with faint grooves and a green
// centre label, the artwork shown before a track's cover has loaded.
func recordImage(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{0x18, 0x18, 0x18, 0xff}
	label := color.RGBA{0x1d, 0xb9, 0x54, 0xff}
	c := float64(size) / 2
	outer, lbl, hole := c*0.92, c*0.34, c*0.04
	for y := range size {
		for x := range size {
			r := math.Hypot(float64(x)+0.5-c, float64(y)+0.5-c)
			px := bg
			switch {
			case r < hole:
			case r < lbl:
				px = label
			case r < outer:
				// Grooves: a slow ripple of brightness across the vinyl.
				v := uint8(0x0c + 6*(1+math.Sin(r*1.4)))
				px = color.RGBA{v, v, v, 0xff}
			}
			img.SetRGBA(x, y, px)
		}
	}
	return img
}
