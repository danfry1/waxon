package app

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// httpImageClient is a dedicated HTTP client for image downloads with a
// reasonable timeout to prevent hanging on slow servers.
var httpImageClient = &http.Client{Timeout: 10 * time.Second}

// maxImageBytes limits the size of downloaded images to prevent OOM from
// malicious or misconfigured URLs. 10 MB is generous for album art.
const maxImageBytes = 10 << 20

const (
	ArtWidth    = 24 // columns for now-playing art
	ArtHeight   = 12 // terminal rows (= 24 pixel rows via half-blocks)
	MinTermRows = 28 // below this, hide art
	HeaderArtW  = 8  // columns for tracklist header art
	HeaderArtH  = 4  // terminal rows for tracklist header art
)

// AlbumArt renders album artwork using Unicode half-block characters.
type AlbumArt struct {
	rendered   string
	currentURL string
	width      int
	height     int
}

func NewAlbumArt() AlbumArt {
	return AlbumArt{width: ArtWidth, height: ArtHeight}
}

func (a *AlbumArt) SetImage(img image.Image) {
	a.rendered = renderHalfBlocks(img, a.width, a.height)
}

func (a *AlbumArt) Clear() {
	a.rendered = ""
	a.currentURL = ""
}

func (a *AlbumArt) CurrentURL() string {
	return a.currentURL
}

func (a *AlbumArt) SetURL(url string) {
	a.currentURL = url
}

func (a AlbumArt) View() string {
	return a.rendered
}

// FetchImage downloads and decodes an image from a URL.
// Uses a dedicated client with timeout and a body size limit to prevent
// hangs and OOM from slow or malicious servers.
func FetchImage(ctx context.Context, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpImageClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxImageBytes)
	img, _, err := image.Decode(limited)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// renderHalfBlocks converts an image to a string using Unicode half-block characters.
// Each terminal row encodes 2 pixel rows: foreground = top pixel, background = bottom pixel.
// Uses direct ANSI escape codes for performance (avoids per-pixel lipgloss allocation).
func renderHalfBlocks(img image.Image, w, h int) string {
	if w == 0 || h == 0 {
		return ""
	}
	pixH := h * 2
	scaled := scaleBilinear(img, w, pixH)

	// Pre-allocate: ~30 bytes per cell (ANSI fg + bg + char) + newlines
	var sb strings.Builder
	sb.Grow(w * h * 30)
	for row := 0; row < pixH; row += 2 {
		for col := range w {
			top := scaled[row][col]
			bot := scaled[row+1][col]
			sb.WriteString("\x1b[38;2;")
			sb.WriteString(decLUT[top.R])
			sb.WriteByte(';')
			sb.WriteString(decLUT[top.G])
			sb.WriteByte(';')
			sb.WriteString(decLUT[top.B])
			sb.WriteString("m\x1b[48;2;")
			sb.WriteString(decLUT[bot.R])
			sb.WriteByte(';')
			sb.WriteString(decLUT[bot.G])
			sb.WriteByte(';')
			sb.WriteString(decLUT[bot.B])
			sb.WriteString("m▀")
		}
		sb.WriteString("\x1b[0m")
		if row+2 < pixH {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

type rgb struct{ R, G, B uint8 }

// hexLUT is a pre-computed lookup table mapping uint8 values to their 2-char
// hex representation. Eliminates fmt.Sprintf calls in the hot rendering path.
var hexLUT [256]string

// decLUT maps uint8 values to their decimal string representation,
// eliminating strconv/fmt calls in the hot ANSI rendering path.
var decLUT [256]string

func init() {
	for i := range 256 {
		hexLUT[i] = fmt.Sprintf("%02x", i)
		decLUT[i] = strconv.Itoa(i)
	}
}

// rgbHex returns the "#rrggbb" hex string for the given color.
func rgbHex(r, g, b uint8) string {
	return "#" + hexLUT[r] + hexLUT[g] + hexLUT[b]
}

// scaleBilinear scales an image to w x h using bilinear interpolation for smooth results.
func scaleBilinear(img image.Image, w, h int) [][]rgb {
	bounds := img.Bounds()
	srcW := float64(bounds.Dx())
	srcH := float64(bounds.Dy())
	minX := bounds.Min.X
	minY := bounds.Min.Y
	maxX := bounds.Max.X - 1
	maxY := bounds.Max.Y - 1

	flat := make([]rgb, w*h)
	pixels := make([][]rgb, h)
	for y := range h {
		pixels[y] = flat[y*w : (y+1)*w]
		srcYf := float64(y) * srcH / float64(h)
		sy := int(srcYf)
		fy := srcYf - float64(sy)
		y0 := clampInt(minY+sy, minY, maxY)
		y1 := clampInt(minY+sy+1, minY, maxY)

		for x := range w {
			srcXf := float64(x) * srcW / float64(w)
			sx := int(srcXf)
			fx := srcXf - float64(sx)
			x0 := clampInt(minX+sx, minX, maxX)
			x1 := clampInt(minX+sx+1, minX, maxX)

			// Sample 4 surrounding pixels
			r00, g00, b00, _ := img.At(x0, y0).RGBA()
			r10, g10, b10, _ := img.At(x1, y0).RGBA()
			r01, g01, b01, _ := img.At(x0, y1).RGBA()
			r11, g11, b11, _ := img.At(x1, y1).RGBA()

			// Bilinear blend
			pixels[y][x] = rgb{
				R: blerp(r00, r10, r01, r11, fx, fy),
				G: blerp(g00, g10, g01, g11, fx, fy),
				B: blerp(b00, b10, b01, b11, fx, fy),
			}
		}
	}
	return pixels
}

func blerp(c00, c10, c01, c11 uint32, fx, fy float64) uint8 {
	// All values are in 0..0xFFFF range from RGBA()
	top := float64(c00)*(1-fx) + float64(c10)*fx
	bot := float64(c01)*(1-fx) + float64(c11)*fx
	v := top*(1-fy) + bot*fy
	// Shift right by 8 and clamp to prevent uint8 overflow wrapping
	// (e.g. 65535/256 rounds to 256 which wraps to 0, causing color speckles)
	r := int(v) >> 8
	if r > 255 {
		r = 255
	}
	return uint8(r)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// DominantColor samples pixels from an image and returns the most vibrant
// color as a lipgloss hex Color. It skips near-black and near-white pixels
// to find a representative accent color. Returns "" if no suitable color is found.
func DominantColor(img image.Image) lipgloss.Color {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w == 0 || h == 0 {
		return ""
	}

	// Sample ~100 evenly spaced pixels (10x10 grid)
	stepX := max(1, w/10)
	stepY := max(1, h/10)

	type colorBucket struct {
		r, g, b    uint64
		count      int
		saturation float64
	}
	var best colorBucket

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			rr, gg, bb, _ := img.At(x, y).RGBA()
			// Convert from 0..0xFFFF to 0..255
			r8 := float64(rr >> 8)
			g8 := float64(gg >> 8)
			b8 := float64(bb >> 8)

			// Skip near-black (too dark for accent)
			lum := 0.299*r8 + 0.587*g8 + 0.114*b8
			if lum < 30 {
				continue
			}
			// Skip near-white (too washed out)
			if lum > 220 {
				continue
			}

			// Compute saturation: (max - min) / max
			mx := math.Max(r8, math.Max(g8, b8))
			mn := math.Min(r8, math.Min(g8, b8))
			sat := 0.0
			if mx > 0 {
				sat = (mx - mn) / mx
			}

			// Prefer saturated, reasonably bright colors
			score := sat * (lum / 255.0)
			bestScore := best.saturation
			if score > bestScore {
				best = colorBucket{
					r: uint64(r8), g: uint64(g8), b: uint64(b8),
					count:      1,
					saturation: score,
				}
			}
		}
	}

	if best.count == 0 {
		return ""
	}

	// Validate: minimum saturation threshold to avoid dull grays
	if best.saturation < 0.08 {
		return ""
	}

	return lipgloss.Color(rgbHex(uint8(best.r), uint8(best.g), uint8(best.b)))
}

// renderVinyl renders album art as a spinning vinyl record using bilinear
// interpolation for smooth rotation. The image is masked to a circle with
// a center hole and subtle groove rings.
// bgRows provides per-terminal-row background colors for outside-circle pixels.
// If nil, defaults to near-black.
func renderVinyl(img image.Image, angle float64, w, h int, bgRows []rgb) string {
	if w == 0 || h == 0 || img == nil {
		return ""
	}
	pixH := h * 2
	bounds := img.Bounds()
	srcW := float64(bounds.Dx())
	srcH := float64(bounds.Dy())
	srcCX := float64(bounds.Min.X) + srcW/2
	srcCY := float64(bounds.Min.Y) + srcH/2
	minX := bounds.Min.X
	minY := bounds.Min.Y
	maxX := bounds.Max.X - 1
	maxY := bounds.Max.Y - 1

	cosA := math.Cos(angle)
	sinA := math.Sin(angle)

	cx := float64(w) / 2
	cy := float64(pixH) / 2
	radius := math.Min(cx, cy)
	holeRadius := radius * 0.08
	grooveInterval := radius * 0.12

	defaultBg := rgb{R: 25, G: 20, B: 20}

	flat := make([]rgb, w*pixH)
	pixels := make([][]rgb, pixH)
	for y := range pixH {
		pixels[y] = flat[y*w : (y+1)*w]
		// Pick background color for this pixel row (2 pixel rows per terminal row)
		bgColor := defaultBg
		if bgRows != nil {
			termRow := y / 2
			if termRow < len(bgRows) {
				bgColor = bgRows[termRow]
			}
		}
		for x := range w {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist > radius || dist < holeRadius {
				pixels[y][x] = bgColor
				continue
			}

			// Inverse rotation to find source coordinates
			rotX := dx*cosA + dy*sinA
			rotY := -dx*sinA + dy*cosA

			srcXf := srcCX + rotX*srcW/float64(w)
			srcYf := srcCY + rotY*srcH/float64(pixH)

			// Bilinear interpolation for smooth rotation
			sx := int(srcXf)
			sy := int(srcYf)
			fx := srcXf - float64(sx)
			fy := srcYf - float64(sy)
			x0 := clampInt(sx, minX, maxX)
			x1 := clampInt(sx+1, minX, maxX)
			y0 := clampInt(sy, minY, maxY)
			y1 := clampInt(sy+1, minY, maxY)

			r00, g00, b00, _ := img.At(x0, y0).RGBA()
			r10, g10, b10, _ := img.At(x1, y0).RGBA()
			r01, g01, b01, _ := img.At(x0, y1).RGBA()
			r11, g11, b11, _ := img.At(x1, y1).RGBA()

			px := rgb{
				R: blerp(r00, r10, r01, r11, fx, fy),
				G: blerp(g00, g10, g01, g11, fx, fy),
				B: blerp(b00, b10, b01, b11, fx, fy),
			}

			// Subtle groove rings
			if math.Mod(dist, grooveInterval) < 0.8 {
				px.R = uint8(float64(px.R) * 0.75)
				px.G = uint8(float64(px.G) * 0.75)
				px.B = uint8(float64(px.B) * 0.75)
			}

			// Smooth edge fade
			if edge := radius - dist; edge < 2.0 {
				f := edge / 2.0
				px.R = uint8(float64(px.R) * f)
				px.G = uint8(float64(px.G) * f)
				px.B = uint8(float64(px.B) * f)
			}

			pixels[y][x] = px
		}
	}

	var sb strings.Builder
	sb.Grow(w * h * 30)
	for row := 0; row < pixH; row += 2 {
		for col := range w {
			top := pixels[row][col]
			bot := pixels[row+1][col]
			sb.WriteString("\x1b[38;2;")
			sb.WriteString(decLUT[top.R])
			sb.WriteByte(';')
			sb.WriteString(decLUT[top.G])
			sb.WriteByte(';')
			sb.WriteString(decLUT[top.B])
			sb.WriteString("m\x1b[48;2;")
			sb.WriteString(decLUT[bot.R])
			sb.WriteByte(';')
			sb.WriteString(decLUT[bot.G])
			sb.WriteByte(';')
			sb.WriteString(decLUT[bot.B])
			sb.WriteString("m▀")
		}
		sb.WriteString("\x1b[0m")
		if row+2 < pixH {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// PlaceholderArt returns a simple colored block when no art is available.
func PlaceholderArt(w, h int) string {
	var sb strings.Builder
	style := lipgloss.NewStyle().
		Foreground(ColorTextDim).
		Background(ColorSurface)
	row := style.Render(strings.Repeat("▀", w))
	for i := range h {
		sb.WriteString(row)
		if i < h-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
