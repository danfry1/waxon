package visual

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// FetchAndRender downloads album art and renders it as terminal pixel art.
// Designed to be called from a tea.Cmd (runs async, returns result as message).
func FetchAndRender(url string, width, height int) string {
	if url == "" {
		return ""
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return ""
	}

	return renderImage(img, width, height)
}

// renderImage converts an image to terminal art using half-block characters.
// Each character cell represents 2 vertical pixels using ▀ with fg=top, bg=bottom.
// Uses area-average downscaling for smooth, high-quality results.
func renderImage(img image.Image, targetW, targetH int) string {
	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	// targetH is character rows, each row = 2 pixels vertically
	pixelH := targetH * 2

	var lines []string
	for row := 0; row < pixelH; row += 2 {
		var sb strings.Builder
		for col := 0; col < targetW; col++ {
			// Area-average sampling for top pixel
			x1 := bounds.Min.X + col*imgW/targetW
			x2 := bounds.Min.X + (col+1)*imgW/targetW
			y1top := bounds.Min.Y + row*imgH/pixelH
			y2top := bounds.Min.Y + (row+1)*imgH/pixelH
			r1, g1, b1 := areaAverage(img, x1, y1top, x2, y2top)

			// Area-average sampling for bottom pixel
			y1bot := bounds.Min.Y + (row+1)*imgH/pixelH
			y2bot := bounds.Min.Y + (row+2)*imgH/pixelH
			r2, g2, b2 := areaAverage(img, x1, y1bot, x2, y2bot)

			topColor := fmt.Sprintf("#%02x%02x%02x", r1, g1, b1)
			botColor := fmt.Sprintf("#%02x%02x%02x", r2, g2, b2)

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(topColor)).
				Background(lipgloss.Color(botColor))
			sb.WriteString(style.Render("▀"))
		}
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

// areaAverage computes the average color of all pixels in the given rectangle.
func areaAverage(img image.Image, x1, y1, x2, y2 int) (uint8, uint8, uint8) {
	if x2 <= x1 {
		x2 = x1 + 1
	}
	if y2 <= y1 {
		y2 = y1 + 1
	}

	var rSum, gSum, bSum uint64
	var count uint64
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rSum += uint64(r >> 8)
			gSum += uint64(g >> 8)
			bSum += uint64(b >> 8)
			count++
		}
	}
	if count == 0 {
		return 0, 0, 0
	}
	return uint8(rSum / count), uint8(gSum / count), uint8(bSum / count)
}
