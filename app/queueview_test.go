package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/danfry1/waxon/source"
)

// queueModel plays track "now" from Liked Songs, whose cached tracks are
// now, c1, c2, c3.
func queueModel(stub *StubSource) Model {
	m := newTestModel(stub)
	m.width, m.height = 140, 40
	m.layoutResize()
	m.playlists = []source.Playlist{{ID: likedPlaylistID, URI: "spotify:collection:tracks", Name: "♥ Liked Songs"}}
	m.trackCache[likedPlaylistID] = cachedPlaylist{tracks: []source.Track{
		{ID: "now"}, {ID: "c1"}, {ID: "c2"}, {ID: "c3"},
	}}
	m.track = &source.Track{ID: "now", Name: "Playing", Playing: true}
	m.playbackContextURI = "spotify:user:me:collection"
	return m
}

// The queue tells the tracks you queued from what plays next from the
// playlist, as the Spotify apps do.
func TestQueueSections(t *testing.T) {
	m := queueModel(&StubSource{})
	rows, info := m.queueRows([]source.Track{
		{ID: "q1", Name: "Mine", Duration: time.Minute},
		{ID: "c1", Name: "Next in list", Duration: time.Minute},
		{ID: "c2", Name: "After", Duration: time.Minute},
	})
	var headings []string
	for _, r := range rows {
		if r.IsSeparator && r.Name != "" {
			headings = append(headings, r.Name+" "+r.Artist)
		}
	}
	want := []string{"Now playing ", "Next in queue 1", "Next from Liked Songs 2"}
	if strings.Join(headings, ", ") != strings.Join(want, ", ") {
		t.Errorf("headings = %v, want %v", headings, want)
	}
	if info != "1 queued · 2 up next · 3m" {
		t.Errorf("info = %q", info)
	}
}

// A queued copy of the playlist's next track stays in "Next in queue":
// the playlist part starts where the rest runs in the playlist's order.
func TestQueueSectionsQueuedCopyOfNext(t *testing.T) {
	m := queueModel(&StubSource{})
	queued, rest, ok := m.queueParts([]source.Track{
		{ID: "q1"}, {ID: "c1"}, {ID: "c2"}, {ID: "c1"}, {ID: "c1"}, {ID: "c2"}, {ID: "c3"},
	})
	if !ok || len(queued) != 4 || len(rest) != 3 || rest[0].ID != "c1" {
		t.Errorf("queued %d, from the playlist %d (ok %v); want 4 and 3", len(queued), len(rest), ok)
	}
}

// With shuffle on, or a context waxon hasn't loaded, they can't be told
// apart: one "Up next".
func TestQueueUnsplitWhenUnknown(t *testing.T) {
	m := queueModel(&StubSource{})
	m.shuffleOn = true
	rows, _ := m.queueRows([]source.Track{{ID: "q1"}, {ID: "c1"}})
	for _, r := range rows {
		if r.IsSeparator && (r.Name == "Next in queue" || strings.HasPrefix(r.Name, "Next from")) {
			t.Errorf("shuffled queue was split: %q", r.Name)
		}
	}
}

// Enter on a queued track skips forward to it, keeping what's after it,
// instead of playing it alone.
func TestEnterInQueueSkipsToTrack(t *testing.T) {
	nexts := 0
	played := false
	stub := &StubSource{
		NextFn:      func(context.Context) error { nexts++; return nil },
		PlayTrackFn: func(context.Context, string, string) error { played = true; return nil },
		QueueFn: func(context.Context) ([]source.Track, error) {
			return []source.Track{{ID: "q1", Name: "Mine"}, {ID: "c1"}, {ID: "c2", Name: "Third"}}, nil
		},
	}
	m := queueModel(stub)
	cmd := m.openQueue()
	res, _ := m.Update(cmd())
	m = res.(Model)
	if m.tracklist.title != "Queue" {
		t.Fatalf("title = %q", m.tracklist.title)
	}
	// Down to the third queued track, past the headings.
	for m.tracklist.SelectedTrack() == nil || m.tracklist.SelectedTrack().ID != "c2" {
		before := m.tracklist.Cursor()
		m.tracklist.MoveDown(1)
		if m.tracklist.Cursor() == before {
			t.Fatal("never reached c2")
		}
	}
	_, enter := m.handleEnter()
	if enter == nil {
		t.Fatal("Enter did nothing")
	}
	enter()
	if nexts != 3 || played {
		t.Errorf("Next ×%d, played alone %v; want three skips", nexts, played)
	}
}

// Enter on the track that's playing does nothing.
func TestEnterOnNowPlayingRow(t *testing.T) {
	nexts := 0
	m := queueModel(&StubSource{NextFn: func(context.Context) error { nexts++; return nil }})
	m.showQueue([]source.Track{{ID: "c1"}})
	m.tracklist.GotoTop()
	m.tracklist.MoveDown(0) // the Now playing row
	if _, cmd := m.handleEnter(); cmd != nil {
		cmd()
	}
	if nexts != 0 {
		t.Errorf("Enter on the playing track skipped %d times", nexts)
	}
}

// A queue reply only lands in the queue view, not over a playlist the
// user moved on to.
func TestStaleQueueReplyIgnored(t *testing.T) {
	m := queueModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "x", Name: "Elsewhere"}}, "Other", "spotify:playlist:o")
	res, _ := m.Update(queueLoadedMsg{tracks: []source.Track{{ID: "q"}}})
	if got := res.(Model).tracklist.title; got != "Other" {
		t.Errorf("title = %q, want the view left alone", got)
	}
}

func TestQueueViewRenders(t *testing.T) {
	m := queueModel(&StubSource{})
	m.showQueue([]source.Track{{ID: "q1", Name: "Mine", Artist: "Me"}, {ID: "c1", Name: "Theirs"}})
	v := ansi.Strip(m.tracklist.View(true))
	for _, want := range []string{"NOW PLAYING ", "NEXT IN QUEUE · 1", "NEXT FROM LIKED SONGS · 1", "Mine"} {
		if !strings.Contains(v, want) {
			t.Errorf("queue view lacks %q:\n%s", want, v)
		}
	}
}

// Queuing from the queue view says the track was added again, at the end:
// Spotify can't move tracks already queued.
func TestQueueFromQueueViewSaysCopy(t *testing.T) {
	m := queueModel(&StubSource{})
	m.tracklist.SetTracks(nil, "Queue", queueViewURI)
	res, _ := m.Update(queueDoneMsg{trackName: "Dance Yrself Clean", count: 1})
	if v := res.(Model).View(); !strings.Contains(v, "end of the queue") {
		t.Errorf("the toast should say it went to the end of the queue:\n%s", v)
	}
}

// Spotify's queue lags an add, so the queue view reads it again once it
// has settled, and only while the queue is showing.
func TestQueueRereadAfterAdd(t *testing.T) {
	stub := &StubSource{}
	m := queueModel(stub)
	if _, cmd := m.Update(queueRefreshMsg{}); cmd != nil {
		t.Error("queue not showing: nothing should be read")
	}
	m.tracklist.SetTracks(nil, "Queue", queueViewURI)
	_, cmd := m.Update(queueRefreshMsg{})
	if cmd == nil {
		t.Fatal("queue showing: it should be read again")
	}
	if _, ok := cmd().(queueLoadedMsg); !ok {
		t.Error("the re-read should load the queue")
	}
}

// Playing a queued copy of a playlist track leaves the playlist where it
// was: what follows is the playlist from there, not from the copy.
func TestQueueSectionsPlayingQueuedCopy(t *testing.T) {
	m := queueModel(&StubSource{})
	m.track = &source.Track{ID: "c2", Name: "Copy", Playing: true}
	queued, rest, ok := m.queueParts([]source.Track{
		{ID: "q1"}, {ID: "q1"}, {ID: "c1"}, {ID: "c2"}, {ID: "c3"},
	})
	if !ok || len(queued) != 2 || len(rest) != 3 || rest[0].ID != "c1" {
		t.Errorf("queued %d, from the playlist %d (ok %v); want 2 and 3", len(queued), len(rest), ok)
	}
}

// Only Now playing gets the ▶, not a queued copy of the same track.
func TestQueuePlayingMarkedOnce(t *testing.T) {
	m := queueModel(&StubSource{})
	m.tracklist.SetTracks(nil, "Queue", queueViewURI)
	m.showQueue([]source.Track{{ID: "q1", Name: "Mine"}, {ID: "now", Name: "Playing"}, {ID: "c1"}})
	if n := strings.Count(m.tracklist.View(true), "▶"); n != 1 {
		t.Errorf("▶ shown %d times, want once", n)
	}
}
