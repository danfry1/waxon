package app

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Switching devices without the picker: gd / ":device -" swaps back to the
// previous device, ":device <name>" picks one by name. Both, like the
// picker's "open the Spotify app" row, can start this computer's Spotify
// app when it isn't running (Options.OpenSpotifyApp) and move playback to
// it once it comes online.

// openSpotifyDeviceID marks the device picker's "open the Spotify app" row.
const openSpotifyDeviceID = "waxon:open-spotify"

// spotifyAppWait is how long a starting Spotify app has to come online,
// checked every spotifyAppPoll (vars for tests).
var (
	spotifyAppWait = 25 * time.Second
	spotifyAppPoll = time.Second
)

var errSpotifyAppSlow = errors.New("the Spotify app didn't come online")

type (
	// playbackMovedMsg: playback is now on device.
	playbackMovedMsg struct{ device string }
	// deviceTargetMsg carries the devices for a switch by name (query) or,
	// with query "-", to the previous device.
	deviceTargetMsg struct {
		devices []source.Device
		query   string
	}
	// spotifyAppMsg: the Spotify app started for a switch came online as
	// dev, or didn't (err). retry is the command waiting on a device.
	spotifyAppMsg struct {
		dev   source.Device
		err   error
		retry tea.Cmd
	}
)

// isComputerApp reports whether dev looks like a Spotify app on a
// computer (not waxon's own player).
func (m Model) isComputerApp(dev source.Device) bool {
	return dev.Type == "Computer" && dev.Name != m.opts.PlayerName
}

// withOpenSpotify adds the "open the Spotify app" row to a device list
// when the app is installed here and no computer's Spotify app is listed.
func (m Model) withOpenSpotify(devices []source.Device) []source.Device {
	if m.opts.OpenSpotifyApp == nil {
		return devices
	}
	for _, d := range devices {
		if m.isComputerApp(d) {
			return devices
		}
	}
	return append(devices[:len(devices):len(devices)],
		source.Device{ID: openSpotifyDeviceID, Name: "This computer: open the Spotify app", Type: "+"})
}

// startSpotifyApp starts the Spotify app and moves playback to it (or, with
// retry, replays the command waiting on a device) once it's online.
func (m Model) startSpotifyApp(retry tea.Cmd) (Model, tea.Cmd) {
	m.toast.Show("Opening the Spotify app…", "Playback moves there once it's ready", ToastInfo)
	return m, tea.Batch(scheduleAutoDismiss(), m.waitForSpotifyApp(retry))
}

func (m Model) waitForSpotifyApp(retry tea.Cmd) tea.Cmd {
	src, ctx, launch, player := m.source, m.ctx, m.opts.OpenSpotifyApp, m.opts.PlayerName
	return func() tea.Msg {
		if err := launch(); err != nil {
			return spotifyAppMsg{err: err}
		}
		wait, cancel := context.WithTimeout(ctx, spotifyAppWait)
		defer cancel()
		tick := time.NewTicker(spotifyAppPoll)
		defer tick.Stop()
		for {
			select {
			case <-wait.Done():
				return spotifyAppMsg{err: errSpotifyAppSlow}
			case <-tick.C:
			}
			devices, err := src.Devices(wait)
			if err != nil {
				continue // it's starting; Spotify may not answer every time
			}
			for _, d := range devices {
				if d.Type == "Computer" && d.Name != player {
					return spotifyAppMsg{dev: d, retry: retry}
				}
			}
		}
	}
}

func (m Model) spotifyAppReady(msg spotifyAppMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		detail := "Open it and sign in, then press D"
		if !errors.Is(msg.err, errSpotifyAppSlow) {
			detail = msg.err.Error()
		}
		m.toast.Show("Couldn't switch to the Spotify app", detail, ToastError)
		return m, scheduleAutoDismiss()
	}
	m.noteDevice(msg.dev)
	if msg.retry != nil {
		m.toast.Show("Playing on "+msg.dev.Name, "", ToastInfo)
		return m, tea.Batch(scheduleAutoDismiss(), m.activateAndRetry(msg.dev.ID, msg.retry))
	}
	return m, m.transferPlayback(msg.dev.ID, msg.dev.Name)
}

// noteDevice remembers dev's type by name: track updates only carry the
// name, and switching back needs to know a computer's Spotify app.
func (m *Model) noteDevice(dev source.Device) {
	if m.deviceTypes == nil {
		m.deviceTypes = map[string]string{}
	}
	m.deviceTypes[dev.Name] = dev.Type
}

// switchDevice looks up the devices for a switch by name, or with "-" to
// the previous device.
func (m Model) switchDevice(query string) tea.Cmd {
	src, ctx := m.source, m.ctx
	return func() tea.Msg {
		devices, err := src.Devices(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return deviceTargetMsg{devices: devices, query: query}
	}
}

func (m Model) deviceTarget(msg deviceTargetMsg) (Model, tea.Cmd) {
	for _, d := range msg.devices {
		m.noteDevice(d)
	}
	if msg.query == "-" {
		return m.swapDevice(msg.devices)
	}
	q := strings.ToLower(msg.query)
	var matches []source.Device
	for _, d := range msg.devices {
		if strings.Contains(strings.ToLower(d.Name), q) {
			matches = append(matches, d)
		}
	}
	switch len(matches) {
	case 0:
		m.toast.Show("No device matches “"+msg.query+"”", "Press D to see the devices online", ToastError)
		return m, scheduleAutoDismiss()
	case 1:
		return m.moveTo(matches[0])
	}
	picker := NewDevicePicker(matches, m.width, m.height)
	picker.SetPrompt("Several devices match “" + msg.query + "”")
	m.devices = &picker
	m.mode = ModeDevices
	return m, nil
}

// swapDevice moves playback to the device played on before this one. With
// none, it toggles this computer between waxon's player and the Spotify
// app, starting the app if it isn't running.
func (m Model) swapDevice(devices []source.Device) (Model, tea.Cmd) {
	current, player := m.deviceName, m.opts.PlayerName
	want := m.prevDevice
	if want == "" || want == current {
		want = "" // this computer's Spotify app
		if player != "" && current != player {
			want = player
		}
	}
	for _, d := range devices {
		if (want != "" && d.Name == want) || (want == "" && m.isComputerApp(d)) {
			return m.moveTo(d)
		}
	}
	// It isn't online. This computer's Spotify app can be started; a phone
	// or a speaker can't be from here.
	wasApp := want == "" || (m.deviceTypes[want] == "Computer" && want != player)
	switch {
	case wasApp && m.opts.OpenSpotifyApp != nil:
		return m.startSpotifyApp(nil)
	case want == "":
		m.toast.Show("No other device to switch to", "Press D to pick one", ToastInfo)
	case want == player:
		m.toast.Show("waxon's player isn't online", "Press D to restart it", ToastError)
	default:
		m.toast.Show(want+" isn't online", "Open Spotify there, then try again", ToastError)
	}
	return m, scheduleAutoDismiss()
}

// moveTo moves playback to dev, saying so at once: a device that has just
// come online can take a few seconds to accept it.
func (m Model) moveTo(dev source.Device) (Model, tea.Cmd) {
	m.toast.Show("Moving playback to "+dev.Name+"…", "", ToastInfo)
	return m, tea.Batch(scheduleAutoDismiss(), m.transferPlayback(dev.ID, dev.Name))
}
