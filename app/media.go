package app

import (
	"image"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/media"
	"github.com/danfry1/waxon/source"
)

// System media controls (package media): the keyboard's media keys and the
// OS's Now Playing widgets. Presses arrive as MediaMsg and are handled like
// waxon's own keys; what's playing is published to them as it changes.

// MediaMsg is a press of a system media control.
type MediaMsg media.Event

func (m Model) handleMedia(msg MediaMsg) (Model, tea.Cmd) {
	switch msg.Command {
	case media.Toggle:
		if m.track != nil && m.track.Playing {
			return m, m.controlCmd(m.source.Pause)
		}
		return m, m.controlCmd(m.source.Play)
	case media.Play:
		return m, m.controlCmd(m.source.Play)
	case media.Pause:
		return m, m.controlCmd(m.source.Pause)
	case media.Next:
		return m, m.controlCmd(m.source.Next)
	case media.Previous:
		return m, m.controlCmd(m.source.Previous)
	case media.Seek:
		return m, m.seekTo(msg.Position)
	}
	return m, nil
}

// mediaPublished is what was last sent to the system's Now Playing.
type mediaPublished struct {
	hasTrack bool
	trackKey string
	playing  bool
	artURL   string
	duration time.Duration
	pos      time.Duration
	at       time.Time
}

// mediaDrift is how far the position may wander from where the system's
// display has it (it advances it by itself while playing) before waxon
// resends it — a seek, or a track that stalled.
const mediaDrift = 2 * time.Second

// publishMedia sends what's playing to the system's media controls when
// something they show has changed.
func (m *Model) publishMedia(now time.Time) {
	if m.opts.Media == nil {
		return
	}
	last := m.mediaLast
	if m.track == nil {
		if last.hasTrack {
			m.opts.Media.Update(media.State{Empty: true})
			m.mediaLast = mediaPublished{at: now}
		}
		return
	}
	t := m.track
	key := mediaTrackKey(t)
	expected := last.pos
	if last.playing {
		expected += now.Sub(last.at)
	}
	drift := t.Position - expected
	changed := !last.hasTrack || last.trackKey != key || last.playing != t.Playing ||
		last.duration != t.Duration || last.artURL != m.mediaArtURL(t) ||
		drift > mediaDrift || drift < -mediaDrift
	if !changed {
		return
	}
	var art image.Image
	if m.mediaArtURL(t) != "" {
		art = m.mediaArt
	}
	m.opts.Media.Update(media.State{
		ID:         key,
		Title:      t.Name,
		Artist:     t.Artist,
		Album:      t.Album,
		Duration:   t.Duration,
		Position:   t.Position,
		Playing:    t.Playing,
		ArtworkURL: m.mediaArtURL(t),
		Artwork:    art,
	})
	m.mediaLast = mediaPublished{
		hasTrack: true, trackKey: key, playing: t.Playing, artURL: m.mediaArtURL(t),
		duration: t.Duration, pos: t.Position, at: now,
	}
}

// mediaTrackKey identifies t for the system's display: local files have
// no Spotify id, so their URI (or failing that, name and artist) stands in.
func mediaTrackKey(t *source.Track) string {
	switch {
	case t.ID != "":
		return t.ID
	case t.URI != "":
		return t.URI
	}
	return t.Name + "\x00" + t.Artist
}

// mediaArtURL is t's artwork URL once its image has loaded ("" until then,
// so the cover is sent when it arrives).
func (m Model) mediaArtURL(t *source.Track) string {
	if m.mediaArt != nil && t.ArtworkURL != "" && m.mediaArtFor == t.ArtworkURL {
		return t.ArtworkURL
	}
	return ""
}
