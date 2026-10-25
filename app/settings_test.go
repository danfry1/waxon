package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func settingsModel(t *testing.T) (Model, *[]string) {
	t.Helper()
	var saved []string
	m := newTestModel(&StubSource{}).WithOptions(Options{
		Settings: Settings{Theme: DefaultThemeName, MediaKeys: true},
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
	m, saved := settingsModel(t)
	m = pressKeys(t, m, ",", "l")
	want := cycle(ThemeNames(), DefaultThemeName, 1)
	if m.opts.Settings.Theme != want {
		t.Errorf("theme = %q, want %q", m.opts.Settings.Theme, want)
	}
	if len(*saved) != 1 || (*saved)[0] != "theme="+want {
		t.Errorf("saved %v, want theme=%s", *saved, want)
	}
	ApplyTheme(Themes[DefaultThemeName])
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
