package app

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Opening an album from search in Now Playing shows it, rather than
// returning to Now Playing with the album loading behind it.
func TestSearchAlbumFromNowPlayingShowsIt(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeNowPlaying
	res, _ := m.Update(runeKey("s"))
	m = res.(Model)
	m.search.results = &source.SearchResults{Albums: []source.SearchAlbum{{ID: "al", Name: "LP"}}}
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := res.(Model).mode; got != ModeNormal {
		t.Errorf("mode = %v, want the main view", got)
	}
}

// :albums from Now Playing goes to the albums, too.
func TestCollectionCommandFromNowPlaying(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeNowPlaying
	res, _ := m.Update(runeKey(":"))
	m = res.(Model)
	m.cmdInput = "albums"
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := res.(Model).mode; got != ModeNormal {
		t.Errorf("mode = %v, want the main view", got)
	}
}

// Where an overlay returns to is decided when it opens: one opened from
// the main view after a trip through Now Playing closes to the main view.
func TestOverlayReturnIsNotLeftOver(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeNowPlaying
	res, _ := m.Update(runeKey("s")) // from Now Playing...
	m = res.(Model)
	m.search, m.mode = nil, ModeNormal // ...then off to an album
	res, _ = m.Update(deviceChoicesMsg{devices: []source.Device{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}})
	m = res.(Model)
	if m.mode != ModeDevices {
		t.Fatalf("mode = %v, want the device picker", m.mode)
	}
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if got := res.(Model).mode; got != ModeNormal {
		t.Errorf("Esc went to %v, want the main view", got)
	}
}

// Shuffle Play on a playlist Spotify won't play says so, as Play does.
func TestShufflePlayUnplayablePlaylist(t *testing.T) {
	stub := &StubSource{
		PlayTrackFn: func(context.Context, string, string) error { return source.ErrNoActiveDevice },
		DevicesFn: func(context.Context) ([]source.Device, error) {
			return []source.Device{{ID: "d", Name: "Phone", IsActive: true}}, nil
		},
	}
	m := newTestModel(stub)
	if msg := m.shufflePlay("spotify:playlist:x")(); msg != (playlistUnplayableMsg{}) {
		t.Errorf("got %#v, want playlistUnplayableMsg", msg)
	}
}

// o on a queue entry opens the track's menu, not a playlist's.
func TestQueueEntryMenuIsATrackMenu(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.sidebar.SetQueueTracks([]source.Track{{ID: "q1", URI: "spotify:track:q1", Name: "Next up"}})
	m.sidebar.SetSection(SectionQueue)
	m.focusPane = PaneSidebar
	res, _ := m.Update(runeKey("o"))
	mm := res.(Model)
	if mm.actions == nil {
		t.Fatal("no menu")
	}
	for _, it := range mm.actions.items {
		if it.Type == ActionShufflePlaylist || it.Type == ActionPlayPlaylist {
			t.Errorf("a queued track's menu offers %q", it.Label)
		}
	}
}

// ga on an album row opens that album, not the playing track's.
func TestGoToAlbumOnAnAlbumRow(t *testing.T) {
	var opened string
	stub := &StubSource{GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
		opened = id
		return &source.AlbumPage{}, nil
	}}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "p", AlbumID: "playingAlbum"}
	m.tracklist.SetTracks([]source.Track{{Name: "Saved", AlbumID: "savedAlbum", IsAlbumRow: true}}, "Albums", "")
	m.focusPane = PaneTrackList
	res, _ := m.Update(runeKey("g"))
	_, cmd := res.(Model).Update(runeKey("a"))
	if cmd != nil {
		cmd()
	}
	if opened != "savedAlbum" {
		t.Errorf("opened %q, want the row's album", opened)
	}
}

// Counts work in Now Playing: 6] seeks half a minute.
func TestCountedSeekInNowPlaying(t *testing.T) {
	var to time.Duration
	stub := &StubSource{SeekFn: func(_ context.Context, d time.Duration) error { to = d; return nil }}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	m.track = &source.Track{ID: "t", Position: time.Minute, Duration: 5 * time.Minute}
	res, _ := m.Update(runeKey("6"))
	_, cmd := res.(Model).Update(runeKey("]"))
	runCmdTree(cmd)
	if to < time.Minute+29*time.Second || to > time.Minute+31*time.Second {
		t.Errorf("6] seeked to %v, want about 1:30", to)
	}
}

func TestCommandParsingEdges(t *testing.T) {
	if got := acceptSuggestion("pl"); got != "pl " {
		t.Errorf("complete(pl) = %q: an alias shouldn't become another command", got)
	}
	for _, bad := range []string{"seek 1:75", "seek +-5", "vol +-5"} {
		if _, err := ParseCommand(bad); err == nil {
			t.Errorf("%q should be an error", bad)
		}
	}
}
