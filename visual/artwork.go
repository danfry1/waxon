package visual

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/image/draw"
)

// ArtworkResult holds the rendered artwork and extracted color palette.
type ArtworkResult struct {
	Rendered   string
	IsKitty    bool
	Primary    string // dominant vibrant color from the art
	Secondary  string // contrasting color from the art
	Background string // dark version for terminal background
	HasColors  bool   // true if color extraction succeeded
}

// FetchAndRender downloads album art, renders it, and extracts dominant colors.
func FetchAndRender(url string, cols, rows int) ArtworkResult {
	if url == "" || cols == 0 || rows == 0 {
		return ArtworkResult{}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ArtworkResult{}
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return ArtworkResult{}
	}

	// Extract dominant colors from the art
	result := ArtworkResult{}
	result.Primary, result.Secondary, result.Background, result.HasColors = extractColors(img)

	// Render the image
	if os.Getenv("SPOTUI_KITTY") == "1" && supportsKittyGraphics() {
		rendered := renderKitty(img, cols, rows)
		if rendered != "" {
			result.Rendered = rendered
			result.IsKitty = true
			return result
		}
	}

	result.Rendered = renderQuarterBlock(img, cols, rows)
	return result
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

// extractColors finds the dominant vibrant color, a contrasting color,
// and a dark background from the album art. Makes every song feel unique.
func extractColors(img image.Image) (primary, secondary, background string, ok bool) {
	small := resizeImage(img, 12, 12)
	bounds := small.Bounds()

	var pixels []rgb
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixels = append(pixels, getPixel(small, x, y))
		}
	}
	if len(pixels) == 0 {
		return "", "", "", false
	}

	// Find the most vibrant color → primary
	bestVib := 0.0
	var priColor rgb
	for _, p := range pixels {
		v := vibrancy(p)
		if v > bestVib {
			bestVib = v
			priColor = p
		}
	}

	// If the art is very desaturated, don't override mood colors
	if bestVib < 0.08 {
		return "", "", "", false
	}

	// Ensure primary is bright enough to read as text
	if priColor.brightness() < 120 {
		priColor = boostBrightness(priColor, 140)
	}

	// Find the most different vibrant color → secondary
	bestDist := 0.0
	var secColor rgb
	for _, p := range pixels {
		d := colorDist(p, priColor)
		if d > bestDist && vibrancy(p) > 0.03 {
			bestDist = d
			secColor = p
		}
	}
	// If secondary is too similar to primary, desaturate primary slightly
	if colorDist(priColor, secColor) < 2000 {
		secColor = rgb{
			uint8(float64(priColor.r)*0.6 + 40),
			uint8(float64(priColor.g)*0.6 + 40),
			uint8(float64(priColor.b)*0.6 + 40),
		}
	}

	// Background: very dark tint of primary
	bgColor := rgb{
		uint8(math.Min(float64(priColor.r)*0.08+5, 30)),
		uint8(math.Min(float64(priColor.g)*0.08+5, 30)),
		uint8(math.Min(float64(priColor.b)*0.08+5, 30)),
	}

	return priColor.hex(), secColor.hex(), bgColor.hex(), true
}

func vibrancy(c rgb) float64 {
	max := math.Max(float64(c.r), math.Max(float64(c.g), float64(c.b)))
	min := math.Min(float64(c.r), math.Min(float64(c.g), float64(c.b)))
	if max == 0 {
		return 0
	}
	saturation := (max - min) / max
	brightness := max / 255.0
	return saturation * brightness
}

func boostBrightness(c rgb, target float64) rgb {
	b := c.brightness()
	if b == 0 {
		return rgb{uint8(target), uint8(target), uint8(target)}
	}
	factor := target / b
	return rgb{
		uint8(math.Min(float64(c.r)*factor, 255)),
		uint8(math.Min(float64(c.g)*factor, 255)),
		uint8(math.Min(float64(c.b)*factor, 255)),
	}
}

func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
