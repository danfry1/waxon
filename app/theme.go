package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette is the set of colours a theme defines. Values are hex strings
// ("#RRGGBB"); lipgloss degrades them automatically on 256/16-colour
// terminals and drops them entirely under NO_COLOR.
type Palette struct {
	Accent     string `json:"accent,omitempty"`      // active pane border, playing track, mode badge
	Bg         string `json:"bg,omitempty"`          // app background (mode line, Now Playing text bg)
	Surface    string `json:"surface,omitempty"`     // selected rows, status bar
	Text       string `json:"text,omitempty"`        // primary text
	TextSec    string `json:"text_sec,omitempty"`    // secondary text (artist, description)
	TextDim    string `json:"text_dim,omitempty"`    // hints, headers, inactive
	Border     string `json:"border,omitempty"`      // inactive pane border
	Error      string `json:"error,omitempty"`       // error toasts
	ModeSearch string `json:"mode_search,omitempty"` // SEARCH mode badge background
	ModeFilter string `json:"mode_filter,omitempty"` // FILTER mode badge background
	Overlay    string `json:"overlay,omitempty"`     // backdrop behind floating overlays
}

// DefaultThemeName is the theme used when none is configured.
const DefaultThemeName = "spotify"

// Themes holds the built-in presets, keyed by name. Keep names lowercase and
// dash-separated; they are what users type in config.json and :theme.
var Themes = map[string]Palette{
	"spotify": {
		Accent: "#1DB954", Bg: "#191414", Surface: "#282828", Text: "#FFFFFF",
		TextSec: "#B3B3B3", TextDim: "#535353", Border: "#333333", Error: "#E22134",
		ModeSearch: "#E2B714", ModeFilter: "#BB9AF7", Overlay: "#000000",
	},
	"catppuccin-mocha": {
		Accent: "#A6E3A1", Bg: "#1E1E2E", Surface: "#313244", Text: "#CDD6F4",
		TextSec: "#A6ADC8", TextDim: "#6C7086", Border: "#45475A", Error: "#F38BA8",
		ModeSearch: "#F9E2AF", ModeFilter: "#CBA6F7", Overlay: "#11111B",
	},
	"catppuccin-latte": {
		Accent: "#40A02B", Bg: "#EFF1F5", Surface: "#CCD0DA", Text: "#4C4F69",
		TextSec: "#6C6F85", TextDim: "#9CA0B0", Border: "#BCC0CC", Error: "#D20F39",
		ModeSearch: "#DF8E1D", ModeFilter: "#8839EF", Overlay: "#DCE0E8",
	},
	"gruvbox": {
		Accent: "#B8BB26", Bg: "#282828", Surface: "#3C3836", Text: "#EBDBB2",
		TextSec: "#BDAE93", TextDim: "#7C6F64", Border: "#504945", Error: "#FB4934",
		ModeSearch: "#FABD2F", ModeFilter: "#D3869B", Overlay: "#1D2021",
	},
	"tokyonight": {
		Accent: "#9ECE6A", Bg: "#1A1B26", Surface: "#292E42", Text: "#C0CAF5",
		TextSec: "#A9B1D6", TextDim: "#565F89", Border: "#3B4261", Error: "#F7768E",
		ModeSearch: "#E0AF68", ModeFilter: "#BB9AF7", Overlay: "#15161E",
	},
	"nord": {
		Accent: "#A3BE8C", Bg: "#2E3440", Surface: "#3B4252", Text: "#ECEFF4",
		TextSec: "#D8DEE9", TextDim: "#616E88", Border: "#434C5E", Error: "#BF616A",
		ModeSearch: "#EBCB8B", ModeFilter: "#B48EAD", Overlay: "#242933",
	},
	"dracula": {
		Accent: "#50FA7B", Bg: "#282A36", Surface: "#44475A", Text: "#F8F8F2",
		TextSec: "#BFBFBF", TextDim: "#6272A4", Border: "#44475A", Error: "#FF5555",
		ModeSearch: "#F1FA8C", ModeFilter: "#BD93F9", Overlay: "#191A21",
	},
	"rose-pine": {
		Accent: "#9CCFD8", Bg: "#191724", Surface: "#26233A", Text: "#E0DEF4",
		TextSec: "#908CAA", TextDim: "#6E6A86", Border: "#403D52", Error: "#EB6F92",
		ModeSearch: "#F6C177", ModeFilter: "#C4A7E7", Overlay: "#12101A",
	},
}

// ThemeNames returns the built-in theme names, sorted.
func ThemeNames() []string {
	names := make([]string, 0, len(Themes))
	for n := range Themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Merge returns p with every non-empty field of o applied on top.
func (p Palette) Merge(o Palette) Palette {
	pick := func(base, over string) string {
		if over != "" {
			return over
		}
		return base
	}
	return Palette{
		Accent:     pick(p.Accent, o.Accent),
		Bg:         pick(p.Bg, o.Bg),
		Surface:    pick(p.Surface, o.Surface),
		Text:       pick(p.Text, o.Text),
		TextSec:    pick(p.TextSec, o.TextSec),
		TextDim:    pick(p.TextDim, o.TextDim),
		Border:     pick(p.Border, o.Border),
		Error:      pick(p.Error, o.Error),
		ModeSearch: pick(p.ModeSearch, o.ModeSearch),
		ModeFilter: pick(p.ModeFilter, o.ModeFilter),
		Overlay:    pick(p.Overlay, o.Overlay),
	}
}

// Validate reports the first field that isn't a #RRGGBB colour (empty fields
// are allowed — they mean "inherit").
func (p Palette) Validate() error {
	fields := []struct{ name, val string }{
		{"accent", p.Accent},
		{"bg", p.Bg},
		{"surface", p.Surface},
		{"text", p.Text},
		{"text_sec", p.TextSec},
		{"text_dim", p.TextDim},
		{"border", p.Border},
		{"error", p.Error},
		{"mode_search", p.ModeSearch},
		{"mode_filter", p.ModeFilter},
		{"overlay", p.Overlay},
	}
	for _, f := range fields {
		if f.val == "" {
			continue
		}
		if !isHexColor(f.val) {
			return fmt.Errorf("colour %s: %q is not a #RRGGBB value", f.name, f.val)
		}
	}
	return nil
}

func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range strings.ToLower(s[1:]) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// PaletteFromMap builds override colours from config-style key/value pairs
// (keys are the palette JSON names). Unknown keys are an error.
func PaletteFromMap(m map[string]string) (Palette, error) {
	var p Palette
	for k, v := range m {
		switch strings.ToLower(k) {
		case "accent":
			p.Accent = v
		case "bg":
			p.Bg = v
		case "surface":
			p.Surface = v
		case "text":
			p.Text = v
		case "text_sec":
			p.TextSec = v
		case "text_dim":
			p.TextDim = v
		case "border":
			p.Border = v
		case "error":
			p.Error = v
		case "mode_search":
			p.ModeSearch = v
		case "mode_filter":
			p.ModeFilter = v
		case "overlay":
			p.Overlay = v
		default:
			return Palette{}, fmt.Errorf("unknown colour %q (available: accent, bg, surface, text, text_sec, text_dim, border, error, mode_search, mode_filter, overlay)", k)
		}
	}
	return p, p.Validate()
}

// ResolveTheme builds the palette for a theme name with optional overrides.
// An empty name means the default; an unknown name is an error listing the
// available presets.
func ResolveTheme(name string, overrides Palette) (Palette, error) {
	if name == "" {
		name = DefaultThemeName
	}
	base, ok := Themes[strings.ToLower(name)]
	if !ok {
		return Palette{}, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(ThemeNames(), ", "))
	}
	if err := overrides.Validate(); err != nil {
		return Palette{}, err
	}
	return base.Merge(overrides), nil
}

// Live colours — read at render time by every view. Set via ApplyTheme.
var (
	ColorAccent     lipgloss.Color
	ColorBg         lipgloss.Color
	ColorSurface    lipgloss.Color
	ColorText       lipgloss.Color
	ColorTextSec    lipgloss.Color
	ColorTextDim    lipgloss.Color
	ColorBorder     lipgloss.Color
	ColorError      lipgloss.Color
	ColorModeSearch lipgloss.Color
	ColorModeFilter lipgloss.Color
	ColorOverlay    lipgloss.Color
)

// currentTheme is the palette most recently applied (for :theme feedback).
var currentTheme Palette

// CurrentPalette returns the palette currently applied.
func CurrentPalette() Palette { return currentTheme }

// CurrentAccent returns the accent color used for active elements.
func CurrentAccent() lipgloss.Color {
	return ColorAccent
}

// Reusable styles — rebuilt by ApplyTheme.
var (
	StyleSectionHeader lipgloss.Style
	StyleActiveItem    lipgloss.Style
	StyleDimText       lipgloss.Style
	StyleModeNormal    lipgloss.Style
	StyleModeCommand   lipgloss.Style
	StyleModeSearch    lipgloss.Style
	StyleModeFilter    lipgloss.Style
	StyleStatusBar     lipgloss.Style
	StyleModeLine      lipgloss.Style
)

// ApplyTheme installs p as the live palette and rebuilds the shared styles.
// Views read the Color*/Style* package variables at render time, so the
// change is visible on the next frame; components that captured styles at
// construction (tracklist table, sidebar list) re-read them via Restyle.
func ApplyTheme(p Palette) {
	currentTheme = p
	ColorAccent = lipgloss.Color(p.Accent)
	ColorBg = lipgloss.Color(p.Bg)
	ColorSurface = lipgloss.Color(p.Surface)
	ColorText = lipgloss.Color(p.Text)
	ColorTextSec = lipgloss.Color(p.TextSec)
	ColorTextDim = lipgloss.Color(p.TextDim)
	ColorBorder = lipgloss.Color(p.Border)
	ColorError = lipgloss.Color(p.Error)
	ColorModeSearch = lipgloss.Color(p.ModeSearch)
	ColorModeFilter = lipgloss.Color(p.ModeFilter)
	ColorOverlay = lipgloss.Color(p.Overlay)

	StyleSectionHeader = lipgloss.NewStyle().Foreground(ColorTextDim).Bold(true).PaddingLeft(1)
	StyleActiveItem = lipgloss.NewStyle().Background(ColorSurface).Foreground(ColorAccent).Bold(true)
	StyleDimText = lipgloss.NewStyle().Foreground(ColorTextDim)
	StyleModeNormal = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true).Padding(0, 1)
	StyleModeCommand = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorText).Bold(true).Padding(0, 1)
	StyleModeSearch = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorModeSearch).Bold(true).Padding(0, 1)
	StyleModeFilter = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorModeFilter).Bold(true).Padding(0, 1)
	StyleStatusBar = lipgloss.NewStyle().Background(ColorSurface).Foreground(ColorText)
	StyleModeLine = lipgloss.NewStyle().Background(ColorBg).Foreground(ColorTextSec)
}

func init() {
	ApplyTheme(Themes[DefaultThemeName])
}

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

// overlayBackdrop is the whitespace option used behind floating overlays.
func overlayBackdrop() lipgloss.WhitespaceOption {
	return lipgloss.WithWhitespaceBackground(ColorOverlay)
}
