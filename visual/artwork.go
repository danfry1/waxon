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
// Returns the rendered string and whether Kitty protocol was used.
func FetchAndRender(url string, cols, rows int) (string, bool) {
	if url == "" || cols == 0 || rows == 0 {
		return "", false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "", false
	}

	// Kitty protocol: opt-in via SPOTUI_KITTY=1
	if os.Getenv("SPOTUI_KITTY") == "1" && supportsKittyGraphics() {
		result := renderKitty(img, cols, rows)
		if result != "" {
			return result, true
		}
	}

	// Default: quarter-block pixel art (2×2 subpixels per character cell)
	return renderQuarterBlock(img, cols, rows), false
}

// Quarter-block characters indexed by 4-bit pattern:
// bit3=TL, bit2=TR, bit1=BL, bit0=BR (1=foreground, 0=background)
var quarterBlocks = [16]string{
	" ", "▗", "▖", "▄", "▝", "▐", "▞", "▟",
	"▘", "▚", "▌", "▙", "▀", "▜", "▛", "█",
}

type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
}

func (c rgb) brightness() float64 {
	return 0.299*float64(c.r) + 0.587*float64(c.g) + 0.114*float64(c.b)
}

// renderQuarterBlock renders an image using quarter-block characters for 2×2
// subpixel resolution per character cell. This doubles horizontal resolution
// compared to half-block (▀) rendering.
func renderQuarterBlock(img image.Image, targetW, targetH int) string {
	// Each character cell = 2×2 pixels
	pixelW := targetW * 2
	pixelH := targetH * 2

	resized := resizeImage(img, pixelW, pixelH)
	bounds := resized.Bounds()

	var lines []string
	for row := 0; row < pixelH; row += 2 {
		var sb strings.Builder
		for col := 0; col < pixelW; col += 2 {
			tl := getPixel(resized, bounds.Min.X+col, bounds.Min.Y+row)
			tr := getPixel(resized, bounds.Min.X+col+1, bounds.Min.Y+row)
			bl := getPixel(resized, bounds.Min.X+col, bounds.Min.Y+row+1)
			br := getPixel(resized, bounds.Min.X+col+1, bounds.Min.Y+row+1)

			fg, bg, pattern := quantize2(tl, tr, bl, br)

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(fg.hex())).
				Background(lipgloss.Color(bg.hex()))
			sb.WriteString(style.Render(quarterBlocks[pattern]))
		}
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

// quantize2 finds the best 2-color representation of 4 pixels
// and returns fg color, bg color, and the 4-bit pattern of which
// pixels map to fg.
func quantize2(tl, tr, bl, br rgb) (fg, bg rgb, pattern uint8) {
	pixels := [4]rgb{tl, tr, bl, br}

	// Find the two most different pixels as cluster seeds
	maxDist := 0.0
	ci, cj := 0, 1
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			d := colorDist(pixels[i], pixels[j])
			if d > maxDist {
				maxDist = d
				ci, cj = i, j
			}
		}
	}

	// If all pixels are very similar, use solid block with average
	if maxDist < 200 {
		avg := avgColors(pixels[:])
		return avg, avg, 0b1111
	}

	c0 := pixels[ci]
	c1 := pixels[cj]

	// Assign each pixel to nearest seed, accumulate group averages
	var sum0, sum1 colorSum
	pattern = 0
	for i, p := range pixels {
		if colorDist(p, c0) <= colorDist(p, c1) {
			sum0.add(p)
			pattern |= 1 << uint(3-i)
		} else {
			sum1.add(p)
		}
	}

	fg = sum0.avg()
	bg = sum1.avg()

	// Ensure fg is brighter for consistent character rendering
	if fg.brightness() < bg.brightness() {
		fg, bg = bg, fg
		pattern ^= 0b1111
	}

	return fg, bg, pattern
}

func getPixel(img image.Image, x, y int) rgb {
	r, g, b, _ := img.At(x, y).RGBA()
	return rgb{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

func colorDist(a, b rgb) float64 {
	dr := float64(a.r) - float64(b.r)
	dg := float64(a.g) - float64(b.g)
	db := float64(a.b) - float64(b.b)
	return dr*dr + dg*dg + db*db
}

type colorSum struct {
	r, g, b uint32
	count   uint32
}

func (s *colorSum) add(c rgb) {
	s.r += uint32(c.r)
	s.g += uint32(c.g)
	s.b += uint32(c.b)
	s.count++
}

func (s colorSum) avg() rgb {
	if s.count == 0 {
		return rgb{}
	}
	return rgb{uint8(s.r / s.count), uint8(s.g / s.count), uint8(s.b / s.count)}
}

func avgColors(colors []rgb) rgb {
	var s colorSum
	for _, c := range colors {
		s.add(c)
	}
	return s.avg()
}

// --- Kitty graphics protocol support ---

func supportsKittyGraphics() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")
	return term == "xterm-kitty" ||
		termProgram == "ghostty" ||
		termProgram == "WezTerm" ||
		strings.Contains(term, "kitty")
}

func renderKitty(img image.Image, cols, rows int) string {
	pixW := cols * 8
	pixH := rows * 16
	resized := resizeImage(img, pixW, pixH)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return ""
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

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
			sb.WriteString(fmt.Sprintf("\x1b_Gf=100,a=T,t=d,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, chunk))
		} else {
			sb.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, chunk))
		}
	}
	return sb.String()
}

func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
