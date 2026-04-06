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
	width int
}

func NewStatusBar(width int) StatusBar {
	return StatusBar{width: width}
}

func (s *StatusBar) Resize(width int) {
	s.width = width
}

// ViewNowPlaying renders the now-playing row.
func (s StatusBar) ViewNowPlaying(track *source.Track, shuffleOn bool, repeatMode source.RepeatMode) string {
	if s.width == 0 {
		return ""
	}

	style := StyleStatusBar.Width(s.width)

	if track == nil {
		return style.Render("  No track playing")
	}

	// Left: play state + track info
	icon := "⏸"
	if track.Playing {
		icon = "▶"
	}
	info := fmt.Sprintf(" %s %s · %s · %s", icon, track.Name, track.Artist, track.Album)

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
	right := progress
	if len(indicators) > 0 {
		right = strings.Join(indicators, " ") + "  " + progress
	}

	// Fill middle with progress bar
	leftW := lipgloss.Width(info)
	rightW := lipgloss.Width(right) + 2
	barW := s.width - leftW - rightW - 4
	if barW < 5 {
		barW = 5
	}

	bar := s.renderProgressBar(track, barW)

	gap := s.width - leftW - lipgloss.Width(bar) - rightW - 2
	if gap < 0 {
		gap = 0
	}

	line := info + strings.Repeat(" ", 2) + bar + strings.Repeat(" ", gap) + right + " "
	return style.Render(line)
}

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
		left = " " + StyleModeCommand.Render(":") + " " + cmdInput
	case ModeFilter:
		left = " " + StyleModeFilter.Render("/") + " " + filterInput
	case ModeSearch:
		left = " " + StyleModeSearch.Render("SEARCH")
	case ModeHelp:
		left = " " + StyleModeNormal.Render("HELP")
	case ModeActions:
		left = " " + StyleModeNormal.Render("ACTIONS")
	case ModeDevices:
		left = " " + StyleModeNormal.Render("DEVICES")
	}

	// Right: help hint + volume + device
	right := "? help  "
	right += fmt.Sprintf("♪ %d%%", volume)
	if deviceName != "" {
		right += "  " + deviceName
	}
	right += " "

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := s.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	line := left + strings.Repeat(" ", gap) + StyleDimText.Render(right)
	return style.Render(line)
}

// ViewNowPlayingWithArt renders the now-playing panel with album art on the left.
func (s StatusBar) ViewNowPlayingWithArt(track *source.Track, shuffleOn bool, repeatMode source.RepeatMode, artBlock string, volume int, deviceName string, mode Mode, cmdInput string, filterInput string, filterActive string) string {
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
	titleLine := nowPlayStyle.Render(fmt.Sprintf(" %s %s", icon, track.Name))
	artistLine := StyleDimText.Render(fmt.Sprintf("   %s — %s", track.Artist, track.Album))

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
	volLine := StyleDimText.Render(fmt.Sprintf("   ♪ %d%%", volume))
	if deviceName != "" {
		volLine += StyleDimText.Render("  " + deviceName)
	}

	// Mode indicator + help hint
	var modeLine string
	switch mode {
	case ModeCommand:
		modeLine = "   " + StyleModeCommand.Render(":") + " " + cmdInput
	case ModeFilter:
		modeLine = "   " + StyleModeFilter.Render("/") + " " + filterInput
	case ModeSearch:
		modeLine = "   " + StyleModeSearch.Render("SEARCH")
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
