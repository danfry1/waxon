package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Setting up waxon's own player from inside the app: the device picker
// offers a "play here" row while the player isn't set up, which opens the
// player part of Setup over the UI.

// playHereDeviceID marks the device picker's "play here" row.
const playHereDeviceID = "waxon:play-here"

// canSetUpPlayer reports whether the "play here" row is offered.
func (m Model) canSetUpPlayer() bool {
	return m.opts.PlayerName == "" && m.opts.PlayerSetup != nil && m.opts.PlayerSetup.Player != nil
}

// withPlayHere adds the "play here" row to a device list when it's offered.
func (m Model) withPlayHere(devices []source.Device) []source.Device {
	if !m.canSetUpPlayer() {
		return devices
	}
	return append(devices[:len(devices):len(devices)],
		source.Device{ID: playHereDeviceID, Name: "This computer: play here with waxon", Type: "+"})
}

// openPlayHerePicker opens the device picker with only the "play here" row,
// for when Spotify lists no devices.
func (m Model) openPlayHerePicker() (Model, tea.Cmd) {
	picker := NewDevicePicker(m.withPlayHere(nil), m.width, m.height)
	picker.SetPrompt("No Spotify apps are open. Play right here?")
	m.devices = &picker
	m.mode = ModeDevices
	return m, nil
}

// openPlayerSetup sets up waxon's player in place, or, when it's installed
// and signed in (it stopped, but not over its login), just restarts it.
func (m Model) openPlayerSetup() (Model, tea.Cmd) {
	if p := m.opts.PlayerSetup.Player; p.Installed() && p.LoggedIn() && !loginTrouble(m.playerErr) && m.opts.StartPlayer != nil {
		m.mode = ModeNormal
		return m.startedPlayer(m.opts.StartPlayer())
	}
	s := NewSetup(*m.opts.PlayerSetup, SetupPlayerOnly)
	s.width, s.height = m.width, m.height
	m.setup = s
	m.mode = ModeSetup
	return m, s.Init()
}

// updateSetup forwards msg to the open Setup, and closes it once it's done:
// a player it turned on is started and becomes the device to play on.
func (m Model) updateSetup(msg tea.Msg) (Model, tea.Cmd) {
	cmd := m.setup.update(msg)
	if !m.setup.Finished() && !m.setup.Quit() {
		return m, cmd
	}
	on := m.setup.PlayerOn()
	m.setup.Close()
	m.setup = nil
	m.mode = ModeNormal
	if on && m.opts.StartPlayer != nil {
		var toast tea.Cmd
		m, toast = m.startedPlayer(m.opts.StartPlayer())
		return m, tea.Batch(cmd, toast)
	}
	return m, cmd
}

// startedPlayer records the player StartPlayer just started (name "" if it
// couldn't) and says so.
func (m Model) startedPlayer(name string) (Model, tea.Cmd) {
	if name == "" {
		m.toast.Show("waxon's player didn't start", "Run 'waxon player' to see what's missing", ToastError)
		return m, scheduleAutoDismiss()
	}
	m.opts.PlayerName = name
	m.playerErr = nil
	m.toast.Show("waxon's player is starting", "Press play in a moment: it plays right here", ToastSuccess)
	return m, scheduleAutoDismiss()
}

// loginTrouble reports whether the player stopped over its login, which a
// plain restart can't fix.
func loginTrouble(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "credential") || strings.Contains(msg, "login") || strings.Contains(msg, "auth")
}

// setupOwnsMsg reports whether msg belongs to an open Setup.
func (m Model) setupOwnsMsg(msg tea.Msg) bool {
	if m.mode != ModeSetup || m.setup == nil {
		return false
	}
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg, spinner.TickMsg, setupMsg:
		return true
	}
	return false
}
