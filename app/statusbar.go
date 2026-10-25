package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

// StatusBar renders the bottom two rows: now-playing bar and mode line.
type StatusBar struct {
	width      int
	visualHint string // selection summary shown in VISUAL mode
	pending    string // count prefix being typed (vim's showcmd)
	volume     volumeShow
}

// volumeShow is how the volume is drawn. While a change is in flight the
// level Spotify has accepted so far is drawn solidly and the rest of the
// way to the new level dimly; once the last level lands it glows faintly in
// the accent, fading out over volumeFadeSteps. Tapping again never re-dims
// what has landed or cuts a glow short, so rapid presses look steady.
type volumeShow struct {
	level    int // the level shown (the newest one set)
	accepted int // the level Spotify has accepted
	pending  bool
	glow     int // fade steps left
}

const (
	volumeFadeSteps = 6
	// volumePulseStep is how long each step of the fade shows.
	volumePulseStep = 70 * time.Millisecond
	// volumeGlowPeak is how far towards the accent the glow starts: a hint
	// of it, not a flash.
	volumeGlowPeak = 0.7
)

// tint is the glow's accent blended over from.
func (v volumeShow) tint(from lipgloss.Color) lipgloss.Color {
	return blendColor(from, ColorAccent, volumeGlowPeak*float64(v.glow)/volumeFadeSteps)
}

// volumeText renders "♪ N%": faint while a change is in flight, tinted by
// the glow after, otherwise in base.
func (s StatusBar) volumeText(volume int, base lipgloss.Style) string {
	text := fmt.Sprintf("♪ %d%%", volume)
	switch v := s.volume; {
	case v.pending:
		return lipgloss.NewStyle().Foreground(ColorTextDim).Faint(true).Render(text)
	case v.glow > 0:
		return lipgloss.NewStyle().Foreground(v.tint(ColorTextDim)).Render(text)
	}
	return base.Render(text)
}

func NewStatusBar(width int) StatusBar {
	return StatusBar{width: width}
}

func (s *StatusBar) Resize(width int) {
	s.width = width
}

// ViewNowPlaying renders the now-playing row.
func (s StatusBar) ViewNowPlaying(track *source.Track, shuffleOn bool, repeatMode source.RepeatMode, liked bool) string {
	if s.width == 0 {
		return ""
	}

	style := StyleStatusBar.Width(s.width)

	if track == nil {
		return style.Render("  No track playing")
	}

	info, right, barW := s.compactLayout(track, shuffleOn, repeatMode, liked)
	leftW := lipgloss.Width(info)
	rightW := lipgloss.Width(right) + 2

	bar := s.renderProgressBar(track, barW)

	gap := s.width - leftW - lipgloss.Width(bar) - rightW - 2
	if gap < 0 {
		gap = 0
	}

	line := info + strings.Repeat(" ", 2) + bar + strings.Repeat(" ", gap) + right + " "
	return style.Render(line)
}

// compactLayout splits the one-row now-playing bar into the track info on the
// left, time and badges on the right, and the progress bar width between.
// The bar starts two columns after the info.
func (s StatusBar) compactLayout(track *source.Track, shuffleOn bool, repeatMode source.RepeatMode, liked bool) (info, right string, barW int) {
	// Left: play state + track info
	icon := "⏸"
	if track.Playing {
		icon = "▶"
	}
	heart := ""
	if liked {
		heart = " ♥"
	}
	info = fmt.Sprintf(" %s %s%s · %s · %s", icon, track.Name, heart, track.Artist, track.Album)

	// Right: progress + indicators
	var indicators []string
	if shuffleOn {
		indicators = append(indicators, "[shfl]")
	}
	switch repeatMode {
	case source.RepeatContext:
		indicators = append(indicators, "[rpt]")
	case source.RepeatTrack:
		indicators = append(indicators, "[rpt1]")
	}

	progress := fmt.Sprintf("%s/%s", fmtDur(track.Position), fmtDur(track.Duration))
	right = progress
	if len(indicators) > 0 {
		right = strings.Join(indicators, " ") + "  " + progress
	}

	// Fill middle with progress bar. On narrow terminals the track info is
	// truncated (album first, then the rest) so the line never wraps.
	const minBarW = 5
	rightW := lipgloss.Width(right) + 2
	if s.width-rightW-minBarW-4 < 12 {
		// Not enough room for shuffle/repeat badges; keep just the time.
		right = progress
		rightW = lipgloss.Width(right) + 2
	}
	if maxInfo := s.width - rightW - minBarW - 4; lipgloss.Width(info) > maxInfo {
		short := fmt.Sprintf(" %s %s%s · %s", icon, track.Name, heart, track.Artist)
		if lipgloss.Width(short) > maxInfo {
			short = truncate(short, max(4, maxInfo))
		}
		info = short
	}
	barW = max(minBarW, s.width-lipgloss.Width(info)-rightW-4)
	return info, right, barW
}

// ProgressBarSpan reports where the progress bar is drawn, relative to the
// top-left of the status area: its row, first column and width. ok is false
// when no bar is shown.
func (s StatusBar) ProgressBarSpan(track *source.Track, shuffleOn bool, repeatMode source.RepeatMode, liked, withArt bool) (row, x, w int, ok bool) {
	if track == nil || s.width == 0 {
		return 0, 0, 0, false
	}
	if withArt {
		// Mirrors ViewNowPlayingWithArt: padded art, a 2-col gap, then the
		// info lines, where the bar is line 4 behind a 3-col indent.
		infoW := s.width - (ArtWidth + 3) - 2
		return artBarLine, 1 + ArtWidth + 2 + 3, max(10, infoW-20), true
	}
	info, _, barW := s.compactLayout(track, shuffleOn, repeatMode, liked)
	return 0, lipgloss.Width(info) + 2, barW, true
}

// artBarLine is the info-panel line the progress bar sits on in the art
// layout ("", title, artist, "", bar, …).
const artBarLine = 4

// ViewModeLine renders the mode/command line row.
func (s StatusBar) ViewModeLine(mode Mode, cmdInput string, filterInput string, filterActive string, volume int, deviceName string) string {
	if s.width == 0 {
		return ""
	}

	style := StyleModeLine.Width(s.width)

	var left string
	switch mode {
	case ModeNormal:
		left = " " + StyleModeNormal.Render("NORMAL")
		if filterActive != "" {
			left += "  " + StyleModeFilter.Render("/"+filterActive)
		}
	case ModeCommand:
		left = " " + StyleModeCommand.Render(":") + " " + cmdInput + StyleDimText.Render(commandSuggestion(cmdInput))
	case ModeFilter:
		left = " " + StyleModeFilter.Render("/") + " " + filterInput
	case ModeSearch:
		left = " " + StyleModeSearch.Render("SEARCH")
	case ModeHelp:
		left = " " + StyleModeNormal.Render("HELP")
	case ModeSettings:
		left = " " + StyleModeNormal.Render("SETTINGS")
	case ModeActions:
		left = " " + StyleModeNormal.Render("ACTIONS")
	case ModeDevices:
		left = " " + StyleModeNormal.Render("DEVICES")
	case ModeVisual:
		left = " " + StyleModeFilter.Render("VISUAL") + " " + StyleDimText.Render(s.visualHint)
	}

	// Right: help hint + volume + device
	vol := s.volumeText(volume, StyleDimText)
	right := StyleDimText.Render("? help  ") + vol
	if s.pending != "" {
		right = StyleModeFilter.Render(s.pending) + StyleDimText.Render("  ") + right
	}
	if deviceName != "" {
		right += StyleDimText.Render("  " + deviceName)
	}
	right += " "

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	if leftW+rightW > s.width {
		// Drop the device name, then the help hint, rather than wrapping.
		right = vol + " "
		rightW = lipgloss.Width(right)
	}
	gap := s.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	line := left + strings.Repeat(" ", gap) + right
	return style.Render(line)
}

// ViewNowPlayingWithArt renders the now-playing panel with album art on the left.
func (s StatusBar) ViewNowPlayingWithArt(track *source.Track, shuffleOn bool, repeatMode source.RepeatMode, liked bool, artBlock string, volume int, deviceName string, mode Mode, cmdInput string, filterInput string, filterActive string) string {
	if s.width == 0 {
		return ""
	}

	artW := ArtWidth + 3 // art + padding
	infoW := s.width - artW - 2
	artPadded := lipgloss.NewStyle().PaddingLeft(1).Render(artBlock)

	if track == nil {
		info := lipgloss.NewStyle().Width(infoW).Height(ArtHeight).
			Foreground(ColorTextDim).Padding(1, 1).
			Render("No track playing")
		return lipgloss.JoinHorizontal(lipgloss.Top, artPadded, "  ", info)
	}

	// Build multi-line info panel to fill the art height
	icon := "⏸"
	if track.Playing {
		icon = "▶"
	}

	nowPlayStyle := lipgloss.NewStyle().Foreground(CurrentAccent()).Bold(true)
	heart := ""
	if liked {
		heart = " ♥"
	}
	// Truncate rather than let the panel wrap: a wrapped line would push the
	// progress bar off artBarLine, where clicks expect it.
	titleLine := nowPlayStyle.Render(truncate(fmt.Sprintf(" %s %s%s", icon, track.Name, heart), infoW))
	artistLine := StyleDimText.Render(truncate(fmt.Sprintf("   %s — %s", track.Artist, track.Album), infoW))

	// Progress bar
	barW := max(10, infoW-20)
	bar := "   " + s.renderProgressBar(track, barW)

	// Time + indicators
	progress := fmt.Sprintf("   %s / %s", fmtDur(track.Position), fmtDur(track.Duration))
	var indicators []string
	if shuffleOn {
		indicators = append(indicators, "[shfl]")
	}
	switch repeatMode {
	case source.RepeatContext:
		indicators = append(indicators, "[rpt]")
	case source.RepeatTrack:
		indicators = append(indicators, "[rpt1]")
	}
	if len(indicators) > 0 {
		progress += "  " + strings.Join(indicators, " ")
	}

	// Volume + device
	volLine := "   " + s.volumeText(volume, StyleDimText)
	if deviceName != "" {
		volLine += StyleDimText.Render("  " + deviceName)
	}

	// Mode indicator + help hint
	var modeLine string
	switch mode {
	case ModeCommand:
		modeLine = "   " + StyleModeCommand.Render(":") + " " + cmdInput + StyleDimText.Render(commandSuggestion(cmdInput))
	case ModeFilter:
		modeLine = "   " + StyleModeFilter.Render("/") + " " + filterInput
	case ModeSearch:
		modeLine = "   " + StyleModeSearch.Render("SEARCH")
	case ModeVisual:
		modeLine = "   " + StyleModeFilter.Render("VISUAL") + " " + StyleDimText.Render(s.visualHint)
	default:
		modeNorm := lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(CurrentAccent()).
			Bold(true).
			Padding(0, 1)
		modeLine = "   " + modeNorm.Render("NORMAL")
		if filterActive != "" {
			modeLine += "  " + StyleModeFilter.Render("/"+filterActive)
		}
	}
	if s.pending != "" {
		modeLine += "  " + StyleModeFilter.Render(s.pending)
	}
	modeLine += "  " + StyleDimText.Render("? help")

	// Pad info to match art height
	lines := []string{"", titleLine, artistLine, "", bar, progress, "", volLine, modeLine}
	for len(lines) < ArtHeight {
		lines = append(lines, "")
	}
	info := strings.Join(lines[:ArtHeight], "\n")

	infoBlock := lipgloss.NewStyle().Width(infoW).Render(info)
	return lipgloss.JoinHorizontal(lipgloss.Top, artPadded, "  ", infoBlock)
}

func (s StatusBar) renderProgressBar(track *source.Track, width int) string {
	if track == nil || track.Duration == 0 {
		return strings.Repeat("─", width)
	}

	ratio := float64(track.Position) / float64(track.Duration)
	filled := int(float64(width) * ratio)
	filled = max(0, min(filled, width-1))

	accentStyle := lipgloss.NewStyle().Foreground(CurrentAccent())
	bar := accentStyle.Render(strings.Repeat("━", filled)) +
		accentStyle.Render("●") +
		StyleDimText.Render(strings.Repeat("─", max(0, width-filled-1)))
	return bar
}

func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
