package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// The queue, shown in the track list (gq): what's playing, then the tracks
// you queued, then what plays next from the playlist or album, as the
// Spotify apps show it. Spotify's queue endpoint returns the last two as one
// list; they're told apart by where the playing context carries on.

// queueViewURI marks the track list as showing the queue (its contextURI:
// the queue isn't a context tracks can be played in).
const queueViewURI = "waxon:queue"

// skipPause is the pause between the skips that move to a queued track, so
// Spotify takes them one at a time.
const skipPause = 150 * time.Millisecond

// showingQueue reports whether the track list is showing the queue.
func (m Model) showingQueue() bool { return m.tracklist.ContextURI() == queueViewURI }

// openQueue shows the queue in the track list and fetches it.
func (m *Model) openQueue() tea.Cmd {
	m.pushNav()
	m.focusPane = PaneTrackList
	m.tracklist.SetLoading("Queue")
	return m.fetchQueue()
}

// queueParts splits the queue into the tracks the user queued and those
// that follow from the playing context, when the context is a playlist
// whose tracks waxon has (and shuffle is off, which would reorder them);
// otherwise it's all "up next" and ok is false. The context part is the
// first place from which the rest of the queue runs in the context's
// order, so a queued copy of the context's next track stays queued.
func (m Model) queueParts(queue []source.Track) (queued, fromContext []source.Track, ok bool) {
	if m.shuffleOn || m.track == nil {
		return nil, queue, false
	}
	next := m.contextNext(m.track.ID)
	if len(next) == 0 {
		return nil, queue, false
	}
	for i := range queue {
		if followsContext(queue[i:], next) {
			return queue[:i], queue[i:], true
		}
	}
	return nil, queue, false
}

// followsContext reports whether rest starts with next's tracks in order,
// as far as both go (past the context's end, Spotify may carry on with
// tracks of its own).
func followsContext(rest, next []source.Track) bool {
	for j := range min(len(rest), len(next)) {
		if rest[j].ID != next[j].ID {
			return false
		}
	}
	return true
}

// contextNext is what follows trackID in the playing context, from the
// track cache: nil when the context isn't a cached playlist.
func (m Model) contextNext(trackID string) []source.Track {
	for _, pl := range m.playlists {
		if !m.isPlayingFrom(pl) {
			continue
		}
		cached, ok := m.trackCache[pl.ID]
		if !ok {
			return nil
		}
		for i, t := range cached.tracks {
			if t.ID == trackID {
				return cached.tracks[i+1:]
			}
		}
		return nil
	}
	return nil
}

// contextName names the playing context for the "Next from" heading.
func (m Model) contextName() string {
	for _, pl := range m.playlists {
		if m.isPlayingFrom(pl) {
			return strings.TrimPrefix(pl.Name, "♥ ")
		}
	}
	return ""
}

// isPlayingFrom reports whether pl is the playing context (Liked Songs
// plays as the account's collection).
func (m Model) isPlayingFrom(pl source.Playlist) bool {
	uri := m.playbackContextURI
	return pl.URI == uri || (pl.ID == likedPlaylistID && isAccountCollection(uri))
}

// queueRows builds the queue view: Now playing, Next in queue, Next from
// the context (or Up next when they can't be told apart). Each queue track
// keeps its place in Spotify's queue, for skipping to it.
func (m Model) queueRows(queue []source.Track) (rows []source.Track, info string) {
	queue = stripTrailingDupes(queue)
	queued, rest, split := m.queueParts(queue)
	section := func(name string, n int) {
		if len(rows) > 0 {
			rows = append(rows, source.Track{IsSeparator: true})
		}
		count := ""
		if n > 0 {
			count = strconv.Itoa(n)
		}
		rows = append(rows, source.Track{Name: name, Artist: count, IsSeparator: true})
	}
	if m.track != nil {
		section("Now playing", 0)
		rows = append(rows, *m.track)
	}
	if len(queued) > 0 {
		section("Next in queue", len(queued))
		rows = append(rows, queued...)
	}
	if len(rest) > 0 {
		name := "Up next"
		if split {
			name = "Next from " + m.contextName()
			if m.contextName() == "" {
				name = "Next"
			}
		}
		section(name, len(rest))
		rows = append(rows, rest...)
	}
	var total time.Duration
	for _, t := range queue {
		total += t.Duration
	}
	parts := []string{}
	if len(queued) > 0 {
		parts = append(parts, fmt.Sprintf("%d queued", len(queued)))
	}
	parts = append(parts, fmt.Sprintf("%d up next", len(rest)))
	if total > 0 {
		parts = append(parts, durationLabel(total))
	}
	if len(queue) == 0 {
		parts = []string{"nothing queued"}
	}
	return rows, strings.Join(parts, " · ")
}

// durationLabel is "41m" or "2h 10m".
func durationLabel(d time.Duration) string {
	if h := int(d.Hours()); h > 0 {
		return fmt.Sprintf("%dh %dm", h, int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// showQueue puts the queue in the track list, keeping the cursor where it
// was when it's a refresh of the queue already on screen.
func (m *Model) showQueue(queue []source.Track) {
	rows, info := m.queueRows(queue)
	cursor := m.tracklist.Cursor()
	playing := ""
	if m.track != nil {
		playing = m.track.ID
	}
	// A refresh keeps the cursor, unless the queue moved on (a new track
	// playing), when the rows under it are different ones.
	refresh := m.showingQueue() && !m.tracklist.loading && playing == m.queueShownFor
	m.queueShownFor = playing
	m.tracklist.SetTracks(rows, "Queue", queueViewURI)
	m.tracklist.SetHeaderInfo(info)
	m.tracklist.SetArt("")
	if m.track != nil {
		m.tracklist.SetNowPlaying(m.track.ID)
	}
	if refresh {
		m.tracklist.GotoRow(cursor)
	} else {
		m.tracklist.GotoTop()
		m.tracklist.MoveDown(0) // off the heading, onto the first track
	}
}

// queueSkips is how many skips reach the queue view's row at display
// index sel: the tracks from after Now playing up to it. 0 is the row
// that's playing (or a heading).
func queueSkips(rows []source.Track, sel int) int {
	if sel < 0 || sel >= len(rows) || rows[sel].IsSeparator {
		return 0
	}
	skips, afterNow := 0, false
	for i, t := range rows {
		switch {
		case t.IsSeparator:
			if t.Name != "" && t.Name != "Now playing" {
				afterNow = true
			}
		case afterNow:
			skips++
		}
		if i == sel {
			break
		}
	}
	if !afterNow {
		return 0
	}
	return skips
}

// skipToQueued plays the queue view's selected track by skipping forward
// to it, as the Spotify apps do, so what's queued after it stays.
func (m Model) skipToQueued() (Model, tea.Cmd) {
	sel := m.tracklist.SelectedTrack()
	skips := queueSkips(m.tracklist.displayTracks(), m.tracklist.Cursor())
	if sel == nil || skips == 0 {
		return m, nil // the track playing, or a heading
	}
	src, ctx := m.source, m.ctx
	name := sel.Name
	return m, deviceAware(func() tea.Msg {
		for i := range skips {
			if i > 0 {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(skipPause):
				}
			}
			if err := src.Next(ctx); err != nil {
				return trackErrorMsg{err}
			}
		}
		return cmdFlashMsg{"Playing " + name}
	})
}
