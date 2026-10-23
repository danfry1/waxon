package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// handleKeyVisual handles keys while a range of tracks is selected: motions
// extend the selection, and a/f/o/d act on every selected track.
func (m Model) handleKeyVisual(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.gtracker.Pending() {
		count := m.count
		m.count = 0
		if m.gtracker.Feed(msg.String()) == GActionTop {
			if count > 0 {
				m.tracklist.GotoRow(count - 1)
			} else {
				m.tracklist.GotoTop()
			}
		}
		return m, m.maybeLoadMore()
	}
	if d, ok := m.countDigit(msg); ok {
		m.count = min(m.count*10+d, maxCount)
		return m, nil
	}
	count := m.count
	m.count = 0
	switch {
	case msg.Type == tea.KeyEscape, key.Matches(msg, m.keys.Visual):
		return m.exitVisual(), nil
	case msg.String() == "g":
		m.count = count
		m.gtracker.Feed("g")
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		if count > 0 {
			m.tracklist.GotoRow(count - 1)
		} else {
			m.tracklist.GotoBottom()
		}
	case key.Matches(msg, m.keys.Up):
		m.tracklist.MoveUp(max(1, count))
	case key.Matches(msg, m.keys.Down):
		m.tracklist.MoveDown(max(1, count))
	case key.Matches(msg, m.keys.HalfUp):
		m.tracklist.HalfPageUp()
	case key.Matches(msg, m.keys.HalfDown):
		m.tracklist.HalfPageDown()
	case key.Matches(msg, m.keys.AddQueue):
		tracks, _ := m.tracklist.VisualSelection()
		return m.exitVisual(), m.queueTracks(tracks)
	case key.Matches(msg, m.keys.Like):
		tracks, _ := m.tracklist.VisualSelection()
		return m.exitVisual(), m.likeTracks(tracks)
	case key.Matches(msg, m.keys.Actions):
		tracks, _ := m.tracklist.VisualSelection()
		m = m.exitVisual()
		ids := trackIDs(tracks)
		if len(ids) == 0 {
			return m, nil
		}
		picker := NewPlaylistPickerBatch(m.playlists, ids, m.width, m.height)
		m.playlistPick = &picker
		m.pickReturn = ModeNormal
		m.mode = ModePlaylistPick
		return m, textinput.Blink
	case msg.String() == "d":
		ctx := m.currentPlaylistContext()
		if !ctx.Editable {
			m.toast.Show("Can't remove from this list", "Only your own playlists can be edited", ToastError)
			return m, scheduleAutoDismiss()
		}
		tracks, positions := m.tracklist.VisualSelection()
		return m.exitVisual(), m.removeTracksFromPlaylist(ctx.PlaylistID, ctx.PlaylistName, tracks, positions)
	}
	return m, m.maybeLoadMore()
}

func (m Model) exitVisual() Model {
	m.gtracker.Reset()
	m.count = 0
	m.tracklist.EndVisual()
	m.mode = ModeNormal
	return m
}

// visualHint summarises the selection for the mode line.
func (m Model) visualHint() string {
	tracks, _ := m.tracklist.VisualSelection()
	hint := fmt.Sprintf("%d %s · a queue · f like · o add to playlist", len(tracks), pluralize(len(tracks), "track", "tracks"))
	if m.currentPlaylistContext().Editable {
		hint += " · d remove"
	}
	return hint + " · Esc cancel"
}

func trackIDs(tracks []source.Track) []string {
	ids := make([]string, 0, len(tracks))
	for _, t := range tracks {
		if t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// queueTracks adds tracks to the queue in order. Requests are sequential so
// the queue order matches the selection; a failure stops the batch.
func (m Model) queueTracks(tracks []source.Track) tea.Cmd {
	if len(tracks) == 0 {
		return nil
	}
	if len(tracks) == 1 {
		return m.queueTrack(tracks[0].ID, tracks[0].Name)
	}
	src, ctx := m.source, m.ctx
	return deviceAware(func() tea.Msg {
		for i, t := range tracks {
			if err := src.AddToQueue(ctx, t.ID); err != nil {
				if i == 0 {
					return trackErrorMsg{err} // nothing queued: let device recovery retry
				}
				return queueDoneMsg{count: i, failed: len(tracks) - i}
			}
		}
		return queueDoneMsg{trackName: fmt.Sprintf("%d tracks", len(tracks)), count: len(tracks)}
	})
}

// likedBatchMsg reports tracks saved to Liked Songs from a visual selection.
type likedBatchMsg struct {
	ids    []string
	failed int
}

// likeTracks saves every selected track (a batch only ever likes; unliking
// stays a per-track toggle so a mixed selection can't flip unexpectedly).
func (m Model) likeTracks(tracks []source.Track) tea.Cmd {
	ids := trackIDs(tracks)
	if len(ids) == 0 {
		return nil
	}
	src, ctx := m.source, m.ctx
	return func() tea.Msg {
		for i, id := range ids {
			if err := src.SaveTrack(ctx, id); err != nil {
				if i == 0 {
					return trackErrorMsg{err}
				}
				return likedBatchMsg{ids: ids[:i], failed: len(ids) - i}
			}
		}
		return likedBatchMsg{ids: ids}
	}
}

// addTracksToPlaylist appends several tracks to a playlist in order.
func (m Model) addTracksToPlaylist(pl source.Playlist, ids []string) tea.Cmd {
	src, ctx := m.source, m.ctx
	return func() tea.Msg {
		for i, id := range ids {
			if err := src.AddToPlaylist(ctx, pl.ID, id); err != nil {
				if i == 0 {
					return trackErrorMsg{err}
				}
				return playlistChangedMsg{playlistID: pl.ID, playlistName: pl.Name, added: true, position: -1, count: i, failed: len(ids) - i}
			}
		}
		return playlistChangedMsg{playlistID: pl.ID, playlistName: pl.Name, trackName: fmt.Sprintf("%d tracks", len(ids)), added: true, position: -1, count: len(ids)}
	}
}

// removeTracksFromPlaylist removes the selected rows from a playlist. Rows go
// last-first so each earlier position is still correct when its turn comes.
func (m Model) removeTracksFromPlaylist(playlistID, playlistName string, tracks []source.Track, positions []int) tea.Cmd {
	if len(tracks) == 0 {
		return nil
	}
	if len(tracks) == 1 {
		return m.removeFromPlaylist(playlistID, tracks[0].ID, positions[0], tracks[0].Name)
	}
	src, ctx := m.source, m.ctx
	return func() tea.Msg {
		var removed []int
		for i := len(tracks) - 1; i >= 0; i-- {
			if err := src.RemoveFromPlaylist(ctx, playlistID, tracks[i].ID, positions[i]); err != nil {
				if len(removed) == 0 {
					return trackErrorMsg{err}
				}
				return playlistChangedMsg{playlistID: playlistID, playlistName: playlistName, position: -1, count: len(removed), positions: removed, failed: i + 1}
			}
			removed = append(removed, positions[i])
		}
		return playlistChangedMsg{playlistID: playlistID, playlistName: playlistName, trackName: fmt.Sprintf("%d tracks", len(tracks)), position: -1, count: len(tracks), positions: removed}
	}
}
