package app

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Keys that work the same in the main view and in Now Playing: playback,
// the playing track (F, O), switching device or mode, and opening search,
// : and help. An overlay opened from Now Playing returns to it.

// seekStep is how far one press of [ or ] seeks; a count multiplies it
// (6] is half a minute).
const seekStep = 5 * time.Second

// handleGlobalKey handles msg if it's one of the shared keys; count is a
// count typed before it (0 for none).
func (m Model) handleGlobalKey(msg tea.KeyMsg, count int) (Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.PlayPause):
		if m.track != nil && m.track.Playing {
			return m, m.controlCmd(m.source.Pause), true
		}
		return m, m.controlCmd(m.source.Play), true
	case key.Matches(msg, m.keys.Next):
		return m, m.controlCmd(m.source.Next), true
	case key.Matches(msg, m.keys.Prev):
		return m, m.controlCmd(m.source.Previous), true
	case key.Matches(msg, m.keys.SeekFwd, m.keys.SeekBack):
		if m.track == nil {
			return m, nil, true
		}
		step := seekStep * time.Duration(max(1, count))
		if key.Matches(msg, m.keys.SeekBack) {
			step = -step
		}
		return m, m.seekRelative(step), true
	case key.Matches(msg, m.keys.VolUp, m.keys.VolDown, m.keys.Shuffle, m.keys.Repeat):
		return m, m.playbackSettingKey(msg), true
	case key.Matches(msg, m.keys.Mute):
		return m.toggleMute()
	case key.Matches(msg, m.keys.LikePlaying):
		if m.track == nil {
			m.toast.Show("No track playing", "", ToastError)
			return m, scheduleAutoDismiss(), true
		}
		return m, m.toggleLike(m.track.ID, m.liked), true
	case key.Matches(msg, m.keys.ActionsPlaying):
		if m.track == nil {
			m.toast.Show("No track playing", "", ToastError)
			return m, scheduleAutoDismiss(), true
		}
		return m.openPlayingActions(), nil, true
	case key.Matches(msg, m.keys.Mode):
		return m, m.switchDevice(toggleHere), true
	case key.Matches(msg, m.keys.Devices):
		return m, m.fetchDevices(), true
	case key.Matches(msg, m.keys.Search):
		s := NewSearch(m.width, m.height)
		m.search = &s
		m.mode = ModeSearch
		return m, nil, true
	case key.Matches(msg, m.keys.Command):
		m.mode = ModeCommand
		m.cmdInput = ""
		return m, nil, true
	case key.Matches(msg, m.keys.Help):
		m.mode = ModeHelp
		return m, nil, true
	}
	return m, nil, false
}

// returnsToOpener reports whether mode is one of the overlays that close
// back to where they were opened (Model.overlayReturn).
func returnsToOpener(mode Mode) bool {
	switch mode {
	case ModeSearch, ModeHelp, ModeCommand, ModeDevices:
		return true
	}
	return false
}

// leaveOverlay closes search, help, the command line or the device picker,
// back to Now Playing if that's where it was opened.
func (m *Model) leaveOverlay() {
	m.mode = ModeNormal
	if m.overlayReturn == ModeNowPlaying {
		m.mode = ModeNowPlaying
	}
}

// openPlayingActions opens the actions menu for the playing track.
func (m Model) openPlayingActions() Model {
	popup := NewTrackActions(m.track.Name, m.track.Artist, m.track.URI,
		m.playbackContextURI, m.track.ArtistID, m.track.AlbumID, m.liked, m.width, m.height)
	m.actions = &popup
	m.actionsReturn = m.mode
	m.mode = ModeActions
	return m
}

// toggleMute sets the volume to 0, or back to what it was before.
func (m Model) toggleMute() (Model, tea.Cmd, bool) {
	if m.volume > 0 {
		m.muteRestore = m.volume
		return m, m.runCommand(Command{Type: CmdVolume, IntArg: 0}), true
	}
	restore := m.muteRestore
	if restore <= 0 {
		restore = 50
	}
	m.muteRestore = 0
	return m, m.runCommand(Command{Type: CmdVolume, IntArg: restore}), true
}
