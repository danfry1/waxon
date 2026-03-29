package visual

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// ArtworkCache caches downloaded and rendered artwork to avoid re-fetching.
type ArtworkCache struct {
	mu         sync.Mutex
	url        string
	rendered   string
	renderedW  int
	renderedH  int
}

// RenderArtwork returns the album art rendered as colored half-block characters.
// Uses ▀ (upper half block) with foreground = top pixel, background = bottom pixel,
// giving 2x vertical resolution. Returns empty string if artwork unavailable.
func (ac *ArtworkCache) RenderArtwork(url string, width, height int) string {
	if url == "" {
		return ""
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Return cached version if same URL and size
	if ac.url == url && ac.renderedW == width && ac.renderedH == height && ac.rendered != "" {
		return ac.rendered
	}

	// Download and render
	img, err := fetchImage(url)
	if err != nil {
		return ""
	}

	ac.rendered = renderImage(img, width, height)
	ac.url = url
	ac.renderedW = width
	ac.renderedH = height
	return ac.rendered
}

func fetchImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// renderImage converts an image to terminal art using half-block characters.
// Each character cell represents 2 vertical pixels using ▀ with fg=top, bg=bottom.
func renderImage(img image.Image, targetW, targetH int) string {
	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	// targetH is in character rows, each row = 2 pixels
	pixelH := targetH * 2

	var lines []string
	for row := 0; row < pixelH; row += 2 {
		var sb strings.Builder
		for col := 0; col < targetW; col++ {
			// Map terminal position to image position
			srcX := bounds.Min.X + col*imgW/targetW
			srcY := bounds.Min.Y + row*imgH/pixelH
			srcY2 := bounds.Min.Y + (row+1)*imgH/pixelH

			// Get top and bottom pixel colors
			r1, g1, b1, _ := img.At(srcX, srcY).RGBA()
			r2, g2, b2, _ := img.At(srcX, srcY2).RGBA()

			// RGBA returns 16-bit values, convert to 8-bit
			topColor := fmt.Sprintf("#%02x%02x%02x", r1>>8, g1>>8, b1>>8)
			botColor := fmt.Sprintf("#%02x%02x%02x", r2>>8, g2>>8, b2>>8)

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(topColor)).
				Background(lipgloss.Color(botColor))
			sb.WriteString(style.Render("▀"))
		}
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}
