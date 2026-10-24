package app

import (
	"context"
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
// been activated. refused is set when Spotify instead refused the command
// ("Restriction violated"), which it also does when the device it still
// counts as playing has gone (a Spotify app that was quit); it's shown if a
// device turns out to be active after all.
type noActiveDeviceMsg struct {
	retry   tea.Cmd
	refused error
}

// deviceChoicesMsg carries the device list fetched in response to a
// noActiveDeviceMsg, along with the command to retry once a device is chosen.
type deviceChoicesMsg struct {
	devices []source.Device
	retry   tea.Cmd
	refused error
}

// deviceAware wraps a playback command so that a no-active-device failure is
// turned into a recoverable noActiveDeviceMsg instead of an error toast. The
// original command is carried along so it can be replayed after a device is
// activated.
func deviceAware(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		if e, ok := msg.(trackErrorMsg); ok {
			switch {
			case errors.Is(e.err, source.ErrNoActiveDevice):
				return noActiveDeviceMsg{retry: cmd}
			case errors.Is(e.err, source.ErrPlaybackRestricted):
				return noActiveDeviceMsg{retry: cmd, refused: e.err}
			}
		}
		return msg
	}
}

// fetchDevicesForRetry lists devices so the UI can decide how to recover from a
// no-active-device failure (auto-activate, pick, or explain).
func (m Model) fetchDevicesForRetry(retry tea.Cmd, refused error) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		devices, err := src.Devices(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return deviceChoicesMsg{devices: devices, retry: retry, refused: refused}
	}
}

// activateAndRetry transfers playback to deviceID, waits for Spotify to settle,
// then replays the original command. A retry that still fails with
// ErrNoActiveDevice surfaces as a plain error toast (no second recovery loop).
func (m Model) activateAndRetry(deviceID string, retry tea.Cmd) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		if err := transferRetrying(ctx, src, deviceID); err != nil {
			return trackErrorMsg{err}
		}
		if retry == nil {
			return controlDoneMsg{}
		}
		// A device that was only just activated (waxon's own player, a
		// woken phone) can refuse the command until it's ready; try again a
		// little later, a couple of times.
		var msg tea.Msg
		for _, wait := range activationRetryWaits {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wait):
			}
			msg = retry()
			if !deviceNotReady(msg) {
				return msg
			}
		}
		return msg
	}
}

// deviceNotReady reports whether msg is a failure a just-activated device
// gives while it's still getting ready.
func deviceNotReady(msg tea.Msg) bool {
	e, ok := msg.(trackErrorMsg)
	return ok && (errors.Is(e.err, source.ErrPlaybackRestricted) || errors.Is(e.err, source.ErrNoActiveDevice))
}

// activationRetryWaits are the pauses before each try of the command that
// needed a device, after activating one.
var activationRetryWaits = []time.Duration{activationSettleDelay, 700 * time.Millisecond, 1500 * time.Millisecond}

// handleDeviceChoices decides how to recover from a no-active-device failure:
//   - no devices: explain how to get one (nothing we can do remotely)
//   - one device: activate it and replay the command transparently
//   - several: open the device picker with the command pending
func (m Model) handleDeviceChoices(msg deviceChoicesMsg) (Model, tea.Cmd) {
	if msg.refused != nil {
		for _, dev := range msg.devices {
			if dev.IsActive {
				// A device that has only just taken over refuses commands
				// for a moment: try again shortly before giving up.
				return m, retryRefused(m.ctx, msg.retry, msg.refused)
			}
		}
	}
	// waxon's own player is what the user set up to play on.
	if name := m.opts.PlayerName; name != "" {
		for _, dev := range msg.devices {
			if dev.Name == name {
				return m, m.activateAndRetry(dev.ID, msg.retry)
			}
		}
		if len(msg.devices) == 0 {
			detail := "It can take a few seconds to appear; try again"
			if m.playerErr != nil {
				detail = m.playerErr.Error()
			}
			m.toast.Show("waxon's player isn't ready", detail, ToastError)
			return m, scheduleAutoDismiss()
		}
	}
	switch len(msg.devices) {
	case 0:
		if m.canOfferDevices() {
			return m.openPlayHerePicker()
		}
		m.toast.Show("No Spotify device found", m.noDeviceHint(), ToastError)
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
		picker := NewDevicePicker(m.withPlayHere(msg.devices), m.width, m.height)
		picker.SetPrompt("No active device — pick one to continue")
		m.devices = &picker
		m.pendingRetry = msg.retry
		m.mode = ModeDevices
		return m, nil
	}
}

// noDeviceHint is the fix offered when Spotify lists no devices: open a
// Spotify app, or, if waxon's own player isn't set up, set it up.
func (m Model) noDeviceHint() string {
	if m.opts.PlayerName == "" && m.opts.PlayerSetup == nil {
		return "Open Spotify on a device, or play here: waxon player login"
	}
	return "Open Spotify on any device, then try again (D to pick)"
}

// playerProblemMsg carries Options.PlayerProblem to the UI.
type playerProblemMsg struct{ problem string }

func (m Model) reportPlayerProblem() tea.Cmd {
	if m.opts.PlayerProblem == "" {
		return nil
	}
	p := m.opts.PlayerProblem
	return func() tea.Msg { return playerProblemMsg{p} }
}

// PlayerExitedMsg reports that waxon's own player (librespot) stopped
// unexpectedly.
type PlayerExitedMsg struct{ Err error }

func (m Model) handlePlayerProblem(msg playerProblemMsg) (Model, tea.Cmd) {
	m.toast.Show("waxon's player didn't start", msg.problem, ToastError)
	return m, scheduleAutoDismiss()
}

func (m Model) handlePlayerExited(msg PlayerExitedMsg) (Model, tea.Cmd) {
	m.playerErr = msg.Err
	detail := "Run 'waxon player login' if this keeps happening"
	if msg.Err != nil {
		detail = msg.Err.Error()
	}
	// It's gone: let the device picker offer "play here" again, which signs
	// it in afresh and restarts it.
	m.opts.PlayerName = ""
	m.toast.Show("waxon's player stopped: press D to restart it", detail, ToastError)
	return m, scheduleAutoDismiss()
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
	case errors.Is(err, source.ErrDeviceUnavailable):
		return "Spotify couldn't play on that device", "It may still be starting; try again in a moment", true
	case errors.Is(err, source.ErrPlaybackRestricted):
		return "Spotify refused that on this device", "A device that has just started can take a moment; try again", true
	case errors.Is(err, source.ErrForbidden):
		return "Not available with this Spotify app", "Spotify blocks this for development-mode apps (see README)", true
	}
	return "", "", false
}

// retryRefused replays retry, refused by an active device, after the
// activation pauses; still refused, it shows refused.
func retryRefused(ctx context.Context, retry tea.Cmd, refused error) tea.Cmd {
	return func() tea.Msg {
		for _, wait := range activationRetryWaits {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wait):
			}
			if msg := retry(); !deviceNotReady(msg) {
				return msg
			}
		}
		return trackErrorMsg{refused}
	}
}
