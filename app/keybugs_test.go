package app

import (
	"context"
	"strings"
	"testing"
	"time"

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

// In the menu, a row's number or the main view's letter runs it at once.
func TestMenuShortcuts(t *testing.T) {
	queued := ""
	stub := &StubSource{AddToQueueFn: func(_ context.Context, id string) error { queued = id; return nil }}
	for _, k := range []string{"a", "4"} { // a: queue; 4: the fourth row, Add to Queue
		queued = ""
		m := newTestModel(stub)
		m.tracklist.SetTracks([]source.Track{{ID: "t1", URI: "spotify:track:t1", Name: "Song"}}, "L", "")
		m.focusPane = PaneTrackList
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		m = res.(Model)
		if m.mode != ModeActions {
			t.Fatalf("o didn't open the menu")
		}
		res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		if cmd != nil {
			cmd()
		}
		if queued != "t1" || res.(Model).mode == ModeActions {
			t.Errorf("o %s: queued %q, mode %v", k, queued, res.(Model).mode)
		}
	}
}

// F likes the playing track, from the track list or Now Playing.
func TestShiftFLikesPlaying(t *testing.T) {
	for _, mode := range []Mode{ModeNormal, ModeNowPlaying} {
		saved := ""
		stub := &StubSource{SaveTrackFn: func(_ context.Context, id string) error { saved = id; return nil }}
		m := newTestModel(stub)
		m.mode = mode
		m.track = &source.Track{ID: "playing", Name: "Now"}
		m.tracklist.SetTracks([]source.Track{{ID: "sel"}}, "L", "")
		m.focusPane = PaneTrackList
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
		if cmd != nil {
			cmd()
		}
		if saved != "playing" {
			t.Errorf("mode %v: F liked %q, want the playing track", mode, saved)
		}
	}
}

// Search, devices, : and ? open from Now Playing and close back to it.
func TestOverlaysFromNowPlayingReturnToIt(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeNowPlaying
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = res.(Model)
	if m.mode != ModeSearch {
		t.Fatalf("s from Now Playing: mode %v", m.mode)
	}
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if got := res.(Model).mode; got != ModeNowPlaying {
		t.Errorf("Esc from search went to %v, want Now Playing", got)
	}
}

// m mutes, and m again brings the volume back.
func TestMute(t *testing.T) {
	var set []int
	stub := &StubSource{SetVolumeFn: func(_ context.Context, v int) error { set = append(set, v); return nil }}
	m := newTestModel(stub)
	m.volume = 60
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	cmd()
	res, cmd = res.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	cmd()
	if len(set) != 2 || set[0] != 0 || set[1] != 60 {
		t.Errorf("volumes set = %v, want [0 60]", set)
	}
	_ = res
}

// A count multiplies a seek: 6] is +30s.
func TestCountedSeek(t *testing.T) {
	var to time.Duration
	stub := &StubSource{SeekFn: func(_ context.Context, d time.Duration) error { to = d; return nil }}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t", Position: time.Minute, Duration: 5 * time.Minute}
	m.focusPane = PaneTrackList
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	_, cmd := res.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	runCmdTree(cmd)
	if to < time.Minute+29*time.Second || to > time.Minute+31*time.Second {
		t.Errorf("6] seeked to %v, want about 1:30", to)
	}
}

// Esc clears a filter left applied.
func TestEscClearsFilter(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "1", Name: "a"}, {ID: "2", Name: "b"}}, "L", "")
	m.focusPane = PaneTrackList
	m.tracklist.SetFilter("a")
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	mm := res.(Model)
	if got := mm.tracklist.FilterText(); got != "" {
		t.Errorf("filter after Esc = %q", got)
	}
}

// Tab in search moves to the results, where a queues the selected track
// instead of typing into the query.
func TestSearchTabActsOnResults(t *testing.T) {
	queued := ""
	stub := &StubSource{AddToQueueFn: func(_ context.Context, id string) error { queued = id; return nil }}
	m := newTestModel(stub)
	s := NewSearch(80, 24)
	s.results = &source.SearchResults{Tracks: []source.Track{{ID: "r1", Name: "Hit"}}}
	m.search, m.mode = &s, ModeSearch
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	res, cmd := res.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if cmd != nil {
		runCmdTree(cmd)
	}
	mm := res.(Model)
	if queued != "r1" || mm.search.input.Value() != "" {
		t.Errorf("queued %q, query %q; want the result queued and nothing typed", queued, mm.search.input.Value())
	}
}
