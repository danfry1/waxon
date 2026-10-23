package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/danfry1/waxon/source"
)

func TestSkeletonWhileLoading(t *testing.T) {
	tl := NewTrackList(90, 30)
	tl.SetLoading("Mix")
	if v := ansi.Strip(tl.View(true)); !strings.Contains(v, "▁▁") || strings.Contains(v, "Loading...") {
		t.Error("first load should draw placeholder rows")
	}
}

func TestLoadingMoreShownInHeader(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "a", Name: "A"}}, "Mix", "spotify:playlist:p")
	m.tracklist.SetHeaderInfo(FormatPartialTrackListInfo(1, 300))
	m.pagination = &paginationState{playlistID: "p", total: 300, loaded: 1, loadingMore: true}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "loading more") {
		t.Error("header should say a page is loading")
	}
	if !m.needsProgressTick() {
		t.Error("the loading-more spinner needs the tick")
	}
	m.pagination.loadingMore = false
	if v := ansi.Strip(m.View()); strings.Contains(v, "loading more") {
		t.Error("no loading note once the page has arrived")
	}
}

func TestNPLyricsHint(t *testing.T) {
	synced := &source.Lyrics{Synced: true, Lines: []source.LyricLine{{Text: "x"}}}
	plain := &source.Lyrics{Lines: []source.LyricLine{{Text: "x"}}}
	for _, tc := range []struct {
		lyr  npLyrics
		want string
	}{
		{npLyrics{loading: true}, "l lyrics…"},
		{npLyrics{lyrics: synced}, "l synced lyrics"},
		{npLyrics{lyrics: plain}, "l lyrics"},
		{npLyrics{}, "no lyrics"},
		{npLyrics{err: true}, "l lyrics"},
	} {
		if got := npLyricsHint(tc.lyr); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.lyr, got, tc.want)
		}
	}
}

func TestOpeningNowPlayingFetchesLyrics(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.track = &source.Track{ID: "t1", Name: "Song"}
	m, _ = press(t, m, "N")
	if m.lyricsTrackID != "t1" || !m.lyricsLoading {
		t.Error("opening Now Playing should look up lyrics for the hint")
	}
}
