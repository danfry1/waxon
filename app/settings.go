package app

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The settings overlay (, or :settings): what's in config.json, changed in
// place. Each change is saved straight away through Options.SaveSetting;
// the ones that need the app to start again say so. Settings with a flow of
// their own (waxon's player, the Spotify app) show where they stand and the
// command that changes them.

// Settings is what the settings overlay shows, read from the config at
// startup and updated as it's changed.
type Settings struct {
	Theme        string
	Art          string // blocks, kitty or auto
	MediaKeysOff bool
	// UpdateCheckOff: waxon doesn't ask GitHub for newer releases.
	UpdateCheckOff bool
	CustomKeys     int    // keybindings remapped in the config
	Path           string // the config file
	// ArtNote and MediaKeysNote, when set, say why what's in effect isn't
	// the setting (WAXON_ART set, no media keys over SSH).
	ArtNote, MediaKeysNote string
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
	version, versionNote := cmp.Or(strings.TrimPrefix(m.opts.Version, "v"), "dev"), ""
	if m.newVersion != "" {
		versionNote = m.newVersion + " is out: " + strings.TrimPrefix(m.upgradeHint(), "Upgrade: ")
	}
	return []settingRow{
		{"Theme", s.Theme, "", func(m *Model, dir int) tea.Cmd {
			return m.changeTheme(cycle(ThemeNames(), m.opts.Settings.Theme, dir))
		}},
		{"Album art", s.Art, cmp.Or(s.ArtNote, "after a restart"), func(m *Model, dir int) tea.Cmd {
			v := cycle(artChoices, m.opts.Settings.Art, dir)
			m.opts.Settings.Art = v
			return m.saveSetting("art", v)
		}},
		{"Media keys", onOff(!s.MediaKeysOff), cmp.Or(s.MediaKeysNote, "after a restart"), func(m *Model, _ int) tea.Cmd {
			m.opts.Settings.MediaKeysOff = !m.opts.Settings.MediaKeysOff
			return m.saveSetting("media_keys", onOff(!m.opts.Settings.MediaKeysOff))
		}},
		{"Lyrics", onOff(!m.opts.LyricsOff), "", func(m *Model, _ int) tea.Cmd {
			m.opts.LyricsOff = !m.opts.LyricsOff
			save := m.saveSetting("lyrics", onOff(!m.opts.LyricsOff))
			if m.opts.LyricsOff {
				return save
			}
			// While off, the playing track was settled as having none: look
			// it up now.
			if m.lyrics == nil && !m.lyricsLoading {
				m.lyricsTrackID = ""
			}
			return tea.Batch(save, m.ensureLyrics())
		}},
		{"Update check", onOff(!s.UpdateCheckOff), "at launch, once a day", func(m *Model, _ int) tea.Cmd {
			m.opts.Settings.UpdateCheckOff = !m.opts.Settings.UpdateCheckOff
			return m.saveSetting("update_check", onOff(!m.opts.Settings.UpdateCheckOff))
		}},
		{"Plays in", player, playerNote, nil},
		{"Spotify app", spotifyApp, appNote, nil},
		{"Keys", keys, "remap in config.json", nil},
		{"Version", version, versionNote, nil},
	}
}

// saveSetting saves a setting already applied. A setting that can't be
// saved (a config.json that doesn't parse) still holds for this session, as
// :theme does, and the toast says why it wasn't saved.
func (m *Model) saveSetting(name, value string) tea.Cmd {
	if m.opts.SaveSetting == nil {
		return nil
	}
	if err := m.opts.SaveSetting(name, value); err != nil {
		m.toast.Show("Changed for now, not saved", err.Error(), ToastError)
		return scheduleAutoDismiss()
	}
	return nil
}

// changeTheme applies and saves a theme from the settings overlay, which
// shows it, so there's no toast.
func (m *Model) changeTheme(name string) tea.Cmd {
	if err := m.applyTheme(name); err != nil {
		m.toast.Show(err.Error(), "", ToastError)
		return scheduleAutoDismiss()
	}
	return m.saveSetting("theme", name)
}

// applyTheme switches to the named theme and records it, for :theme and
// the settings overlay.
func (m *Model) applyTheme(name string) error {
	p, err := ResolveTheme(name, m.opts.ColorOverrides)
	if err != nil {
		return err
	}
	ApplyTheme(p)
	m.tracklist.Restyle()
	m.sidebar.Restyle()
	m.opts.Settings.Theme = name
	return nil
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

	header := title.Render("Settings") + "  " + dim.Render("demo: nothing is saved")
	if m.opts.SaveSetting != nil {
		header = title.Render("Settings") + "  " + dim.Render("saved to "+cmp.Or(m.opts.Settings.Path, "config.json"))
	}
	footer := dim.Render("j/k move · h/l or Enter change · Esc close")

	// Fit the terminal: every line is cut (never wrapped) to the width
	// inside the border, and a short terminal loses the spacing and footer.
	room := max(10, m.width-6)
	pad, gap := 2, "\n\n"
	if m.height < len(rows)+8 {
		pad, gap, footer = 1, "\n", ""
	}
	all := append([]string{header, ""}, lines...)
	for i, l := range all {
		all[i] = ansi.Truncate(l, room-2*(pad-1), "…")
	}
	body := all[0] + gap + strings.Join(all[2:], "\n")
	if footer != "" {
		body += gap + ansi.Truncate(footer, room, "…")
	}

	w := 0
	for _, l := range strings.Split(body, "\n") {
		w = max(w, lipgloss.Width(l))
	}
	vpad := 1
	if footer == "" {
		vpad = 0
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(vpad, pad).
		Render(lipgloss.NewStyle().Width(w).Render(body))

	// The panel floats over the screen it was opened from, so a theme
	// change shows on the app behind it as you move through them.
	if under := m.underOverlay(); under != "" {
		return overlayCentered(under, box, m.width, m.height)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box, overlayBackdrop())
}
