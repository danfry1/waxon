package app

import (
	"image"
	"image/color"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEveryThemeIsCompleteAndValid(t *testing.T) {
	for name, p := range Themes {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		// Every field must be set — presets must not inherit anything.
		if p.Merge(Palette{}) != p || p.Accent == "" || p.Bg == "" || p.Surface == "" || p.Text == "" ||
			p.TextSec == "" || p.TextDim == "" || p.Border == "" || p.Error == "" ||
			p.ModeSearch == "" || p.ModeFilter == "" || p.Overlay == "" {
			t.Errorf("%s: incomplete palette %+v", name, p)
		}
		if name != strings.ToLower(name) || strings.Contains(name, " ") {
			t.Errorf("theme name %q should be lowercase, no spaces", name)
		}
	}
	if _, ok := Themes[DefaultThemeName]; !ok {
		t.Fatal("default theme missing")
	}
}

func TestResolveTheme(t *testing.T) {
	p, err := ResolveTheme("", Palette{})
	if err != nil || p != Themes[DefaultThemeName] {
		t.Fatalf("empty name should resolve to default: %v %+v", err, p)
	}
	p, err = ResolveTheme("Nord", Palette{Accent: "#ABCDEF"})
	if err != nil || p.Accent != "#ABCDEF" || p.Bg != Themes["nord"].Bg {
		t.Fatalf("case-insensitive + override: %v %+v", err, p)
	}
	if _, err := ResolveTheme("solarized-nope", Palette{}); err == nil || !strings.Contains(err.Error(), "available:") {
		t.Errorf("unknown theme should list available ones, got %v", err)
	}
	if _, err := ResolveTheme("nord", Palette{Accent: "green"}); err == nil {
		t.Error("invalid override colour should error")
	}
}

func TestPaletteFromMap(t *testing.T) {
	p, err := PaletteFromMap(map[string]string{"accent": "#112233", "TEXT_DIM": "#445566"})
	if err != nil || p.Accent != "#112233" || p.TextDim != "#445566" {
		t.Fatalf("%v %+v", err, p)
	}
	if _, err := PaletteFromMap(map[string]string{"acent": "#112233"}); err == nil {
		t.Error("typo in colour key should error")
	}
	if _, err := PaletteFromMap(map[string]string{"accent": "#12"}); err == nil {
		t.Error("short hex should error")
	}
	if p, err := PaletteFromMap(nil); err != nil || p != (Palette{}) {
		t.Error("nil map is an empty override set")
	}
}

func TestApplyThemeUpdatesLiveColorsAndStyles(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(Themes[DefaultThemeName]) })
	ApplyTheme(Themes["dracula"])
	if string(ColorAccent) != "#50FA7B" || string(ColorOverlay) != "#191A21" {
		t.Errorf("colours not applied: %s %s", ColorAccent, ColorOverlay)
	}
	if CurrentPalette() != Themes["dracula"] {
		t.Error("CurrentPalette should report the applied theme")
	}
	if got := StyleModeNormal.GetBackground(); got != ColorAccent {
		t.Errorf("styles not rebuilt: mode badge bg = %v", got)
	}
}

func TestThemeCommandAppliesAndSaves(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(Themes[DefaultThemeName]) })
	saved := ""
	m := newTestModel(&StubSource{}).WithOptions(Options{
		ColorOverrides: Palette{Accent: "#010203"},
		SaveTheme:      func(name string) error { saved = name; return nil },
	})
	m.mode = ModeCommand
	m.cmdInput = "theme gruvbox"
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)
	if string(ColorBg) != Themes["gruvbox"].Bg {
		t.Errorf("theme not applied: bg=%s", ColorBg)
	}
	if string(ColorAccent) != "#010203" {
		t.Errorf("colour overrides should survive a theme switch, accent=%s", ColorAccent)
	}
	if saved != "gruvbox" {
		t.Errorf("theme should be persisted, saved=%q", saved)
	}
	if !strings.Contains(model.toast.View(120), "gruvbox") {
		t.Errorf("toast = %q", model.toast.View(120))
	}
	// Unknown theme: toast error, nothing saved.
	saved = ""
	m = newTestModel(&StubSource{}).WithOptions(Options{SaveTheme: func(name string) error { saved = name; return nil }})
	m.mode = ModeCommand
	m.cmdInput = "theme nope"
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if saved != "" || !strings.Contains(result.(Model).toast.View(120), "unknown theme") {
		t.Errorf("unknown theme should not save; toast=%q", result.(Model).toast.View(120))
	}
}

func TestParseThemeCommandUsage(t *testing.T) {
	if _, err := ParseCommand("theme"); err == nil || !strings.Contains(err.Error(), "nord") {
		t.Errorf("theme without arg should show usage listing themes, got %v", err)
	}
	c, err := ParseCommand("colorscheme nord")
	if err != nil || c.Type != CmdTheme || c.StrArg != "nord" {
		t.Errorf("alias: %+v %v", c, err)
	}
}

func TestKeyMapFromOverrides(t *testing.T) {
	km, err := KeyMapFromOverrides(map[string]string{"next": "l, right", "play_pause": "space,p"})
	if err != nil {
		t.Fatal(err)
	}
	if !keyMatches(km.Next, "l") || !keyMatches(km.Next, "right") || keyMatches(km.Next, "n") {
		t.Errorf("next override not applied: %v", km.Next.Keys())
	}
	if !keyMatches(km.PlayPause, " ") || !keyMatches(km.PlayPause, "p") {
		t.Errorf("space alias not handled: %v", km.PlayPause.Keys())
	}
	if km.Next.Help().Desc != DefaultKeyMap().Next.Help().Desc || km.Next.Help().Key != "l" {
		t.Errorf("help should keep the description and show the new key: %+v", km.Next.Help())
	}
	// Untouched bindings stay default.
	if !keyMatches(km.Prev, "p") {
		t.Error("prev should be unchanged")
	}
	if _, err := KeyMapFromOverrides(map[string]string{"nxt": "l"}); err == nil || !strings.Contains(err.Error(), "available:") {
		t.Errorf("unknown action should error with the list, got %v", err)
	}
	if _, err := KeyMapFromOverrides(map[string]string{"next": " , "}); err == nil {
		t.Error("empty key list should error")
	}
}

func TestKeyOverridesDriveTheModelAndHelp(t *testing.T) {
	km, _ := KeyMapFromOverrides(map[string]string{"quit": "ctrl+q", "help": "F1"})
	m := newTestModel(&StubSource{}).WithOptions(Options{Keys: &km})
	// Default q no longer quits; ctrl+q does.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if result.(Model).quitting {
		t.Error("q should no longer quit after override")
	}
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if !result.(Model).quitting || cmd == nil {
		t.Error("ctrl+q should quit")
	}
	// Help overlay reflects the override.
	m.mode = ModeHelp
	if v := m.View(); !strings.Contains(v, "C-q") {
		t.Error("help should show the overridden quit key")
	}
}

func keyMatches(b interface{ Keys() []string }, k string) bool {
	for _, key := range b.Keys() {
		if key == k {
			return true
		}
	}
	return false
}

func TestArtModes(t *testing.T) {
	t.Cleanup(func() { SetArtMode(ArtTrueColor) })
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	SetArtMode(ArtTrueColor)
	if got := renderHalfBlocks(img, 2, 1); !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Errorf("truecolor: %q", got)
	}
	SetArtMode(ArtANSI256)
	got := renderHalfBlocks(img, 2, 1)
	if !strings.Contains(got, "\x1b[38;5;196m") || strings.Contains(got, "38;2;") {
		t.Errorf("ansi256 should use 5;N escapes (red=196): %q", got)
	}
	SetArtMode(ArtOff)
	if got := renderHalfBlocks(img, 2, 1); got != "" {
		t.Errorf("art off should render nothing, got %q", got)
	}
}

func TestAnsi256Mapping(t *testing.T) {
	cases := []struct {
		c    rgb
		want int
	}{
		{rgb{0, 0, 0}, 16},
		{rgb{255, 255, 255}, 231},
		{rgb{255, 0, 0}, 196},
		{rgb{0, 255, 0}, 46},
		{rgb{0, 0, 255}, 21},
		{rgb{128, 128, 128}, 244},
		{rgb{8, 8, 8}, 232},
		{rgb{238, 238, 238}, 255},
	}
	for _, c := range cases {
		if got := ansi256(c.c); got != c.want {
			t.Errorf("ansi256(%v) = %d, want %d", c.c, got, c.want)
		}
	}
}
