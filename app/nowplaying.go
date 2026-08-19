package app

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

const (
	npArtW = 70 // max columns for large album art
	npArtH = 35 // max terminal rows for large album art (square: 2 px rows per cell)
	// npTextRows is the vertical space the text block under the art needs
	// (title, artist, bar, time, hint + spacing).
	npTextRows = 9
)

// npLyrics carries the lyrics view state into the Now Playing renderer.
type npLyrics struct {
	active  bool           // lyrics view toggled on (replaces the art region)
	lyrics  *source.Lyrics // loaded lyrics, or nil
	line    int            // focus line index (highlighted when synced)
	loading bool           // fetch in flight
	err     bool           // last fetch failed
}

// RenderNowPlaying renders the full-screen now playing overlay with
// a blurred, darkened album art background.
// npArtSize returns the album-art cell size for a terminal: up to 70×35,
// shrunk to fit the width and to leave room for the text below. Always an
// even number of columns so the square stays square.
func npArtSize(width, height int) (w, h int) {
	w = min(npArtW, width-4)
	h = w / 2
	if maxH := height - npTextRows; h > maxH {
		h = maxH
		w = h * 2
	}
	if h < 2 {
		return 0, 0
	}
	return w &^ 1, h
}

func RenderNowPlaying(track *source.Track, artBlock string, albumImg image.Image, vinylMode bool, vinylAngle float64, liked bool, lyr npLyrics, width, height int) string {
	if width == 0 || height == 0 {
		return ""
	}
	artW, artH := npArtSize(width, height)

	// Accent colour and blurred background depend only on the image and the
	// terminal height, but View runs every 500ms tick — memoise them.
	accent, bgRows := npDerived(albumImg, height)

	// Render art block — vinyl uses bgRows for outside-circle pixels
	if vinylMode && albumImg != nil {
		artBlock = renderVinyl(albumImg, vinylAngle, artW, artH, bgRows)
	}

	// Compute a single bg color for the text area (bottom-center of gradient)
	textBg := ColorBg
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
	subtitleStyle := lipgloss.NewStyle().Foreground(ColorText).Background(textBg)
	dimStyle := lipgloss.NewStyle().Foreground(ColorTextSec).Background(textBg)

	var sections []string

	switch {
	case lyr.active:
		sections = append(sections, renderLyricsBlock(lyr, artW, artH, textAccent, dimStyle, textBg))
	case artBlock != "" && artW > 0:
		sections = append(sections, artBlock)
	case artW > 0:
		sections = append(sections, PlaceholderArt(artW, artH))
	}

	sections = append(sections, "")

	if track != nil {
		heart := ""
		if liked {
			heart = " ♥"
		}
		sections = append(sections, titleStyle.Render(track.Name+heart))
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
		sections = append(sections, "")
		hint := "space play · n/p skip · f like · a queue · l lyrics · o actions · N close"
		if width < lipgloss.Width(hint)+2 {
			hint = "space · n/p · f · a · l · o · N"
		}
		sections = append(sections, dimStyle.Render(hint))
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
		bgHex := CurrentPalette().Bg
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

// npDerivedMemo caches the per-image derivations for the Now Playing view.
// View is only ever called from the Bubbletea render goroutine, so a single
// unguarded entry is sufficient.
var npDerivedMemo struct {
	img    image.Image
	height int
	accent lipgloss.Color
	rows   []rgb
}

// npDerived returns the accent colour and background gradient rows for img
// at the given height, recomputing only when either changes.
func npDerived(img image.Image, height int) (lipgloss.Color, []rgb) {
	if img == nil {
		return CurrentAccent(), nil
	}
	if npDerivedMemo.img == img && npDerivedMemo.height == height && npDerivedMemo.rows != nil {
		return npDerivedMemo.accent, npDerivedMemo.rows
	}
	accent := CurrentAccent()
	if c := DominantColor(img); c != "" {
		accent = c
	}
	rows := computeBgRows(img, height)
	npDerivedMemo.img, npDerivedMemo.height = img, height
	npDerivedMemo.accent, npDerivedMemo.rows = accent, rows
	return accent, rows
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

// activeLyricLine returns the index of the last synced line whose timestamp has
// been reached at pos. Returns 0 when nothing has started yet.
func activeLyricLine(lines []source.LyricLine, pos time.Duration) int {
	idx := 0
	for i, l := range lines {
		if l.Time <= pos {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// renderLyricsBlock renders the lyrics into a w×h region for the Now Playing
// overlay: a vertically-centered window around the focus line. For synced
// lyrics the active line is spotlit in the accent color and surrounding lines
// fade toward the background with distance (an Apple/Spotify-style focus
// effect); plain lyrics render evenly. Loading, error, and not-found states
// render as a single centered message.
func renderLyricsBlock(lyr npLyrics, w, h int, accent lipgloss.Color, dim lipgloss.Style, bg lipgloss.Color) string {
	switch {
	case lyr.loading:
		return centeredMessageBlock("Loading lyrics…", w, h, dim)
	case lyr.err:
		return centeredMessageBlock("Couldn't load lyrics", w, h, dim)
	case lyr.lyrics == nil || len(lyr.lyrics.Lines) == 0:
		return centeredMessageBlock("No lyrics found", w, h, dim)
	}

	lines := lyr.lyrics.Lines
	focus := clampInt(lyr.line, 0, len(lines)-1)
	start := focus - h/2 // center the focus line vertically
	baseText := ColorText

	rows := make([]string, h)
	for row := range h {
		idx := start + row
		if idx < 0 || idx >= len(lines) {
			continue // leave blank; the composer pads with background
		}
		text := truncate(lines[idx].Text, w)
		if text == "" {
			continue // preserve verse gaps as blank rows
		}
		var style lipgloss.Style
		switch {
		case lyr.lyrics.Synced && idx == focus:
			style = lipgloss.NewStyle().Foreground(accent).Bold(true).Background(bg)
		case lyr.lyrics.Synced:
			// Fade non-active lines toward the background with distance so the
			// current line stands out.
			d := idx - focus
			if d < 0 {
				d = -d
			}
			t := math.Min(0.82, 0.22*float64(d))
			style = lipgloss.NewStyle().Foreground(blendColor(baseText, bg, t)).Background(bg)
		default:
			// Plain lyrics have no active line; render evenly and readable.
			style = lipgloss.NewStyle().Foreground(blendColor(baseText, bg, 0.30)).Background(bg)
		}
		rows[row] = style.Render(text)
	}
	return strings.Join(rows, "\n")
}

// blendColor linearly interpolates from→to by t in [0,1] (t=0 returns from,
// t=1 returns to). Non-hex inputs return from unchanged.
func blendColor(from, to lipgloss.Color, t float64) lipgloss.Color {
	fr, fg, fb, ok1 := parseHexColor(from)
	tr, tg, tb, ok2 := parseHexColor(to)
	if !ok1 || !ok2 {
		return from
	}
	t = math.Max(0, math.Min(1, t))
	lerp := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	return lipgloss.Color(rgbHex(lerp(fr, tr), lerp(fg, tg), lerp(fb, tb)))
}

func parseHexColor(c lipgloss.Color) (r, g, b uint8, ok bool) {
	n, _ := fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b)
	return r, g, b, n == 3
}

// centeredMessageBlock renders msg on the middle row of an otherwise-blank
// h-row block.
func centeredMessageBlock(msg string, w, h int, dim lipgloss.Style) string {
	rows := make([]string, h)
	rows[h/2] = dim.Render(truncate(msg, w))
	return strings.Join(rows, "\n")
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
