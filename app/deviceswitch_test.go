package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

var (
	playerDev = source.Device{ID: "p", Name: "waxon (Mac)", Type: "Computer"}
	appDev    = source.Device{ID: "a", Name: "Dan's MacBook", Type: "Computer"}
	phoneDev  = source.Device{ID: "ph", Name: "Dan's iPhone", Type: "Smartphone"}
)

// switchStub lists devices and records where playback was moved.
func switchStub(devices ...source.Device) (*StubSource, *[]string) {
	var mu sync.Mutex
	var moved []string
	return &StubSource{
		DevicesFn: func(context.Context) ([]source.Device, error) { return devices, nil },
		TransferPlaybackFn: func(_ context.Context, id string) error {
			mu.Lock()
			defer mu.Unlock()
			moved = append(moved, id)
			return nil
		},
	}, &moved
}

// run feeds cmd's messages back into m until nothing is left to do.
// Commands still running after a moment are timers (toast dismissals), and
// are dropped.
func run(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && steps < 50; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(200 * time.Millisecond):
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		res, more := m.Update(msg)
		m = res.(Model)
		queue = append(queue, more)
	}
	return m
}

func TestSwapGoesBackToThePreviousDevice(t *testing.T) {
	stub, moved := switchStub(playerDev, phoneDev)
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	for _, dev := range []string{phoneDev.Name, playerDev.Name} {
		res, _ := m.Update(trackUpdateMsg{track: &source.Track{ID: "t", DeviceName: dev}})
		m = res.(Model)
	}
	run(t, m, m.switchDevice("-"))
	if len(*moved) != 1 || (*moved)[0] != phoneDev.ID {
		t.Errorf("moved to %v, want the phone it played on before", *moved)
	}
}

// gd from waxon's player goes to the Spotify app, starting it when it's
// closed.
func TestToggleOpensTheSpotifyAppFromThePlayer(t *testing.T) {
	spotifyAppPoll = time.Millisecond
	t.Cleanup(func() { spotifyAppPoll = time.Second })
	var mu sync.Mutex
	online := []source.Device{playerDev}
	opened := 0
	stub, moved := switchStub()
	stub.DevicesFn = func(context.Context) ([]source.Device, error) {
		mu.Lock()
		defer mu.Unlock()
		return online, nil
	}
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name, OpenSpotifyApp: func() error {
		mu.Lock()
		defer mu.Unlock()
		opened++
		online = []source.Device{playerDev, appDev} // it comes online
		return nil
	}})
	m.deviceName = playerDev.Name
	run(t, m, m.switchDevice(toggleHere))
	if opened != 1 || len(*moved) != 1 || (*moved)[0] != appDev.ID {
		t.Errorf("opened %d times, moved to %v; want the app opened once and playback on it", opened, *moved)
	}
}

func TestToggleFromTheSpotifyAppGoesToThePlayer(t *testing.T) {
	stub, moved := switchStub(playerDev, appDev)
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	m.deviceName = appDev.Name
	run(t, m, m.switchDevice(toggleHere))
	if len(*moved) != 1 || (*moved)[0] != playerDev.ID {
		t.Errorf("moved to %v, want waxon's player", *moved)
	}
}

// A phone that's gone offline can't be started from here: say so.
func TestSwapToOfflinePhoneExplains(t *testing.T) {
	stub, moved := switchStub(playerDev)
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name, OpenSpotifyApp: func() error {
		t.Error("the Spotify app shouldn't be opened for a phone")
		return nil
	}})
	m.noteDevice(phoneDev)
	m.deviceName, m.prevDevice = playerDev.Name, phoneDev.Name
	m = run(t, m, m.switchDevice("-"))
	if len(*moved) != 0 || !m.toast.Visible() {
		t.Errorf("moved to %v, toast %q", *moved, m.toast.message)
	}
}

func TestDeviceByName(t *testing.T) {
	stub, moved := switchStub(playerDev, appDev, phoneDev)
	m := newTestModel(stub)
	run(t, m, m.runCommand(Command{Type: CmdDevice, StrArg: "iphone"}))
	if len(*moved) != 1 || (*moved)[0] != phoneDev.ID {
		t.Errorf(":device iphone moved to %v", *moved)
	}
	m = run(t, m, m.runCommand(Command{Type: CmdDevice, StrArg: "mac"})) // waxon (Mac), Dan's MacBook
	if m.mode != ModeDevices || m.devices == nil || len(m.devices.devices) != 2 {
		t.Errorf("two matches should open the picker with them")
	}
}

func TestPickerOffersToOpenTheSpotifyApp(t *testing.T) {
	launcher := func() error { return nil }
	has := func(devs []source.Device) bool {
		for _, d := range devs {
			if d.ID == openSpotifyDeviceID {
				return true
			}
		}
		return false
	}
	m := newTestModel(&StubSource{}).WithOptions(Options{PlayerName: playerDev.Name, OpenSpotifyApp: launcher})
	if !has(m.withOpenSpotify([]source.Device{playerDev})) {
		t.Error("with only waxon's player online, the picker should offer the app")
	}
	if has(m.withOpenSpotify([]source.Device{playerDev, appDev})) {
		t.Error("with the app online, it shouldn't be offered")
	}
	if has(newTestModel(&StubSource{}).withOpenSpotify(nil)) {
		t.Error("without the app installed, it shouldn't be offered")
	}
}

// Spotify answers a transfer to a device that has only just come online
// with a server error for a few seconds; waxon keeps trying.
func TestTransferRetriesWhileTheDeviceStarts(t *testing.T) {
	saved := transferRetryWaits
	transferRetryWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { transferRetryWaits = saved })
	tries := 0
	stub := &StubSource{TransferPlaybackFn: func(context.Context, string) error {
		tries++
		if tries < 3 {
			return fmt.Errorf("%w: Server error.", source.ErrDeviceUnavailable)
		}
		return nil
	}}
	m := newTestModel(stub)
	if msg := m.transferPlayback(appDev.ID, appDev.Name)(); tries != 3 {
		t.Errorf("tried %d times (%T), want success on the third", tries, msg)
	} else if _, ok := msg.(playbackMovedMsg); !ok {
		t.Errorf("got %T, want the transfer to succeed", msg)
	}
	tries = -10 // never comes up
	msg := m.transferPlayback(appDev.ID, appDev.Name)()
	e, ok := msg.(trackErrorMsg)
	if !ok || !errors.Is(e.err, source.ErrDeviceUnavailable) {
		t.Fatalf("got %#v, want the error after the last try", msg)
	}
	if title, _, ok := friendlyPlaybackError(e.err); !ok || strings.Contains(title, "Server error") {
		t.Errorf("toast title = %q, want an explanation", title)
	}
}

// Spotify refuses a command ("Restriction violated") sent to a device that
// has gone, such as a Spotify app that was quit while playing. With no
// device really active, waxon activates its own player and replays it.
func TestRefusedCommandOnGoneDeviceRecovers(t *testing.T) {
	saved := activationRetryWaits
	activationRetryWaits = []time.Duration{time.Millisecond}
	t.Cleanup(func() { activationRetryWaits = saved })
	plays := 0
	stub, moved := switchStub(playerDev) // listed, not active
	stub.PlayTrackFn = func(context.Context, string, string) error {
		plays++
		if plays == 1 {
			return fmt.Errorf("%w: Restriction violated", source.ErrPlaybackRestricted)
		}
		return nil
	}
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	m = run(t, m, m.playTrack("spotify:track:x", "spotify:album:y"))
	if len(*moved) != 1 || (*moved)[0] != playerDev.ID || plays != 2 {
		t.Errorf("moved to %v, played %d times; want waxon's player activated and the song replayed", *moved, plays)
	}
}

// A device that is active and still refuses: that's the answer, shown as is.
func TestRefusedCommandOnActiveDeviceIsShown(t *testing.T) {
	saved := activationRetryWaits
	activationRetryWaits = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { activationRetryWaits = saved })
	active := playerDev
	active.IsActive = true
	stub, moved := switchStub(active)
	stub.PlayTrackFn = func(context.Context, string, string) error {
		return fmt.Errorf("%w: Restriction violated", source.ErrPlaybackRestricted)
	}
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	m = run(t, m, m.playTrack("spotify:track:x", "spotify:album:y"))
	if len(*moved) != 0 || !m.toast.Visible() || m.toast.toastType != ToastError {
		t.Errorf("moved to %v, toast %q; want the refusal shown", *moved, m.toast.message)
	}
}

// Two toggles in a row go there and back, even before a poll has caught
// up with the first.
func TestToggleTwiceGoesThereAndBack(t *testing.T) {
	stub, moved := switchStub(playerDev, appDev)
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	m.deviceName = playerDev.Name
	m = run(t, m, m.switchDevice(toggleHere))
	m = run(t, m, m.switchDevice(toggleHere))
	if len(*moved) != 2 || (*moved)[0] != appDev.ID || (*moved)[1] != playerDev.ID {
		t.Errorf("moved to %v, want the app and then back to waxon's player", *moved)
	}
}

// gd is strictly this computer's two: having been on a phone in between
// doesn't send it there (":device -" does).
func TestToggleIgnoresThePreviousDevice(t *testing.T) {
	stub, moved := switchStub(playerDev, appDev, phoneDev)
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	m.deviceName, m.prevDevice = playerDev.Name, phoneDev.Name
	run(t, m, m.switchDevice(toggleHere))
	if len(*moved) != 1 || (*moved)[0] != appDev.ID {
		t.Errorf("gd moved to %v, want the Spotify app", *moved)
	}
}

func TestGDKeyToggles(t *testing.T) {
	stub, moved := switchStub(playerDev, appDev)
	m := newTestModel(stub).WithOptions(Options{PlayerName: playerDev.Name})
	m.deviceName = playerDev.Name
	res, _ := m.Update(runeKey("g"))
	m = res.(Model)
	res, cmd := m.Update(runeKey("d"))
	run(t, res.(Model), cmd)
	if len(*moved) != 1 || (*moved)[0] != appDev.ID {
		t.Errorf("gd moved to %v, want the Spotify app", *moved)
	}
}
