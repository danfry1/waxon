package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Switching devices without the picker: gd toggles this computer between
// waxon's player and the Spotify app, ":device -" goes back to the previous
// device, ":device <name>" picks one by name. Like the picker's "open the
// Spotify app" row, they can start this computer's Spotify app when it
// isn't running (Options.OpenSpotifyApp) and move playback to it once it
// comes online.

// Queries for switchDevice besides a name: back to the previous device, and
// gd's toggle between waxon's player and this computer's Spotify app.
const (
	previousDevice = "-"
	toggleHere     = "\x00here"
)

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
	// deviceTargetMsg carries the devices for a switch by name (query), to
	// the previous device (previousDevice) or gd's toggle (toggleHere).
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
		gen   int // the switch it belongs to (Model.switchGen)
	}
)

// isOtherApp reports whether dev is a Spotify app on some computer: not a
// phone or speaker, and not a waxon player (this one or another machine's).
func (m Model) isOtherApp(dev source.Device) bool {
	return dev.Type == "Computer" && dev.Name != m.opts.PlayerName && !strings.HasPrefix(dev.Name, "waxon (")
}

// isThisComputersApp reports whether dev is the Spotify app on this
// computer: a Spotify app named like this machine (the app names itself
// after the computer). Another computer's app on the account isn't one.
func (m Model) isThisComputersApp(dev source.Device) bool {
	if !m.isOtherApp(dev) {
		return false
	}
	name := deviceKey(dev.Name)
	for _, n := range m.opts.ThisComputer {
		if k := deviceKey(n); k != "" && k == name {
			return true
		}
	}
	return false
}

// deviceKey is name reduced to lower-case letters and digits, so
// "Daniel’s MacBook Pro" matches the host name "Daniels-MacBook-Pro".
func deviceKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// withOpenSpotify adds the "open the Spotify app" row to a device list
// when the app is installed here and no computer's Spotify app is listed.
func (m Model) withOpenSpotify(devices []source.Device) []source.Device {
	if m.opts.OpenSpotifyApp == nil {
		return devices
	}
	for _, d := range devices {
		if m.isThisComputersApp(d) {
			return devices
		}
	}
	return append(devices[:len(devices):len(devices)],
		source.Device{ID: openSpotifyDeviceID, Name: "This computer: open the Spotify app", Type: "+"})
}

// startSpotifyApp starts the Spotify app and moves playback to it (or, with
// retry, replays the command waiting on a device) once it's online.
func (m Model) startSpotifyApp(retry tea.Cmd) (Model, tea.Cmd) {
	if m.spotifyAppOpening {
		m.toast.Show("Still opening the Spotify app…", "waxon controls it once it's ready", ToastInfo)
		return m, scheduleAutoDismiss()
	}
	m.switchGen++
	m.spotifyAppOpening = true
	m.toast.Show("Opening the Spotify app…", "waxon controls it once it's ready", ToastInfo)
	return m, tea.Batch(scheduleAutoDismiss(), m.waitForSpotifyApp(retry, m.switchGen))
}

func (m Model) waitForSpotifyApp(retry tea.Cmd, gen int) tea.Cmd {
	src, ctx, launch := m.source, m.ctx, m.opts.OpenSpotifyApp
	return func() tea.Msg {
		// The app is the computer that's named like this one, or failing
		// that one that wasn't online before it was started.
		seen := map[string]bool{}
		if before, err := src.Devices(ctx); err == nil {
			for _, d := range before {
				seen[d.ID] = true
			}
		}
		if err := launch(); err != nil {
			return spotifyAppMsg{err: err, gen: gen}
		}
		wait, cancel := context.WithTimeout(ctx, spotifyAppWait)
		defer cancel()
		tick := time.NewTicker(spotifyAppPoll)
		defer tick.Stop()
		for {
			select {
			case <-wait.Done():
				return spotifyAppMsg{err: errSpotifyAppSlow, gen: gen}
			case <-tick.C:
			}
			devices, err := src.Devices(wait)
			if err != nil {
				continue // it's starting; Spotify may not answer every time
			}
			for _, d := range devices {
				if m.isThisComputersApp(d) || (m.isOtherApp(d) && !seen[d.ID]) {
					return spotifyAppMsg{dev: d, retry: retry, gen: gen}
				}
			}
		}
	}
}

func (m Model) spotifyAppReady(msg spotifyAppMsg) (Model, tea.Cmd) {
	if msg.gen != m.switchGen {
		return m, nil // superseded: playback was moved some other way since
	}
	m.spotifyAppOpening = false
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

// switchDevice looks up the devices for a switch by name, to the previous
// device (previousDevice) or for gd's toggle (toggleHere).
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
	switch msg.query {
	case previousDevice:
		return m.swapDevice(msg.devices)
	case toggleHere:
		return m.toggleHere(msg.devices)
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

// toggleHere moves playback between waxon's player and this computer's
// Spotify app: to the app from the player, to the player from anywhere
// else. The app is started if it isn't running.
func (m Model) toggleHere(devices []source.Device) (Model, tea.Cmd) {
	want := "" // this computer's Spotify app
	if player := m.opts.PlayerName; player != "" && m.deviceName != player {
		want = player
	}
	return m.switchTo(want, devices)
}

// swapDevice moves playback to the device played on before this one. With
// none, it toggles as gd does.
func (m Model) swapDevice(devices []source.Device) (Model, tea.Cmd) {
	want := m.prevDevice
	if want == "" || want == m.deviceName {
		return m.toggleHere(devices)
	}
	return m.switchTo(want, devices)
}

// switchTo moves playback to the device named want ("" for this
// computer's Spotify app), starting the app if that's the one and it isn't
// running.
func (m Model) switchTo(want string, devices []source.Device) (Model, tea.Cmd) {
	player := m.opts.PlayerName
	for _, d := range devices {
		if (want != "" && d.Name == want) || (want == "" && m.isThisComputersApp(d)) {
			return m.moveTo(d)
		}
	}
	// It isn't online. This computer's Spotify app can be started; a phone
	// or a speaker can't be from here.
	wasApp := want == "" || m.isThisComputersApp(source.Device{Name: want, Type: m.deviceTypes[want]})
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
	m.supersedeSwitch()
	title, _ := m.modeLabel(dev.Name)
	m.toast.Show(title+"…", "", ToastInfo)
	return m, tea.Batch(scheduleAutoDismiss(), m.transferPlayback(dev.ID, dev.Name))
}

// modeLabel says what playing on device means for this computer: waxon
// playing by itself, or controlling the Spotify app (gd's two modes), or
// just which device it is.
func (m Model) modeLabel(device string) (title, detail string) {
	switch {
	case device != "" && device == m.opts.PlayerName:
		return "Playing in waxon", "gd hands it to the Spotify app"
	case m.isThisComputersApp(source.Device{Name: device, Type: m.deviceTypes[device]}):
		return "Controlling the Spotify app", device + " · gd plays in waxon again"
	}
	return "Playing on " + device, ""
}

// supersedeSwitch drops a Spotify app still being opened for an earlier
// switch: playback is going elsewhere now, and it mustn't be taken back
// when the app comes online.
func (m *Model) supersedeSwitch() {
	m.switchGen++
	m.spotifyAppOpening = false
}
