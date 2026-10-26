package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/danfry1/waxon/source"
)

func settingsModel(t *testing.T) (Model, *[]string) {
	return settingsModelWith(t, &StubSource{})
}

func settingsModelWith(t *testing.T, src *StubSource) (Model, *[]string) {
	t.Helper()
	var saved []string
	m := newTestModel(src).WithOptions(Options{
		Settings: Settings{Theme: DefaultThemeName},
		SaveSetting: func(name, value string) error {
			saved = append(saved, name+"="+value)
			return nil
		},
	})
	m.width, m.height = 120, 40
	return m, &saved
}

func pressKeys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		res, _ := m.Update(msg)
		m = res.(Model)
	}
	return m
}

// , opens the settings, and Esc closes them.
func TestSettingsOpenAndClose(t *testing.T) {
	m, _ := settingsModel(t)
	m = pressKeys(t, m, ",")
	if m.mode != ModeSettings || !strings.Contains(m.View(), "Settings") {
		t.Fatal(", should open the settings overlay")
	}
	if m = pressKeys(t, m, "esc"); m.mode != ModeNormal {
		t.Error("Esc should close the settings")
	}
}

// Changing the theme applies it and saves it under its config name.
func TestSettingsTheme(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(Themes[DefaultThemeName]) })
	m, saved := settingsModel(t)
	m = pressKeys(t, m, ",", "l")
	want := cycle(ThemeNames(), DefaultThemeName, 1)
	if m.opts.Settings.Theme != want {
		t.Errorf("theme = %q, want %q", m.opts.Settings.Theme, want)
	}
	if len(*saved) != 1 || (*saved)[0] != "theme="+want {
		t.Errorf("saved %v, want theme=%s", *saved, want)
	}
}

// :theme and the overlay agree on the theme, so the overlay moves on from
// the one :theme chose.
func TestSettingsFollowsThemeCommand(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(Themes[DefaultThemeName]) })
	m, _ := settingsModel(t)
	m.setTheme("nord")
	m = pressKeys(t, m, ",", "l")
	if want := cycle(ThemeNames(), "nord", 1); m.opts.Settings.Theme != want {
		t.Errorf("after :theme nord, the overlay moved to %q, want %q", m.opts.Settings.Theme, want)
	}
}

// Turning lyrics back on looks up the playing track's, which were settled
// as none while they were off.
func TestSettingsLyricsBackOn(t *testing.T) {
	asked := 0
	src := &StubSource{LyricsFn: func(context.Context, source.Track) (*source.Lyrics, error) {
		asked++
		return &source.Lyrics{Plain: "la"}, nil
	}}
	m, _ := settingsModelWith(t, src)
	m.opts.LyricsOff = true
	m.track = &source.Track{ID: "t1", Name: "Song"}
	m.ensureLyrics()
	m = pressKeys(t, m, ",", "j", "j", "j")
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.opts.LyricsOff {
		t.Fatal("Enter should turn lyrics on")
	}
	drain(t, cmd)
	if asked != 1 {
		t.Errorf("turning lyrics on asked lrclib %d times, want 1", asked)
	}
}

// drain runs cmd and any batch inside it.
func drain(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	if b, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range b {
			drain(t, c)
		}
	}
}

// The panel fits small terminals: no line wider than the screen, and no
// row of it lost off the bottom.
func TestSettingsFitsSmallTerminals(t *testing.T) {
	for _, sz := range [][2]int{{40, 10}, {60, 12}, {80, 24}} {
		m, _ := settingsModel(t)
		m.opts.SharedClientID = true
		res, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = pressKeys(t, res.(Model), ",")
		out := m.View()
		for _, l := range strings.Split(out, "\n") {
			if w := lipgloss.Width(l); w > sz[0] {
				t.Errorf("%dx%d: a line is %d wide", sz[0], sz[1], w)
				break
			}
		}
		if !strings.Contains(out, "defaults") {
			t.Errorf("%dx%d: the panel's last row or border is cut off", sz[0], sz[1])
		}
	}
}

// Lyrics turn off and on at once, and are saved.
func TestSettingsLyrics(t *testing.T) {
	m, saved := settingsModel(t)
	m = pressKeys(t, m, ",", "j", "j", "j", "enter")
	if !m.opts.LyricsOff {
		t.Fatal("Enter on Lyrics should turn them off")
	}
	m = pressKeys(t, m, "enter")
	if m.opts.LyricsOff {
		t.Error("Enter again should turn them back on")
	}
	if got := strings.Join(*saved, " "); got != "lyrics=off lyrics=on" {
		t.Errorf("saved %q", got)
	}
}

// Rows changed elsewhere (the player, the Spotify app, keys) don't change
// here, and say how.
func TestSettingsReadOnlyRows(t *testing.T) {
	m, saved := settingsModel(t)
	m = pressKeys(t, m, ",", "G", "enter", "l")
	if len(*saved) != 0 {
		t.Errorf("a read-only row saved %v", *saved)
	}
	if !strings.Contains(m.View(), "remap in config.json") {
		t.Error("the keys row should say where to remap")
	}
}

// :settings opens the overlay too.
func TestSettingsCommand(t *testing.T) {
	m, _ := settingsModel(t)
	m = pressKeys(t, m, ":", "s", "e", "t", "t", "i", "n", "g", "s", "enter")
	if m.mode != ModeSettings {
		t.Errorf(":settings left mode %v", m.mode)
	}
}

// The update check is turned off and on from the settings, and a newer
// release shows on the Version row.
func TestSettingsUpdateCheck(t *testing.T) {
	m, saved := settingsModel(t)
	m.opts.Version = "1.13.0"
	m.opts.UpgradeCommand = "brew upgrade waxon"
	res, _ := m.Update(updateMsg{"1.14.0"})
	m = res.(Model)
	if m.NewVersion() != "1.14.0" || !strings.Contains(m.View(), "1.14.0 is out") {
		t.Fatal("a newer release should be toasted and remembered")
	}
	m = pressKeys(t, m, ",")
	for range 4 {
		m = pressKeys(t, m, "j")
	}
	m = pressKeys(t, m, "l")
	if !m.opts.Settings.UpdateCheckOff || strings.Join(*saved, " ") != "update_check=off" {
		t.Errorf("saved %v, want update_check=off", *saved)
	}
	if v := m.View(); !strings.Contains(v, "1.14.0 is out: brew upgrade waxon") {
		t.Errorf("the Version row should say how to upgrade:\n%s", v)
	}
}

// Without a newer release there's nothing to say.
func TestUpdateNothingNew(t *testing.T) {
	m, _ := settingsModel(t)
	if m.checkUpdate() != nil {
		t.Error("no CheckUpdate: nothing should run")
	}
	res, cmd := m.Update(updateMsg{""})
	if res.(Model).NewVersion() != "" || cmd != nil {
		t.Error("no newer release: no toast")
	}
}
