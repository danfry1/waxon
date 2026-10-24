package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func TestDeviceAwareConvertsNoActiveDeviceError(t *testing.T) {
	inner := func() tea.Msg { return trackErrorMsg{source.ErrNoActiveDevice} }
	msg := deviceAware(inner)()
	nd, ok := msg.(noActiveDeviceMsg)
	if !ok {
		t.Fatalf("got %T, want noActiveDeviceMsg", msg)
	}
	if nd.retry == nil {
		t.Error("retry should carry the original command")
	}
}

func TestDeviceAwarePassesOtherMessagesThrough(t *testing.T) {
	other := errors.New("boom")
	cases := []tea.Msg{trackErrorMsg{other}, controlDoneMsg{}, queueDoneMsg{trackName: "x"}}
	for _, want := range cases {
		got := deviceAware(func() tea.Msg { return want })()
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

func TestControlCmdWrapsNoActiveDevice(t *testing.T) {
	stub := &StubSource{PlayFn: func(context.Context) error {
		return errors.Join(source.ErrNoActiveDevice, errors.New("404"))
	}}
	m := newTestModel(stub)
	cmd := m.controlCmd(m.source.Play)
	if _, ok := cmd().(noActiveDeviceMsg); !ok {
		t.Fatalf("controlCmd should surface noActiveDeviceMsg")
	}
}

func TestNoActiveDeviceMsgFetchesDevices(t *testing.T) {
	devs := []source.Device{{ID: "d1", Name: "Laptop"}}
	stub := &StubSource{DevicesFn: func(context.Context) ([]source.Device, error) { return devs, nil }}
	m := newTestModel(stub)
	retry := func() tea.Msg { return controlDoneMsg{} }

	result, cmd := m.Update(noActiveDeviceMsg{retry: retry})
	if cmd == nil {
		t.Fatal("expected a device fetch command")
	}
	msg, ok := cmd().(deviceChoicesMsg)
	if !ok {
		t.Fatalf("got %T, want deviceChoicesMsg", cmd())
	}
	if len(msg.devices) != 1 || msg.retry == nil {
		t.Errorf("deviceChoicesMsg = %+v, want 1 device and a retry", msg)
	}
	_ = result
}

func TestDeviceChoicesNoDevicesShowsHelp(t *testing.T) {
	m := newTestModel(&StubSource{})
	result, _ := m.Update(deviceChoicesMsg{devices: nil, retry: func() tea.Msg { return nil }})
	model := result.(Model)
	if !model.toast.Visible() {
		t.Fatal("expected a toast")
	}
	if !strings.Contains(model.toast.View(120), "No Spotify device found") {
		t.Errorf("toast = %q, want no-device guidance", model.toast.View(120))
	}
	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
}

func TestDeviceChoicesSingleDeviceActivatesWithoutPicker(t *testing.T) {
	m := newTestModel(&StubSource{})
	result, cmd := m.Update(deviceChoicesMsg{
		devices: []source.Device{{ID: "d1", Name: "Laptop"}},
		retry:   func() tea.Msg { return controlDoneMsg{} },
	})
	model := result.(Model)
	if model.mode != ModeNormal || model.devices != nil {
		t.Errorf("single device should not open the picker (mode=%v)", model.mode)
	}
	if !strings.Contains(model.toast.View(120), "Playing on Laptop") {
		t.Errorf("toast = %q, want 'Playing on Laptop'", model.toast.View(120))
	}
	if cmd == nil {
		t.Fatal("expected an activation command")
	}
}

func TestActivateAndRetryTransfersThenReplays(t *testing.T) {
	var transferred string
	stub := &StubSource{TransferPlaybackFn: func(_ context.Context, id string) error {
		transferred = id
		return nil
	}}
	m := newTestModel(stub)
	retried := false
	msg := m.activateAndRetry("d1", func() tea.Msg { retried = true; return controlDoneMsg{} })()
	if transferred != "d1" {
		t.Errorf("transferred to %q, want d1", transferred)
	}
	if !retried {
		t.Error("original command was not replayed")
	}
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("got %T, want the retry's controlDoneMsg", msg)
	}
}

func TestDeviceChoicesMultipleDevicesOpensPickerWithPending(t *testing.T) {
	m := newTestModel(&StubSource{})
	retry := func() tea.Msg { return controlDoneMsg{} }
	result, _ := m.Update(deviceChoicesMsg{
		devices: []source.Device{{ID: "d1", Name: "A"}, {ID: "d2", Name: "B"}},
		retry:   retry,
	})
	model := result.(Model)
	if model.mode != ModeDevices || model.devices == nil {
		t.Fatalf("expected device picker to open, mode=%v", model.mode)
	}
	if model.pendingRetry == nil {
		t.Error("pendingRetry should be set")
	}
	if !strings.Contains(model.devices.View(), "No active device") {
		t.Error("picker should explain why it opened")
	}
}

func TestDeviceChoicesKeepsManuallyOpenedPicker(t *testing.T) {
	m := newTestModel(&StubSource{})
	picker := NewDevicePicker([]source.Device{{ID: "d1", Name: "A"}, {ID: "d2", Name: "B"}}, m.width, m.height)
	picker.MoveDown() // user already moved the cursor
	m.devices = &picker
	m.mode = ModeDevices

	result, _ := m.Update(deviceChoicesMsg{
		devices: []source.Device{{ID: "d1", Name: "A"}, {ID: "d2", Name: "B"}},
		retry:   func() tea.Msg { return controlDoneMsg{} },
	})
	model := result.(Model)
	if model.devices.Selected() == nil || model.devices.Selected().ID != "d2" {
		t.Error("late recovery result must not reset the user's picker cursor")
	}
	if model.pendingRetry == nil {
		t.Error("the user's choice should still replay the failed command")
	}
}

func TestDevicePickerEnterWithPendingRetryActivates(t *testing.T) {
	m := newTestModel(&StubSource{})
	picker := NewDevicePicker([]source.Device{{ID: "d1", Name: "A"}, {ID: "d2", Name: "B"}}, m.width, m.height)
	m.devices = &picker
	m.pendingRetry = func() tea.Msg { return controlDoneMsg{} }
	m.mode = ModeDevices

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	result, cmd := result.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)
	if model.mode != ModeNormal || model.devices != nil || model.pendingRetry != nil {
		t.Errorf("picker state not cleared: mode=%v devices=%v pending=%v",
			model.mode, model.devices != nil, model.pendingRetry != nil)
	}
	if !strings.Contains(model.toast.View(120), "Playing on B") {
		t.Errorf("toast = %q, want 'Playing on B'", model.toast.View(120))
	}
	if cmd == nil {
		t.Fatal("expected an activation command")
	}
}

func TestDevicePickerEnterWithoutPendingTransfersOnly(t *testing.T) {
	var transferred string
	stub := &StubSource{TransferPlaybackFn: func(_ context.Context, id string) error {
		transferred = id
		return nil
	}}
	m := newTestModel(stub)
	picker := NewDevicePicker([]source.Device{{ID: "d1", Name: "A"}}, m.width, m.height)
	m.devices = &picker
	m.mode = ModeDevices

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected transfer command")
	}
	msg := cmd()
	if _, ok := msg.(cmdFlashMsg); !ok {
		t.Errorf("got %T, want cmdFlashMsg from plain transfer", msg)
	}
	if transferred != "d1" {
		t.Errorf("transferred to %q, want d1", transferred)
	}
}

func TestDevicePickerEscapeClearsPendingRetry(t *testing.T) {
	m := newTestModel(&StubSource{})
	picker := NewDevicePicker([]source.Device{{ID: "d1", Name: "A"}}, m.width, m.height)
	m.devices = &picker
	m.pendingRetry = func() tea.Msg { return nil }
	m.mode = ModeDevices

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if result.(Model).pendingRetry != nil {
		t.Error("Escape should drop the pending retry")
	}
}

func TestActivateAndRetryTransferFailureSurfacesError(t *testing.T) {
	stub := &StubSource{TransferPlaybackFn: func(context.Context, string) error {
		return errors.New("transfer failed")
	}}
	m := newTestModel(stub)
	msg := m.activateAndRetry("d1", func() tea.Msg { t.Fatal("retry must not run"); return nil })()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("got %T, want trackErrorMsg", msg)
	}
}

func TestTrackErrorMsgFriendlyPlaybackErrors(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{source.ErrPremiumRequired, "Spotify Premium required"},
		{source.ErrNoActiveDevice, "No active Spotify device"},
	}
	for _, tc := range cases {
		m := newTestModel(&StubSource{})
		result, _ := m.Update(trackErrorMsg{tc.err})
		view := result.(Model).toast.View(120)
		if !strings.Contains(view, tc.want) {
			t.Errorf("toast for %v = %q, want %q", tc.err, view, tc.want)
		}
	}
}

func TestDevicesLoadedEmptyExplains(t *testing.T) {
	m := newTestModel(&StubSource{})
	result, _ := m.Update(devicesLoadedMsg{})
	view := result.(Model).toast.View(120)
	if !strings.Contains(view, "No Spotify device found") {
		t.Errorf("toast = %q", view)
	}
}

func TestDeviceChoicesPrefersWaxonPlayer(t *testing.T) {
	var transferred string
	stub := &StubSource{TransferPlaybackFn: func(_ context.Context, id string) error {
		transferred = id
		return nil
	}}
	m := newTestModel(stub).WithOptions(Options{PlayerName: "waxon (box)"})
	result, cmd := m.Update(deviceChoicesMsg{
		devices: []source.Device{{ID: "d1", Name: "Phone"}, {ID: "d2", Name: "waxon (box)"}},
		retry:   func() tea.Msg { return controlDoneMsg{} },
	})
	if model := result.(Model); model.mode != ModeNormal {
		t.Errorf("with waxon's player listed, no picker should open (mode=%v)", model.mode)
	}
	if cmd == nil {
		t.Fatal("expected an activation command")
	}
	cmd()
	if transferred != "d2" {
		t.Errorf("transferred to %q, want waxon's player d2", transferred)
	}
}

func TestDeviceChoicesPlayerNotReady(t *testing.T) {
	m := newTestModel(&StubSource{}).WithOptions(Options{PlayerName: "waxon (box)"})
	result, _ := m.Update(deviceChoicesMsg{retry: func() tea.Msg { return nil }})
	if got := result.(Model).toast.View(120); !strings.Contains(got, "player isn't ready") {
		t.Errorf("toast = %q, want the player-not-ready hint", got)
	}

	// Once it has died, say why instead.
	result, _ = m.Update(PlayerExitedMsg{Err: errors.New("librespot stopped: Bad credentials")})
	result, _ = result.(Model).Update(deviceChoicesMsg{retry: func() tea.Msg { return nil }})
	if got := result.(Model).toast.View(120); !strings.Contains(got, "Bad credentials") {
		t.Errorf("toast = %q, want librespot's error", got)
	}
}

func TestPlayerExitedShowsToast(t *testing.T) {
	m := newTestModel(&StubSource{})
	result, _ := m.Update(PlayerExitedMsg{Err: errors.New("librespot stopped: Bad credentials")})
	if got := result.(Model).toast.View(120); !strings.Contains(got, "player stopped") || !strings.Contains(got, "Bad credentials") {
		t.Errorf("toast = %q", got)
	}
}

// waxon's own player, just activated, answers "Restriction violated" until
// it's ready; the command is retried rather than shown as an error.
func TestActivateAndRetryWaitsOutRestriction(t *testing.T) {
	transfers := 0
	stub := &StubSource{TransferPlaybackFn: func(context.Context, string) error { transfers++; return nil }}
	m := newTestModel(stub)
	tries := 0
	retry := func() tea.Msg {
		tries++
		if tries < 3 {
			return trackErrorMsg{fmt.Errorf("%w: Restriction violated", source.ErrPlaybackRestricted)}
		}
		return controlDoneMsg{}
	}
	msg := m.activateAndRetry("d1", retry)()
	if _, ok := msg.(controlDoneMsg); !ok || tries != 3 || transfers != 1 {
		t.Errorf("got %T after %d tries, %d transfers; want success on the 3rd try, one transfer", msg, tries, transfers)
	}
}

func TestActivateAndRetryGivesUpOnOtherErrors(t *testing.T) {
	m := newTestModel(&StubSource{})
	tries := 0
	retry := func() tea.Msg { tries++; return trackErrorMsg{source.ErrPremiumRequired} }
	if _, ok := m.activateAndRetry("d1", retry)().(trackErrorMsg); !ok || tries != 1 {
		t.Errorf("a non-transient failure should not be retried; tries=%d", tries)
	}
}
