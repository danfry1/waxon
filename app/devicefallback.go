package app

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// activationSettleDelay is how long to wait after transferring playback to a
// freshly activated device before re-issuing the original command. Spotify
// needs a moment to register the new active device; retrying immediately can
// 404 again.
const activationSettleDelay = 400 * time.Millisecond

// noActiveDeviceMsg is emitted when a playback command failed because Spotify
// has no active device. retry re-runs the original command once a device has
// been activated.
type noActiveDeviceMsg struct{ retry tea.Cmd }

// deviceChoicesMsg carries the device list fetched in response to a
// noActiveDeviceMsg, along with the command to retry once a device is chosen.
type deviceChoicesMsg struct {
	devices []source.Device
	retry   tea.Cmd
}

// deviceAware wraps a playback command so that a no-active-device failure is
// turned into a recoverable noActiveDeviceMsg instead of an error toast. The
// original command is carried along so it can be replayed after a device is
// activated.
func deviceAware(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		if e, ok := msg.(trackErrorMsg); ok && errors.Is(e.err, source.ErrNoActiveDevice) {
			return noActiveDeviceMsg{retry: cmd}
		}
		return msg
	}
}

// fetchDevicesForRetry lists devices so the UI can decide how to recover from a
// no-active-device failure (auto-activate, pick, or explain).
func (m Model) fetchDevicesForRetry(retry tea.Cmd) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		devices, err := src.Devices(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return deviceChoicesMsg{devices: devices, retry: retry}
	}
}

// activateAndRetry transfers playback to deviceID, waits for Spotify to settle,
// then replays the original command. A retry that still fails with
// ErrNoActiveDevice surfaces as a plain error toast (no second recovery loop).
func (m Model) activateAndRetry(deviceID string, retry tea.Cmd) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		if err := src.TransferPlayback(ctx, deviceID); err != nil {
			return trackErrorMsg{err}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(activationSettleDelay):
		}
		if retry == nil {
			return controlDoneMsg{}
		}
		return retry()
	}
}

// handleDeviceChoices decides how to recover from a no-active-device failure:
//   - no devices: explain how to get one (nothing we can do remotely)
//   - one device: activate it and replay the command transparently
//   - several: open the device picker with the command pending
func (m Model) handleDeviceChoices(msg deviceChoicesMsg) (Model, tea.Cmd) {
	switch len(msg.devices) {
	case 0:
		m.toast.Show("No Spotify device found",
			"Open Spotify on any device, then try again (D to pick)", ToastError)
		return m, scheduleAutoDismiss()
	case 1:
		dev := msg.devices[0]
		m.toast.Show("Playing on "+dev.Name, "", ToastInfo)
		return m, tea.Batch(scheduleAutoDismiss(), m.activateAndRetry(dev.ID, msg.retry))
	default:
		if m.mode == ModeDevices && m.devices != nil {
			// The user already opened the picker (D) while the lookup was in
			// flight. Keep their picker and cursor; just make their choice also
			// replay the failed command.
			m.devices.SetPrompt("No active device — pick one to continue")
			m.pendingRetry = msg.retry
			return m, nil
		}
		picker := NewDevicePicker(msg.devices, m.width, m.height)
		picker.SetPrompt("No active device — pick one to continue")
		m.devices = &picker
		m.pendingRetry = msg.retry
		m.mode = ModeDevices
		return m, nil
	}
}

// friendlyPlaybackError rewrites known playback failures into actionable toast
// text. Returns ok=false for errors that should be shown as-is.
func friendlyPlaybackError(err error) (title, detail string, ok bool) {
	switch {
	case errors.Is(err, source.ErrPremiumRequired):
		return "Spotify Premium required", "Playback control needs a Premium account", true
	case errors.Is(err, source.ErrNoActiveDevice):
		return "No active Spotify device", "Open Spotify on any device, or press D", true
	case errors.Is(err, source.ErrInsufficientScope):
		return "Permission needed", "Run 'waxon auth' once to allow playlist changes", true
	case errors.Is(err, source.ErrForbidden):
		return "Not available with this Spotify app", "Spotify blocks this for development-mode apps (see README)", true
	}
	return "", "", false
}
