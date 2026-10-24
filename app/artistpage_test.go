package app

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/danfry1/waxon/source"
)

// An artist page starts with Play and Shuffle (Spotify won't list most
// apps an artist's top tracks), and its sections count their releases.
func TestArtistPageRows(t *testing.T) {
	rows := buildArtistTrackList(&source.ArtistPage{ID: "ar", Name: "Band", Albums: []source.ArtistAlbum{
		{ID: "a1", Name: "LP", Type: "Album", ImageURL: "img"},
		{ID: "s1", Name: "Single", Type: "Single"},
	}})
	if rows[0].ID != playArtistRowID || rows[1].ID != shuffleArtistRowID || rows[0].URI != "spotify:artist:ar" {
		t.Fatalf("first rows = %+v, %+v; want Play and Shuffle", rows[0], rows[1])
	}
	var album source.Track
	for _, r := range rows {
		if r.AlbumID == "a1" {
			album = r
		}
	}
	if album.ArtworkURL != "img" {
		t.Errorf("album row has no cover URL: %+v", album)
	}
}

// Play and Shuffle play the artist; Shuffle turns shuffle on first.
func TestPlayAndShuffleArtist(t *testing.T) {
	var played string
	shuffled := false
	stub := &StubSource{
		PlayTrackFn:  func(_ context.Context, c, _ string) error { played = c; return nil },
		SetShuffleFn: func(_ context.Context, on bool) error { shuffled = on; return nil },
	}
	for _, id := range []string{playArtistRowID, shuffleArtistRowID} {
		played, shuffled = "", false
		m := newTestModel(stub)
		cmd, ok := m.openRow(source.Track{ID: id, URI: "spotify:artist:ar", IsAlbumRow: true})
		if !ok || cmd == nil {
			t.Fatalf("%s: not opened", id)
		}
		cmd()
		if played != "spotify:artist:ar" || shuffled != (id == shuffleArtistRowID) {
			t.Errorf("%s: played %q, shuffled %v", id, played, shuffled)
		}
	}
}

// A release list is headed for releases, and each row shows its cover in
// the row itself.
func TestReleaseHeadersAndCovers(t *testing.T) {
	m := newTestModel(&StubSource{SavedAlbumsFn: func(context.Context) ([]source.SavedAlbum, error) {
		return []source.SavedAlbum{{ID: "a", Name: "First", Artist: "X", ImageURL: "u1"}, {ID: "b", Name: "Second", Artist: "Y", ImageURL: "u2"}}, nil
	}})
	m.width, m.height = 140, 40
	m.layoutResize()
	res, _ := m.Update(m.fetchCollection(collectionAlbumsID, "Saved Albums")())
	m = res.(Model)
	res, _ = m.Update(rowIconLoadedMsg{url: "u2", icon: "\x1b[31m▀▀▀\x1b[0m"})
	m = res.(Model)
	view := m.tracklist.View(true)
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "Album") || !strings.Contains(plain, "Year") || strings.Contains(plain, "Duration") {
		t.Errorf("headers aren't for albums:\n%s", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Second") && !strings.Contains(line, "▀▀▀") {
			t.Errorf("Second's row has no cover: %q", line)
		}
		if strings.Contains(line, "First") && strings.Contains(line, "▀▀▀") {
			t.Errorf("First's row has Second's cover: %q", line)
		}
	}
}

// Moving skips section lines and blank rows: there's nothing to do on them.
func TestCursorSkipsSeparators(t *testing.T) {
	var tl TrackList
	tl = NewTrackList(100, 30)
	tl.SetTracks([]source.Track{
		{ID: "t1", Name: "One"},
		{IsSeparator: true},
		{Name: "Albums", IsSeparator: true},
		{Name: "LP", AlbumID: "a", IsAlbumRow: true},
	}, "Band", "")
	tl.MoveDown(1)
	if tl.cursor != 3 {
		t.Errorf("down from the track: cursor %d, want the album row (3)", tl.cursor)
	}
	tl.MoveUp(1)
	if tl.cursor != 0 {
		t.Errorf("up from the album: cursor %d, want the track (0)", tl.cursor)
	}
}
