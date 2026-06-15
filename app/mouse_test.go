package app

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// makeTracks builds n simple, distinguishable tracks.
func makeTracks(n int) []source.Track {
	tracks := make([]source.Track, n)
	for i := range tracks {
		tracks[i] = source.Track{
			ID:     fmt.Sprintf("t%03d", i),
			Name:   fmt.Sprintf("Song %03d", i),
			Artist: "Artist",
			URI:    fmt.Sprintf("spotify:track:%03d", i),
		}
	}
	return tracks
}

// newScrollableTrackList returns a tracklist short enough that n tracks
// overflow its viewport, forcing a scroll offset.
func newScrollableTrackList(t *testing.T, n int) TrackList {
	t.Helper()
	tl := NewTrackList(80, 16)
	tl.SetTracks(makeTracks(n), "Test", "ctx")
	if tl.capacity() >= n {
		t.Fatalf("test setup: capacity %d should be < %d to exercise scrolling", tl.capacity(), n)
	}
	return tl
}

// clickRow simulates a click on the k-th visible row (0 = first visible).
func clickRow(tl *TrackList, k int) bool {
	return tl.SetCursorFromClick(tl.headerRows() + k)
}

// ---- #1: click maps to the correct row regardless of scroll ----

func TestClickUnscrolledSelectsRow(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	c := tl.capacity()
	for k := 0; k < c; k++ {
		if !clickRow(&tl, k) {
			t.Fatalf("click on visible row %d returned false", k)
		}
		got := tl.SelectedTrack()
		want := fmt.Sprintf("t%03d", k)
		if got == nil || got.ID != want {
			t.Fatalf("click row %d selected %v, want %s", k, got, want)
		}
		if tl.Cursor() != k {
			t.Fatalf("click row %d cursor = %d, want %d", k, tl.Cursor(), k)
		}
	}
}

// The core regression test for the original bug: once the list is scrolled,
// the visible row index no longer equals the data index. The click must add
// the scroll offset.
func TestClickScrolledSelectsCorrectRow(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	tl.GotoBottom() // force a non-zero offset
	if tl.offset == 0 {
		t.Fatal("expected a non-zero scroll offset after GotoBottom")
	}
	offset := tl.offset
	visible := len(tl.table.Rows())

	for k := 0; k < visible; k++ {
		clickRow(&tl, k)
		want := fmt.Sprintf("t%03d", offset+k)
		got := tl.SelectedTrack()
		if got == nil || got.ID != want {
			t.Fatalf("scrolled click row %d selected %v, want %s (offset=%d)", k, got, want, offset)
		}
		if tl.Cursor() != offset+k {
			t.Fatalf("scrolled click row %d cursor = %d, want %d", k, tl.Cursor(), offset+k)
		}
	}
}

// Scroll to a mid-list position and verify the mapping there too.
func TestClickMidScrollSelectsCorrectRow(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	tl.JumpToTrack("t050")
	offset := tl.offset
	if offset == 0 {
		t.Fatal("expected non-zero offset at mid-list")
	}
	clickRow(&tl, 2)
	want := fmt.Sprintf("t%03d", offset+2)
	if got := tl.SelectedTrack(); got == nil || got.ID != want {
		t.Fatalf("mid-scroll click selected %v, want %s", got, want)
	}
}

// The window handed to the table must always be exactly allRows[offset:end],
// which is what guarantees offset is the true first-visible row.
func TestViewportWindowMatchesOffset(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	for _, cursor := range []int{0, 5, 20, 50, 75, 99} {
		tl.cursor = cursor
		tl.syncViewport()
		rows := tl.table.Rows()
		end := tl.offset + len(rows)
		if end > len(tl.allRows) {
			t.Fatalf("cursor %d: window end %d exceeds %d rows", cursor, end, len(tl.allRows))
		}
		for i, r := range rows {
			if r[0] != tl.allRows[tl.offset+i][0] || r[2] != tl.allRows[tl.offset+i][2] {
				t.Fatalf("cursor %d: window row %d = %v, want %v", cursor, i, r, tl.allRows[tl.offset+i])
			}
		}
		// The cursor must remain within the visible window.
		rel := tl.cursor - tl.offset
		if rel < 0 || rel >= len(rows) {
			t.Fatalf("cursor %d: relative cursor %d outside window of %d rows", cursor, rel, len(rows))
		}
	}
}

func TestClickBelowLastRowReturnsFalse(t *testing.T) {
	tl := NewTrackList(80, 30)
	tl.SetTracks(makeTracks(3), "Test", "ctx")
	// Row 10 is well past the 3 tracks but within the viewport height.
	if clickRow(&tl, 10) {
		t.Error("click below last row should return false")
	}
	if tl.Cursor() != 0 {
		t.Errorf("cursor moved on out-of-range click: %d", tl.Cursor())
	}
}

func TestClickAboveDataReturnsFalse(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	if tl.SetCursorFromClick(0) { // Y=0 is the pane border, above the data area
		t.Error("click in header area should return false")
	}
}

func TestClickBeyondViewportReturnsFalse(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	// A Y far below the viewport must not select the row that happens to sit
	// at offset+thatRow.
	if tl.SetCursorFromClick(tl.headerRows() + tl.capacity() + 5) {
		t.Error("click beyond viewport should return false")
	}
}

func TestClickOnEmptyTrackListReturnsFalse(t *testing.T) {
	tl := NewTrackList(80, 30)
	if clickRow(&tl, 0) {
		t.Error("click on empty tracklist should return false")
	}
	if tl.SelectedTrack() != nil {
		t.Error("empty tracklist SelectedTrack should be nil")
	}
}

func TestClickOnFilteredListMapsToFiltered(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "a", Name: "Apple", Artist: "X", URI: "u:a"},
		{ID: "b", Name: "Banana", Artist: "X", URI: "u:b"},
		{ID: "c", Name: "Apricot", Artist: "X", URI: "u:c"},
	}
	tl.SetTracks(tracks, "Test", "ctx")
	tl.SetFilter("ap") // matches Apple, Apricot
	if !clickRow(&tl, 1) {
		t.Fatal("click on second filtered row returned false")
	}
	if got := tl.SelectedTrack(); got == nil || got.ID != "c" {
		t.Fatalf("filtered click selected %v, want c", got)
	}
}

// ---- movement + scroll invariants ----

func TestMovementClamps(t *testing.T) {
	tl := newScrollableTrackList(t, 50)
	tl.MoveUp(1000)
	if tl.Cursor() != 0 || tl.offset != 0 {
		t.Errorf("MoveUp past top: cursor=%d offset=%d, want 0/0", tl.Cursor(), tl.offset)
	}
	tl.MoveDown(1000)
	if tl.Cursor() != 49 {
		t.Errorf("MoveDown past bottom: cursor=%d, want 49", tl.Cursor())
	}
	tl.GotoTop()
	if tl.Cursor() != 0 {
		t.Errorf("GotoTop cursor = %d, want 0", tl.Cursor())
	}
	tl.GotoBottom()
	if tl.Cursor() != 49 {
		t.Errorf("GotoBottom cursor = %d, want 49", tl.Cursor())
	}
}

func TestHalfPageMovesHalfViewport(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	half := max(1, tl.capacity()/2)
	tl.GotoTop()
	tl.HalfPageDown()
	if tl.Cursor() != half {
		t.Errorf("HalfPageDown cursor = %d, want %d", tl.Cursor(), half)
	}
	tl.HalfPageUp()
	if tl.Cursor() != 0 {
		t.Errorf("HalfPageUp cursor = %d, want 0", tl.Cursor())
	}
}

// Whatever the cursor position, it must always be inside the rendered window.
func TestCursorAlwaysVisible(t *testing.T) {
	tl := newScrollableTrackList(t, 200)
	for i := 0; i < 200; i++ {
		tl.cursor = i
		tl.syncViewport()
		rows := len(tl.table.Rows())
		if tl.cursor < tl.offset || tl.cursor >= tl.offset+rows {
			t.Fatalf("cursor %d not visible: offset=%d rows=%d", i, tl.offset, rows)
		}
		if tl.offset < 0 {
			t.Fatalf("cursor %d: negative offset %d", i, tl.offset)
		}
	}
}

func TestSelectedTrackMatchesCursorAfterScroll(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	for _, want := range []int{0, 13, 64, 99} {
		tl.JumpToTrack(fmt.Sprintf("t%03d", want))
		got := tl.SelectedTrack()
		if got == nil || got.ID != fmt.Sprintf("t%03d", want) {
			t.Fatalf("after jump to %d, SelectedTrack = %v", want, got)
		}
	}
}

func TestNavStateRoundTripsScroll(t *testing.T) {
	tl := newScrollableTrackList(t, 100)
	tl.JumpToTrack("t070")
	wantCursor, wantOffset := tl.cursor, tl.offset

	state := tl.GetState(PaneTrackList)

	// Disturb the list, then restore.
	tl.SetTracks(makeTracks(3), "Other", "ctx2")
	tl.RestoreState(state)

	if tl.cursor != wantCursor || tl.offset != wantOffset {
		t.Errorf("restore: cursor=%d offset=%d, want %d/%d", tl.cursor, tl.offset, wantCursor, wantOffset)
	}
	if got := tl.SelectedTrack(); got == nil || got.ID != "t070" {
		t.Errorf("restore selected %v, want t070", got)
	}
}

// ---- #2: wheel scrolls the pane under the cursor, not the focused pane ----

func wheel(m Model, x int, down bool) (Model, tea.Cmd) {
	btn := tea.MouseButtonWheelUp
	if down {
		btn = tea.MouseButtonWheelDown
	}
	return m.handleMouse(tea.MouseMsg{Button: btn, X: x, Action: tea.MouseActionPress})
}

func TestWheelScrollsPaneUnderCursorNotFocused(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks(makeTracks(100), "Test", "ctx")
	m.sidebar.SetPlaylists([]source.Playlist{
		{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}, {ID: "p3", Name: "Three"},
	})

	sidebarW := max(20, m.width/4)
	trackX := sidebarW + 5
	sidebarX := 1

	// Focus the sidebar, but scroll over the tracklist: tracklist should move.
	m.focusPane = PaneSidebar
	startSidebar := m.sidebar.list.Index()
	m, _ = wheel(m, trackX, true)
	if m.tracklist.Cursor() != wheelScrollLines {
		t.Errorf("wheel over tracklist: cursor = %d, want %d", m.tracklist.Cursor(), wheelScrollLines)
	}
	if m.sidebar.list.Index() != startSidebar {
		t.Errorf("wheel over tracklist moved the sidebar (index %d -> %d)", startSidebar, m.sidebar.list.Index())
	}

	// Now scroll over the sidebar while the tracklist is focused.
	m.focusPane = PaneTrackList
	startTrack := m.tracklist.Cursor()
	m, _ = wheel(m, sidebarX, true)
	if m.sidebar.list.Index() == startSidebar {
		t.Error("wheel over sidebar did not move the sidebar")
	}
	if m.tracklist.Cursor() != startTrack {
		t.Errorf("wheel over sidebar moved the tracklist (%d -> %d)", startTrack, m.tracklist.Cursor())
	}
}

func TestWheelUpAndDownDirections(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks(makeTracks(100), "Test", "ctx")
	trackX := max(20, m.width/4) + 5

	m.tracklist.cursor = 50
	m.tracklist.syncViewport()
	m, _ = wheel(m, trackX, true) // down
	if m.tracklist.Cursor() != 50+wheelScrollLines {
		t.Errorf("wheel down: cursor = %d, want %d", m.tracklist.Cursor(), 50+wheelScrollLines)
	}
	m, _ = wheel(m, trackX, false) // up
	if m.tracklist.Cursor() != 50 {
		t.Errorf("wheel up: cursor = %d, want 50", m.tracklist.Cursor())
	}
}

// ---- #3: single click selects + focuses; double click activates ----

func leftClick(m Model, x, y int) (Model, tea.Cmd) {
	return m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease, X: x, Y: y})
}

func TestSingleClickSelectsAndFocusesTrackList(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks(makeTracks(20), "Test", "ctx")
	m.focusPane = PaneSidebar

	trackX := max(20, m.width/4) + 5
	y := m.tracklist.headerRows() + 2
	m, cmd := leftClick(m, trackX, y)

	if m.focusPane != PaneTrackList {
		t.Error("click in tracklist did not focus it")
	}
	if got := m.tracklist.SelectedTrack(); got == nil || got.ID != "t002" {
		t.Errorf("single click selected %v, want t002", got)
	}
	if cmd != nil {
		t.Error("single click should not trigger playback")
	}
}

func TestDoubleClickPlaysTrack(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks(makeTracks(20), "Test", "ctx")
	m.focusPane = PaneTrackList

	trackX := max(20, m.width/4) + 5
	y := m.tracklist.headerRows() + 1

	m, _ = leftClick(m, trackX, y)    // first click
	m, cmd := leftClick(m, trackX, y) // second click within threshold
	if cmd == nil {
		t.Error("double click on a track should return a playback command")
	}
}

func TestClickInStatusBarIgnored(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks(makeTracks(20), "Test", "ctx")
	statusRows := ArtHeight
	if m.height < MinTermRows {
		statusRows = 2
	}
	m, cmd := leftClick(m, 50, m.height-statusRows+1)
	if cmd != nil {
		t.Error("click in status bar should be a no-op")
	}
}

// ---- #4: overlay mouse handling ----

func TestOverlayLeftClickCloses(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Model)
		want  Mode
	}{
		{"search", func(m *Model) { s := NewSearch(m.width, m.height); m.search = &s; m.mode = ModeSearch }, ModeNormal},
		{"devices", func(m *Model) {
			p := NewDevicePicker([]source.Device{{ID: "d1", Name: "Dev"}}, m.width, m.height)
			m.devices = &p
			m.mode = ModeDevices
		}, ModeNormal},
		{"help", func(m *Model) { m.mode = ModeHelp }, ModeNormal},
		{"nowplaying", func(m *Model) { m.mode = ModeNowPlaying }, ModeNormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(&StubSource{})
			tc.setup(&m)
			m, _ = leftClick(m, 10, 10)
			if m.mode != tc.want {
				t.Errorf("after click, mode = %v, want %v", m.mode, tc.want)
			}
		})
	}
}

func TestActionsOverlayClickReturnsToOrigin(t *testing.T) {
	m := newTestModel(&StubSource{})
	a := NewTrackActions("Song", "Artist", "uri", "ctx", "artist", "album", false, m.width, m.height)
	m.actions = &a
	m.mode = ModeActions
	m.actionsReturn = ModeNowPlaying

	m, _ = leftClick(m, 10, 10)
	if m.mode != ModeNowPlaying {
		t.Errorf("actions click returned to %v, want ModeNowPlaying", m.mode)
	}
	if m.actions != nil {
		t.Error("actions popup should be cleared after click")
	}
}

func TestOverlayWheelScrollsList(t *testing.T) {
	m := newTestModel(&StubSource{})
	p := NewDevicePicker([]source.Device{
		{ID: "d1", Name: "A"}, {ID: "d2", Name: "B"}, {ID: "d3", Name: "C"}, {ID: "d4", Name: "D"},
	}, m.width, m.height)
	m.devices = &p
	m.mode = ModeDevices
	start := m.devices.cursor

	m, _ = wheel(m, 10, true) // wheel down
	if m.devices.cursor <= start {
		t.Errorf("wheel down did not advance device cursor (%d -> %d)", start, m.devices.cursor)
	}
}

func TestInputModeClickIgnored(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeCommand
	m, _ = leftClick(m, 10, 10)
	if m.mode != ModeCommand {
		t.Errorf("click in command mode changed mode to %v", m.mode)
	}
}
