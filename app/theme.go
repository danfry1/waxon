package app

import "github.com/charmbracelet/lipgloss"

// Spotify-inspired color palette
var (
	ColorAccent  = lipgloss.Color("#1DB954")
	ColorBg      = lipgloss.Color("#191414")
	ColorSurface = lipgloss.Color("#282828")
	ColorText    = lipgloss.Color("#FFFFFF")
	ColorTextSec = lipgloss.Color("#B3B3B3")
	ColorTextDim = lipgloss.Color("#535353")
	ColorBorder  = lipgloss.Color("#333333")
	ColorError   = lipgloss.Color("#E22134")
)

// CurrentAccent returns the accent color used for active elements.
func CurrentAccent() lipgloss.Color {
	return ColorAccent
}

// Reusable styles
var (
	StyleSectionHeader = lipgloss.NewStyle().
				Foreground(ColorTextDim).
				Bold(true).
				PaddingLeft(1)

	StyleActiveItem = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorAccent).
			Bold(true)

	StyleDimText = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	StyleModeNormal = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(ColorAccent).
			Bold(true).
			Padding(0, 1)

	StyleModeCommand = lipgloss.NewStyle().
				Foreground(ColorBg).
				Background(ColorText).
				Bold(true).
				Padding(0, 1)

	StyleModeSearch = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(lipgloss.Color("#E2B714")).
			Bold(true).
			Padding(0, 1)

	StyleModeFilter = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(lipgloss.Color("#BB9AF7")).
			Bold(true).
			Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorText)

	StyleModeLine = lipgloss.NewStyle().
			Background(ColorBg).
			Foreground(ColorTextSec)
)

// PaneBorder returns a border style for a pane.
func PaneBorder(active bool) lipgloss.Style {
	borderColor := ColorBorder
	if active {
		borderColor = CurrentAccent()
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
}
