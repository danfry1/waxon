package visual

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/image/draw"
)

// FetchAndRender downloads album art and renders it for the terminal.
// Uses Kitty graphics protocol on supported terminals (Ghostty, Kitty, WezTerm),
// falls back to high-quality half-block pixel art otherwise.
func FetchAndRender(url string, cols, rows int) string {
	if url == "" || cols == 0 || rows == 0 {
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

	// Try Kitty graphics protocol for supported terminals
	if supportsKittyGraphics() {
		result := renderKitty(img, cols, rows)
		if result != "" {
			return result
		}
	}

	// Fallback: high-quality half-block rendering
	return renderHalfBlock(img, cols, rows)
}

// supportsKittyGraphics checks if the terminal supports the Kitty graphics protocol.
func supportsKittyGraphics() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")
	return term == "xterm-kitty" ||
		termProgram == "ghostty" ||
		termProgram == "WezTerm" ||
		strings.Contains(term, "kitty")
}

// renderKitty renders the image using the Kitty graphics protocol.
// The image is displayed inline at the specified character cell dimensions.
func renderKitty(img image.Image, cols, rows int) string {
	// Resize image to reasonable pixel dimensions for display
	// Assume ~8px per column, ~16px per row
	pixW := cols * 8
	pixH := rows * 16

	// Resize with high-quality Catmull-Rom interpolation
	resized := resizeImage(img, pixW, pixH)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return ""
	}

	// Base64 encode
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Build Kitty graphics escape sequence (chunked if needed)
	var sb strings.Builder
	chunkSize := 4096
	for i := 0; i < len(b64); i += chunkSize {
		end := min(i+chunkSize, len(b64))
		chunk := b64[i:end]
		more := 0
		if end < len(b64) {
			more = 1
		}

		if i == 0 {
			// First chunk: transmit and display
			sb.WriteString(fmt.Sprintf("\x1b_Gf=100,a=T,t=d,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, chunk))
		} else {
			// Continuation chunk
			sb.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, chunk))
		}
	}

	// Reserve terminal rows (spaces for the image area)
	for range rows {
		sb.WriteString(strings.Repeat(" ", cols))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderHalfBlock renders the image using ▀ half-block characters with 24-bit color.
// Uses high-quality Catmull-Rom downscaling for the best possible pixel art.
func renderHalfBlock(img image.Image, targetW, targetH int) string {
	// Each character row represents 2 vertical pixels
	pixelH := targetH * 2

	// Resize with high-quality interpolation
	resized := resizeImage(img, targetW, pixelH)
	bounds := resized.Bounds()

	var lines []string
	for row := 0; row < bounds.Dy(); row += 2 {
		var sb strings.Builder
		for col := 0; col < bounds.Dx(); col++ {
			r1, g1, b1, _ := resized.At(bounds.Min.X+col, bounds.Min.Y+row).RGBA()
			r2, g2, b2, _ := resized.At(bounds.Min.X+col, bounds.Min.Y+row+1).RGBA()

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

// resizeImage resizes an image to the target dimensions using Catmull-Rom (high quality).
func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
