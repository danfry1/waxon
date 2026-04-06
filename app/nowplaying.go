package app

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

const (
	npArtW = 70 // columns for large album art
	npArtH = 35 // terminal rows for large album art (same as original)
)

// RenderNowPlaying renders the full-screen now playing overlay with
// a blurred, darkened album art background.
func RenderNowPlaying(track *source.Track, artBlock string, albumImg image.Image, vinylMode bool, vinylAngle float64, width, height int) string {
	if width == 0 || height == 0 {
		return ""
	}

	// Extract accent color
	accent := CurrentAccent()
	if albumImg != nil {
		if c := DominantColor(albumImg); c != "" {
			accent = c
		}
	}

	// Compute blurred background (per terminal row)
	var bgRows []rgb
	if albumImg != nil {
		bgRows = computeBgRows(albumImg, height)
	}

	// Render art block — vinyl uses bgRows for outside-circle pixels
	if vinylMode && albumImg != nil {
		artBlock = renderVinyl(albumImg, vinylAngle, npArtW, npArtH, bgRows)
	}

	// Compute a single bg color for the text area (bottom-center of gradient)
	textBg := lipgloss.Color("#191414")
	if bgRows != nil {
		// Use a row from the lower third for text bg
		textRowIdx := min(len(bgRows)-1, height*3/4)
		c := bgRows[textRowIdx]
		textBg = lipgloss.Color(rgbHex(c.R, c.G, c.B))
	}

	// Lighten accent for text readability on dark backgrounds
	textAccent := lightenColor(accent)

	// Build foreground content — text elements get explicit bg
	titleStyle := lipgloss.NewStyle().Foreground(textAccent).Bold(true).Background(textBg)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(textBg)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#B3B3B3")).Background(textBg)

	var sections []string

	if artBlock != "" {
		sections = append(sections, artBlock)
	} else {
		sections = append(sections, PlaceholderArt(npArtW, npArtH))
	}

	sections = append(sections, "")

	if track != nil {
		sections = append(sections, titleStyle.Render(track.Name))
		sections = append(sections, subtitleStyle.Render(track.Artist+" — "+track.Album))
	} else {
		sections = append(sections, dimStyle.Render("No track playing"))
	}

	sections = append(sections, "")

	if track != nil {
		barW := min(60, width-20)
		if barW < 10 {
			barW = 10
		}
		sections = append(sections, renderNPProgressBarBg(track, barW, textAccent, textBg))
		timeStr := fmt.Sprintf("%s / %s", fmtDur(track.Position), fmtDur(track.Duration))
		sections = append(sections, dimStyle.Render(timeStr))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, sections...)
	contentLines := strings.Split(content, "\n")

	topPad := (height - len(contentLines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	// Compose each row with gradient background filling full width
	var sb strings.Builder
	for y := range height {
		bgHex := "#191414"
		if bgRows != nil && y < len(bgRows) {
			c := bgRows[y]
			bgHex = rgbHex(c.R, c.G, c.B)
		}
		bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(bgHex))

		contentIdx := y - topPad
		if contentIdx >= 0 && contentIdx < len(contentLines) {
			line := contentLines[contentIdx]
			lineW := lipgloss.Width(line)
			leftPad := (width - lineW) / 2
			if leftPad < 0 {
				leftPad = 0
			}
			rightPad := width - lineW - leftPad
			if rightPad < 0 {
				rightPad = 0
			}
			sb.WriteString(bgStyle.Render(strings.Repeat(" ", leftPad)))
			sb.WriteString(line)
			sb.WriteString(bgStyle.Render(strings.Repeat(" ", rightPad)))
		} else {
			sb.WriteString(bgStyle.Render(strings.Repeat(" ", width)))
		}

		if y < height-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// computeBgRows generates one background color per terminal row from a blurred
// version of the album art, with darkening and vignette.
func computeBgRows(img image.Image, height int) []rgb {
	// Scale to tiny for blur
	tiny := scaleBilinear(img, 8, 8)

	rows := make([]rgb, height)
	for y := range height {
		srcY := float64(y) * 7.0 / float64(max(1, height-1))
		sy := int(srcY)
		fy := srcY - float64(sy)
		y0 := min(sy, 6)
		y1 := min(sy+1, 7)

		// Sample center column for this row
		r := bilinearBlend(tiny[y0][3].R, tiny[y0][4].R, tiny[y1][3].R, tiny[y1][4].R, 0.5, fy)
		g := bilinearBlend(tiny[y0][3].G, tiny[y0][4].G, tiny[y1][3].G, tiny[y1][4].G, 0.5, fy)
		b := bilinearBlend(tiny[y0][3].B, tiny[y0][4].B, tiny[y1][3].B, tiny[y1][4].B, 0.5, fy)

		// Darken + vignette
		t := float64(y) / float64(max(1, height-1))
		brightness := 0.05 + 0.12*math.Exp(-10*(t-0.5)*(t-0.5))

		rows[y] = rgb{
			R: uint8(math.Min(255, float64(r)*brightness)),
			G: uint8(math.Min(255, float64(g)*brightness)),
			B: uint8(math.Min(255, float64(b)*brightness)),
		}
	}
	return rows
}

func bilinearBlend(c00, c10, c01, c11 uint8, fx, fy float64) uint8 {
	top := float64(c00)*(1-fx) + float64(c10)*fx
	bot := float64(c01)*(1-fx) + float64(c11)*fx
	return uint8(top*(1-fy) + bot*fy)
}

// lightenColor takes an accent color and ensures it's bright enough to
// read on a dark background. Blends toward white to reach min luminance.
func lightenColor(c lipgloss.Color) lipgloss.Color {
	var r, g, b uint8
	if n, _ := fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b); n != 3 {
		return c // can't parse — return as-is
	}

	// Compute perceived luminance
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)

	// If already bright enough, use as-is
	if lum >= 140 {
		return c
	}

	// Blend toward white until luminance reaches ~160
	target := 160.0
	blend := (target - lum) / (255.0 - lum)
	if blend > 0.7 {
		blend = 0.7
	}

	lr := uint8(float64(r) + (255-float64(r))*blend)
	lg := uint8(float64(g) + (255-float64(g))*blend)
	lb := uint8(float64(b) + (255-float64(b))*blend)
	return lipgloss.Color(rgbHex(lr, lg, lb))
}

func renderNPProgressBarBg(track *source.Track, width int, accent lipgloss.Color, bg lipgloss.Color) string {
	if track.Duration == 0 {
		return lipgloss.NewStyle().Foreground(ColorTextDim).Background(bg).Render(strings.Repeat("─", width))
	}
	ratio := float64(track.Position) / float64(track.Duration)
	filled := int(float64(width) * ratio)
	filled = max(0, min(filled, width-1))

	accentStyle := lipgloss.NewStyle().Foreground(accent).Background(bg)
	dimStyle := lipgloss.NewStyle().Foreground(ColorTextDim).Background(bg)
	return accentStyle.Render(strings.Repeat("━", filled)) +
		accentStyle.Render("●") +
		dimStyle.Render(strings.Repeat("─", max(0, width-filled-1)))
}
