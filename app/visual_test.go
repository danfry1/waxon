package app

import (
	"context"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func visualModel(stub *StubSource) Model {
	m := newTestModel(stub)
	m.focusPane = PaneTrackList
	m.playlists = []source.Playlist{{ID: "pl1", Name: "Mine", URI: "spotify:playlist:pl1", Editable: true}, {ID: "pl2", Name: "Other", Editable: true}}
	m.tracklist.SetTracks([]source.Track{
		{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}, {ID: "d", Name: "D"},
	}, "Mine", "spotify:playlist:pl1")
	return m
}

func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyMsg
		if k == "esc" {
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		} else {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		var res tea.Model
		res, cmd = m.Update(msg)
		m = res.(Model)
	}
	return m, cmd
}

func TestVisualQueuesRangeInOrder(t *testing.T) {
	var queued []string
	stub := &StubSource{AddToQueueFn: func(_ context.Context, id string) error { queued = append(queued, id); return nil }}
	m := visualModel(stub)
	m.tracklist.MoveDown(1) // on b

	m, _ = press(t, m, "v", "j", "j")
	if m.mode != ModeVisual {
		t.Fatalf("mode = %v, want VISUAL", m.mode)
	}
	if lo, hi := m.tracklist.VisualRange(); lo != 1 || hi != 3 {
		t.Fatalf("range = %d..%d, want 1..3", lo, hi)
	}
	m, cmd := press(t, m, "a")
	runCmdTree(cmd)
	if m.mode != ModeNormal || m.tracklist.Visual() {
		t.Error("an action should leave visual mode")
	}
	if !slices.Equal(queued, []string{"b", "c", "d"}) {
		t.Errorf("queued %v, want [b c d]", queued)
	}
}

func TestVisualSelectionUpwards(t *testing.T) {
	m := visualModel(&StubSource{})
	m.tracklist.GotoBottom()
	m, _ = press(t, m, "v", "k")
	tracks, _ := m.tracklist.VisualSelection()
	if len(tracks) != 2 || tracks[0].ID != "c" || tracks[1].ID != "d" {
		t.Errorf("selection = %+v, want c, d in list order", tracks)
	}
}

func TestVisualRemoveGoesLastFirst(t *testing.T) {
	var positions []int
	stub := &StubSource{RemoveFromPlaylistFn: func(_ context.Context, _, _ string, pos int) error {
		positions = append(positions, pos)
		return nil
	}}
	m := visualModel(stub)
	m, _ = press(t, m, "v", "j", "j")
	m, cmd := press(t, m, "d")
	msg, ok := cmd().(playlistChangedMsg)
	if !ok {
		t.Fatalf("expected playlistChangedMsg, got %T", msg)
	}
	if !slices.Equal(positions, []int{2, 1, 0}) {
		t.Errorf("remove order %v, want [2 1 0]", positions)
	}
	res, _ := m.Update(msg)
	tl := res.(Model).tracklist
	left := tl.displayTracks()
	if len(left) != 1 || left[0].ID != "d" {
		t.Errorf("list after removal = %+v, want just d", left)
	}
}

func TestVisualRemoveUsesPlaylistPositionsUnderFilter(t *testing.T) {
	var positions []int
	stub := &StubSource{RemoveFromPlaylistFn: func(_ context.Context, _, _ string, pos int) error {
		positions = append(positions, pos)
		return nil
	}}
	m := visualModel(stub)
	m.tracklist.SetTracks([]source.Track{
		{ID: "a", Name: "Keep"}, {ID: "b", Name: "Drop"}, {ID: "c", Name: "Keep"}, {ID: "d", Name: "Drop"},
	}, "Mine", "spotify:playlist:pl1")
	m.tracklist.SetFilter("drop")
	_, cmd := press(t, m, "v", "j", "d")
	runCmdTree(cmd)
	if !slices.Equal(positions, []int{3, 1}) {
		t.Errorf("positions %v, want [3 1] (rows in the full playlist)", positions)
	}
}

func TestVisualRemoveRefusedOnOthersPlaylist(t *testing.T) {
	called := false
	stub := &StubSource{RemoveFromPlaylistFn: func(context.Context, string, string, int) error { called = true; return nil }}
	m := visualModel(stub)
	m.playlists[0].Editable = false
	m, cmd := press(t, m, "v", "j", "d")
	runCmdTree(cmd)
	if called {
		t.Error("remove must not run on a playlist the user can't edit")
	}
	if m.mode != ModeVisual {
		t.Error("a refused action should keep the selection")
	}
}

func TestVisualLikeAndPlaylistPicker(t *testing.T) {
	var saved, added []string
	stub := &StubSource{
		SaveTrackFn:     func(_ context.Context, id string) error { saved = append(saved, id); return nil },
		AddToPlaylistFn: func(_ context.Context, _, id string) error { added = append(added, id); return nil },
	}
	m := visualModel(stub)
	m, cmd := press(t, m, "v", "j", "f")
	if msg, ok := cmd().(likedBatchMsg); ok {
		res, _ := m.Update(msg)
		m = res.(Model)
	}
	if !slices.Equal(saved, []string{"a", "b"}) || !m.likedCache["a"] {
		t.Errorf("saved %v / cache %v", saved, m.likedCache)
	}

	m, _ = press(t, m, "v", "j", "o")
	if m.mode != ModePlaylistPick || !strings.Contains(m.playlistPick.View(), "2 tracks") {
		t.Fatalf("expected a batch playlist picker, mode %v", m.mode)
	}
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = res
	runCmdTree(cmd)
	if !slices.Equal(added, []string{"b", "c"}) { // cursor stayed on b after the first batch
		t.Errorf("added %v, want [b c]", added)
	}
}

func TestVisualEscCancels(t *testing.T) {
	m := visualModel(&StubSource{})
	m, _ = press(t, m, "v", "j", "esc")
	if m.mode != ModeNormal || m.tracklist.Visual() {
		t.Error("Esc should cancel the selection")
	}
}

func TestVisualSingleRowAddToPlaylist(t *testing.T) {
	var added []string
	stub := &StubSource{AddToPlaylistFn: func(_ context.Context, _, id string) error { added = append(added, id); return nil }}
	m := visualModel(stub)
	m, _ = press(t, m, "v", "o")
	if !strings.Contains(m.playlistPick.View(), "1 track") {
		t.Error("picker subtitle should read 1 track")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	runCmdTree(cmd)
	if !slices.Equal(added, []string{"a"}) {
		t.Errorf("added %v, want [a]", added)
	}
}

func TestVisualEndsWhenListReloads(t *testing.T) {
	m := visualModel(&StubSource{})
	m, _ = press(t, m, "v", "j")
	res, _ := m.Update(tracksLoadedMsg{tracks: []source.Track{{ID: "x"}}, title: "Mine", contextURI: "spotify:playlist:pl1"})
	m = res.(Model)
	if m.mode != ModeNormal || m.tracklist.Visual() {
		t.Error("a reloaded list should end visual mode")
	}
}

func TestVisualClearedWhenAnotherModeTakesOver(t *testing.T) {
	m := visualModel(&StubSource{})
	m, _ = press(t, m, "v", "j")
	m.mode = ModeDevices // e.g. a no-device picker opened by an async reply
	res, _ := m.Update(noopMsg{})
	if tl := res.(Model).tracklist; tl.Visual() {
		t.Error("selection should not outlive visual mode")
	}
}

func TestVisualSingleRowLikeSaves(t *testing.T) {
	var saved []string
	stub := &StubSource{SaveTrackFn: func(_ context.Context, id string) error { saved = append(saved, id); return nil }}
	m := visualModel(stub)
	m.likedCache["a"] = true // already liked: a batch like must not unlike it
	_, cmd := press(t, m, "v", "f")
	runCmdTree(cmd)
	if !slices.Equal(saved, []string{"a"}) {
		t.Errorf("saved %v, want [a]", saved)
	}
}
