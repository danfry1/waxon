package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func searchWith(t *testing.T, stub *StubSource, track source.Track) tea.Cmd {
	t.Helper()
	m := newTestModel(stub)
	s := NewSearch(m.width, m.height)
	s.input.SetValue("q")
	s, _ = s.Update(searchResultsMsg{query: "q", results: &source.SearchResults{Tracks: []source.Track{track}}})
	m.search = &s
	m.mode = ModeSearch
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return cmd
}

func TestSearchPlaysTrackInItsAlbum(t *testing.T) {
	var gotCtx, gotTrack string
	stub := &StubSource{PlayTrackFn: func(_ context.Context, c, tr string) error { gotCtx, gotTrack = c, tr; return nil }}
	runCmdTree(searchWith(t, stub, source.Track{ID: "t1", URI: "spotify:track:t1", AlbumID: "al1"}))
	if gotCtx != "spotify:album:al1" || gotTrack != "spotify:track:t1" {
		t.Errorf("played %q in %q, want the track inside spotify:album:al1", gotTrack, gotCtx)
	}
}

func TestSearchWithoutAlbumPlaysDirect(t *testing.T) {
	direct := ""
	stub := &StubSource{PlayTrackDirectFn: func(_ context.Context, tr string) error { direct = tr; return nil }}
	runCmdTree(searchWith(t, stub, source.Track{ID: "t1", URI: "spotify:track:t1"}))
	if direct != "spotify:track:t1" {
		t.Errorf("direct play = %q", direct)
	}
}
