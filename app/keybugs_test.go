package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Ctrl-C quits from the main view and from inside overlays.
func TestCtrlCQuits(t *testing.T) {
	for _, mode := range []Mode{ModeNormal, ModeHelp, ModeSearch, ModeCommand} {
		m := newTestModel(&StubSource{})
		m.mode = mode
		if mode == ModeSearch {
			s := NewSearch(80, 24)
			m.search = &s
		}
		res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if !res.(Model).quitting || cmd == nil {
			t.Errorf("Ctrl-C didn't quit in mode %v", mode)
		}
	}
}

func sidebarWith(n int) Model {
	m := newTestModel(&StubSource{})
	m.width, m.height = 120, 40
	m.layoutResize()
	pls := make([]source.Playlist, n)
	for i := range pls {
		pls[i] = source.Playlist{ID: string(rune('a' + i)), Name: "P", URI: "spotify:playlist:" + string(rune('a'+i))}
	}
	m.sidebar.SetPlaylists(pls)
	m.focusPane = PaneSidebar
	return m
}

// C-d / C-u move half a page in the sidebar too.
func TestHalfPageInSidebar(t *testing.T) {
	m := sidebarWith(20)
	start := m.sidebar.list.Index()
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = res.(Model)
	if m.sidebar.list.Index() <= start+1 {
		t.Fatalf("C-d moved the sidebar from %d to %d", start, m.sidebar.list.Index())
	}
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if got := res.(Model).sidebar.list.Index(); got != start {
		t.Errorf("C-u came back to %d, want %d", got, start)
	}
}

// Rebound movement keys move the sidebar, not only the track list.
func TestReboundKeysMoveSidebar(t *testing.T) {
	km, _ := KeyMapFromOverrides(map[string]string{"down": "ctrl+n"})
	m := sidebarWith(5).WithOptions(Options{Keys: &km})
	start := m.sidebar.list.Index()
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if got := res.(Model).sidebar.list.Index(); got != start+1 {
		t.Errorf("rebound down moved the sidebar to %d, want %d", got, start+1)
	}
}

// Enter on a playlist moves into the track list, so the next Enter plays.
func TestEnterOnPlaylistFocusesTrackList(t *testing.T) {
	m := sidebarWith(3)
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := res.(Model).focusPane; got != PaneTrackList {
		t.Errorf("focus = %v after opening a playlist, want the track list", got)
	}
}

// a in the sidebar doesn't queue the track list's (unseen) selection.
func TestQueueFromSidebarExplains(t *testing.T) {
	queued := 0
	stub := &StubSource{AddToQueueFn: func(context.Context, string) error { queued++; return nil }}
	m := newTestModel(stub)
	m.tracklist.SetTracks([]source.Track{{ID: "t", Name: "Hidden"}}, "L", "")
	m.focusPane = PaneSidebar
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if cmd != nil {
		cmd()
	}
	if queued != 0 || !strings.Contains(res.(Model).toast.message, "Pick a track") {
		t.Errorf("queued %d, toast %q", queued, res.(Model).toast.message)
	}
}

// Search results move with C-n/C-p, since letters type into the query.
func TestSearchMovesWithCtrlNP(t *testing.T) {
	s := NewSearch(80, 24)
	s.results = &source.SearchResults{Tracks: []source.Track{{ID: "1"}, {ID: "2"}}}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if s.cursor != 1 {
		t.Errorf("C-n: cursor %d, want 1", s.cursor)
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if s.cursor != 0 {
		t.Errorf("C-p: cursor %d, want 0", s.cursor)
	}
}

// A key problem in config.json is shown in the app at startup.
func TestKeysProblemShown(t *testing.T) {
	m := newTestModel(&StubSource{}).WithOptions(Options{KeysProblem: `unknown action "nxt"`})
	res, _ := m.Update(keysProblemMsg{m.opts.KeysProblem})
	if mm := res.(Model); !mm.toast.Visible() || !strings.Contains(mm.toast.detail, "nxt") {
		t.Errorf("toast %q / %q", mm.toast.message, mm.toast.detail)
	}
}
