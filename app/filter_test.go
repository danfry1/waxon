package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// findMoreTracks runs cmd (recursing into batches) and returns the first
// moreTracksLoadedMsg it produces, if any.
func findMoreTracks(cmd tea.Cmd) (moreTracksLoadedMsg, bool) {
	if cmd == nil {
		return moreTracksLoadedMsg{}, false
	}
	switch msg := cmd().(type) {
	case moreTracksLoadedMsg:
		return msg, true
	case tea.BatchMsg:
		for _, c := range msg {
			if m, ok := findMoreTracks(c); ok {
				return m, true
			}
		}
	}
	return moreTracksLoadedMsg{}, false
}

func TestFilterLoadsRemainingPages(t *testing.T) {
	pages := map[int][]source.Track{
		2: {{ID: "t3", Name: "Three"}, {ID: "t4", Name: "Four"}},
		4: {{ID: "t5", Name: "Five", Album: "Zebra Crossing"}, {ID: "t6", Name: "Six"}},
	}
	var fetched []int
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, _ string, offset, _ int) ([]source.Track, int, error) {
			fetched = append(fetched, offset)
			return pages[offset], 6, nil
		},
	}
	m := newTestModel(stub)
	m.focusPane = PaneTrackList
	m.tracklist.SetTracks([]source.Track{{ID: "t1", Name: "One"}, {ID: "t2", Name: "Two"}}, "PL", "spotify:playlist:pl1")
	m.pagination = &paginationState{playlistID: "pl1", total: 6, loaded: 2}
	m.mode = ModeFilter

	// Typing a query with the cursor at the top still fetches the next page:
	// the match lives on the last one.
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zebra")})
	model := result.(Model)
	for range 2 {
		msg, ok := findMoreTracks(cmd)
		if !ok {
			t.Fatalf("expected a page fetch while filtering; fetched so far %v", fetched)
		}
		result, cmd = model.Update(msg)
		model = result.(Model)
	}

	if model.pagination != nil {
		t.Errorf("pagination should be cleared once every page is loaded")
	}
	if _, ok := findMoreTracks(cmd); ok {
		t.Errorf("no further fetch expected after the last page")
	}
	got := model.tracklist.displayTracks()
	if len(got) != 1 || got[0].ID != "t5" {
		t.Errorf("filter should match t5 by album once loaded; got %+v", got)
	}
}

func TestNoFilterKeepsLazyLoading(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := make([]source.Track, 100)
	for i := range tracks {
		tracks[i] = source.Track{ID: string(rune('a' + i%26)), Name: "x"}
	}
	m.tracklist.SetTracks(tracks, "PL", "spotify:playlist:pl1")
	m.pagination = &paginationState{playlistID: "pl1", total: 500, loaded: 100}

	if cmd := m.maybeLoadMore(); cmd != nil {
		t.Error("cursor at the top without a filter should not fetch more pages")
	}
}

func TestFilterKeysTrackRemoval(t *testing.T) {
	tl := NewTrackList(80, 30)
	tl.SetTracks([]source.Track{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Bravo"},
		{ID: "c", Name: "Charlie"},
	}, "PL", "")
	tl.RemoveAt(0)
	tl.SetFilter("charlie")
	got := tl.displayTracks()
	if len(got) != 1 || got[0].ID != "c" {
		t.Errorf("filter after removal should still line up with tracks; got %+v", got)
	}
}

func TestEmptyPageEndsPagination(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "t1"}, {ID: "t2"}}, "PL", "spotify:playlist:pl1")
	m.tracklist.SetFilter("t")
	m.pagination = &paginationState{playlistID: "pl1", total: 10, loaded: 2, loadingMore: true}

	// The playlist shrank after the first page: Spotify returns nothing at
	// offset 2. That must end loading, not re-request the same offset.
	result, cmd := m.Update(moreTracksLoadedMsg{playlistID: "pl1", offset: 2, total: 10})
	model := result.(Model)
	if model.pagination != nil {
		t.Errorf("pagination should end on an empty page; got %+v", model.pagination)
	}
	if _, ok := findMoreTracks(cmd); ok {
		t.Error("an empty page must not trigger another fetch")
	}
}

func TestStalePageOffsetIsDropped(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "t1"}, {ID: "t2"}}, "PL", "spotify:playlist:pl1")
	m.pagination = &paginationState{playlistID: "pl1", total: 6, loaded: 4}

	result, _ := m.Update(moreTracksLoadedMsg{playlistID: "pl1", offset: 2, tracks: []source.Track{{ID: "t3"}}, total: 6})
	model := result.(Model)
	if n := len(model.tracklist.tracks); n != 2 {
		t.Errorf("page for a stale offset should be dropped; have %d tracks", n)
	}
	if model.pagination.loaded != 4 {
		t.Errorf("loaded = %d, want 4", model.pagination.loaded)
	}
}

func TestSetLoadingClearsFilter(t *testing.T) {
	tl := NewTrackList(80, 30)
	tl.SetTracks([]source.Track{{ID: "a", Name: "Alpha"}}, "PL", "")
	tl.SetFilter("alp")
	tl.SetLoading("Next")
	if tl.Filtering() {
		t.Error("opening a new view should drop the previous filter")
	}
}
