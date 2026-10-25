package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The settings overlay (, or :settings): what's in config.json, changed in
// place. Each change is saved straight away through Options.SaveSetting;
// the ones that need the app to start again say so. Settings with a flow of
// their own (waxon's player, the Spotify app) show where they stand and the
// command that changes them.

// Settings is what the settings overlay shows, read from the config at
// startup and updated as it's changed.
type Settings struct {
	Theme      string
	Art        string // blocks, kitty or auto
	MediaKeys  bool
	CustomKeys int    // keybindings remapped in the config
	Path       string // the config file
}

// withDefaults fills what the config left unset with what waxon uses then.
func (s Settings) withDefaults() Settings {
	if s.Theme == "" {
		s.Theme = DefaultThemeName
	}
	if s.Art == "" {
		s.Art = "blocks"
	}
	return s
}

// artChoices are the "art" settings, in the order they cycle.
var artChoices = []string{"blocks", "kitty", "auto"}

// settingRow is one line of the overlay.
type settingRow struct {
	label, value, note string
	// change moves the setting by dir (-1 or 1); nil: not changed here.
	change func(m *Model, dir int) tea.Cmd
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// cycle returns the item dir steps from cur in items, wrapping.
func cycle(items []string, cur string, dir int) string {
	i := max(0, slices.Index(items, cur))
	return items[(i+dir+len(items))%len(items)]
}

// settingRows are the overlay's rows, from the Model's current state.
func (m Model) settingRows() []settingRow {
	s := m.opts.Settings.withDefaults()
	player, playerNote := "the Spotify app", "turn on: waxon player login"
	if m.opts.PlayerName != "" {
		player, playerNote = "waxon's player", "turn off: waxon player off"
	}
	spotifyApp, appNote := "your own", "change: waxon auth"
	if m.opts.SharedClientID {
		spotifyApp, appNote = "shared", "your own avoids rate limits: waxon auth --own"
	}
	keys := "defaults"
	if s.CustomKeys > 0 {
		keys = fmt.Sprintf("%d remapped", s.CustomKeys)
	}
	return []settingRow{
		{"Theme", s.Theme, "", func(m *Model, dir int) tea.Cmd {
			return m.changeTheme(cycle(ThemeNames(), m.opts.Settings.Theme, dir))
		}},
		{"Album art", s.Art, "after a restart", func(m *Model, dir int) tea.Cmd {
			v := cycle(artChoices, m.opts.Settings.Art, dir)
			return m.saveSetting("art", v, func() { m.opts.Settings.Art = v })
		}},
		{"Media keys", onOff(s.MediaKeys), "after a restart", func(m *Model, _ int) tea.Cmd {
			v := !m.opts.Settings.MediaKeys
			return m.saveSetting("media_keys", onOff(v), func() { m.opts.Settings.MediaKeys = v })
		}},
		{"Lyrics", onOff(!m.opts.LyricsOff), "", func(m *Model, _ int) tea.Cmd {
			off := !m.opts.LyricsOff
			return m.saveSetting("lyrics", onOff(!off), func() { m.opts.LyricsOff = off })
		}},
		{"Plays in", player, playerNote, nil},
		{"Spotify app", spotifyApp, appNote, nil},
		{"Keys", keys, "remap in config.json", nil},
	}
}

// saveSetting saves name=value and, once saved, applies it with apply.
func (m *Model) saveSetting(name, value string, apply func()) tea.Cmd {
	if m.opts.SaveSetting != nil {
		if err := m.opts.SaveSetting(name, value); err != nil {
			m.toast.Show("Not saved", err.Error(), ToastError)
			return scheduleAutoDismiss()
		}
	}
	apply()
	return nil
}

// changeTheme applies and saves a theme from the settings overlay, which
// shows it, so there's no toast.
func (m *Model) changeTheme(name string) tea.Cmd {
	p, err := ResolveTheme(name, m.opts.ColorOverrides)
	if err != nil {
		m.toast.Show(err.Error(), "", ToastError)
		return scheduleAutoDismiss()
	}
	return m.saveSetting("theme", name, func() {
		ApplyTheme(p)
		m.tracklist.Restyle()
		m.sidebar.Restyle()
		m.opts.Settings.Theme = name
	})
}

func (m Model) handleKeySettings(msg tea.KeyMsg) (Model, tea.Cmd) {
	rows := m.settingRows()
	switch {
	case msg.Type == tea.KeyEscape || msg.String() == "q" || key.Matches(msg, m.keys.Settings):
		m.leaveOverlay()
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.settingsCursor = min(m.settingsCursor+1, len(rows)-1)
	case key.Matches(msg, m.keys.Up):
		m.settingsCursor = max(m.settingsCursor-1, 0)
	case key.Matches(msg, m.keys.Bottom):
		m.settingsCursor = len(rows) - 1
	case msg.String() == "g":
		m.settingsCursor = 0
	case key.Matches(msg, m.keys.FocusLeft), msg.String() == "left":
		if r := rows[m.settingsCursor]; r.change != nil {
			return m, r.change(&m, -1)
		}
	case key.Matches(msg, m.keys.FocusRight), msg.String() == "right", key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.PlayPause):
		if r := rows[m.settingsCursor]; r.change != nil {
			return m, r.change(&m, 1)
		}
	}
	return m, nil
}

// viewSettings renders the overlay centred on the screen.
func (m Model) viewSettings() string {
	title := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorTextDim)
	text := lipgloss.NewStyle().Foreground(ColorText)
	sel := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	rows := m.settingRows()
	labelW, valueW := 0, 0
	for _, r := range rows {
		labelW = max(labelW, lipgloss.Width(r.label))
		valueW = max(valueW, lipgloss.Width(r.value)+4) // room for ‹ ›
	}
	lines := make([]string, 0, len(rows))
	for i, r := range rows {
		cursor, label := "  ", text.Render(r.label)
		value := r.value
		if r.change != nil {
			value = "‹ " + value + " ›"
		}
		valueS := dim.Render(value)
		if r.change != nil {
			valueS = text.Render(value)
		}
		if i == m.settingsCursor {
			cursor, label = sel.Render("› "), sel.Render(r.label)
			if r.change != nil {
				valueS = sel.Render(value)
			}
		}
		line := cursor + label + strings.Repeat(" ", labelW-lipgloss.Width(r.label)+3) +
			valueS + strings.Repeat(" ", max(1, valueW-lipgloss.Width(value)+3)) + dim.Render(r.note)
		lines = append(lines, strings.TrimRight(line, " "))
	}

	path := m.opts.Settings.Path
	if path == "" {
		path = "config.json"
	}
	header := title.Render("Settings") + "  " + dim.Render("saved to "+path)
	footer := dim.Render("j/k move · h/l or Enter change · Esc close")
	body := header + "\n\n" + strings.Join(lines, "\n") + "\n\n" + footer

	w := 0
	for _, l := range strings.Split(body, "\n") {
		w = max(w, lipgloss.Width(l))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Width(min(w+4, max(20, m.width-2))).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box, overlayBackdrop())
}
