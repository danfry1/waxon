package app

import (
	"context"
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// ---------------------------------------------------------------------------
// trackUpdateMsg handling
// ---------------------------------------------------------------------------

func TestUpdateTrackUpdate(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	track := &source.Track{
		ID:         "track1",
		Name:       "Test Song",
		Artist:     "Artist",
		Album:      "Album",
		ArtworkURL: "https://example.com/art.jpg",
		Duration:   3 * time.Minute,
		Position:   30 * time.Second,
		Playing:    true,
		DeviceName: "My Speaker",
	}

	msg := trackUpdateMsg{
		track:      track,
		volume:     75,
		shuffleOn:  true,
		repeatMode: source.RepeatContext,
	}

	result, _ := m.Update(msg)
	model := result.(Model)

	if model.track == nil {
		t.Fatal("track should be set after trackUpdateMsg")
	}
	if model.track.ID != "track1" {
		t.Errorf("track.ID = %q, want %q", model.track.ID, "track1")
	}
	if model.volume != 75 {
		t.Errorf("volume = %d, want 75", model.volume)
	}
	if !model.shuffleOn {
		t.Error("shuffleOn should be true")
	}
	if model.repeatMode != source.RepeatContext {
		t.Errorf("repeatMode = %q, want %q", model.repeatMode, source.RepeatContext)
	}
	if model.deviceName != "My Speaker" {
		t.Errorf("deviceName = %q, want %q", model.deviceName, "My Speaker")
	}
	if model.consecutiveErrors != 0 {
		t.Errorf("consecutiveErrors = %d, want 0", model.consecutiveErrors)
	}
}

func TestUpdateTrackUpdateNilTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.deviceName = "Old Device"

	msg := trackUpdateMsg{track: nil}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.track != nil {
		t.Error("track should be nil")
	}
	if model.deviceName != "" {
		t.Errorf("deviceName should be cleared, got %q", model.deviceName)
	}
}

// ---------------------------------------------------------------------------
// trackErrorMsg handling
// ---------------------------------------------------------------------------

func TestUpdateTrackError(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := trackErrorMsg{err: errors.New("network timeout")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.consecutiveErrors != 1 {
		t.Errorf("consecutiveErrors = %d, want 1", model.consecutiveErrors)
	}
	if !model.toast.Visible() {
		t.Error("toast should be visible after error")
	}
}

func TestUpdateTrackErrorResetsPaginationLoading(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.pagination = &paginationState{loadingMore: true, playlistID: "pl1"}

	msg := trackErrorMsg{err: errors.New("fail")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.pagination.loadingMore {
		t.Error("pagination.loadingMore should be reset to false")
	}
}

func TestUpdateConsecutiveErrorsReset(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.consecutiveErrors = 5

	msg := trackUpdateMsg{track: nil}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.consecutiveErrors != 0 {
		t.Errorf("consecutiveErrors = %d, want 0 after successful poll", model.consecutiveErrors)
	}
}

// ---------------------------------------------------------------------------
// tracksLoadedMsg handling
// ---------------------------------------------------------------------------

func TestUpdateTracksLoaded(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	tracks := []source.Track{
		{ID: "t1", Name: "Song A", Artist: "Artist", Duration: 3 * time.Minute},
		{ID: "t2", Name: "Song B", Artist: "Artist", Duration: 4 * time.Minute},
	}

	msg := tracksLoadedMsg{
		tracks:     tracks,
		title:      "My Playlist",
		contextURI: "spotify:playlist:abc",
		playlistID: "abc",
		total:      2,
	}

	result, _ := m.Update(msg)
	model := result.(Model)

	if len(model.tracklist.tracks) != 2 {
		t.Errorf("tracklist.tracks = %d, want 2", len(model.tracklist.tracks))
	}
	// Should be cached
	if cached, ok := model.trackCache["abc"]; !ok {
		t.Error("tracks should be cached")
	} else if len(cached.tracks) != 2 {
		t.Errorf("cached tracks = %d, want 2", len(cached.tracks))
	}
	// Pagination should be nil (fully loaded)
	if model.pagination != nil {
		t.Error("pagination should be nil when fully loaded")
	}
}

func TestUpdateTracksLoadedPartial(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
	}

	msg := tracksLoadedMsg{
		tracks:     tracks,
		title:      "Big Playlist",
		contextURI: "spotify:playlist:big",
		playlistID: "big",
		total:      500,
	}

	result, _ := m.Update(msg)
	model := result.(Model)

	if model.pagination == nil {
		t.Fatal("pagination should be set for partial load")
	}
	if model.pagination.total != 500 {
		t.Errorf("pagination.total = %d, want 500", model.pagination.total)
	}
	if model.pagination.loaded != 1 {
		t.Errorf("pagination.loaded = %d, want 1", model.pagination.loaded)
	}
}

// ---------------------------------------------------------------------------
// moreTracksLoadedMsg handling
// ---------------------------------------------------------------------------

func TestUpdateMoreTracksLoaded(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	// Set up initial state with partial load
	initial := []source.Track{
		{ID: "t1", Name: "Song A"},
	}
	m.tracklist.SetTracks(initial, "Playlist", "uri")
	m.pagination = &paginationState{
		playlistID:  "pl1",
		total:       3,
		loaded:      1,
		loadingMore: true,
	}
	m.trackCache = make(map[string]cachedPlaylist)

	more := []source.Track{
		{ID: "t2", Name: "Song B"},
		{ID: "t3", Name: "Song C"},
	}

	msg := moreTracksLoadedMsg{tracks: more, playlistID: "pl1"}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.pagination != nil {
		t.Error("pagination should be nil after all tracks loaded")
	}
	if len(model.tracklist.tracks) != 3 {
		t.Errorf("tracklist.tracks = %d, want 3", len(model.tracklist.tracks))
	}
}

// ---------------------------------------------------------------------------
// Key handling — Normal mode
// ---------------------------------------------------------------------------

func TestHandleKeyQuit(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if !model.quitting {
		t.Error("q key should set quitting=true")
	}
}

func TestHandleKeyModeSwitch(t *testing.T) {
	stub := &StubSource{}

	tests := []struct {
		name string
		key  string
		want Mode
	}{
		{"? opens help", "?", ModeHelp},
		{": opens command", ":", ModeCommand},
		{"/ opens filter", "/", ModeFilter},
		{"N opens now playing", "N", ModeNowPlaying},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(stub)
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			result, _ := m.Update(msg)
			model := result.(Model)
			if model.mode != tt.want {
				t.Errorf("mode = %v, want %v", model.mode, tt.want)
			}
		})
	}
}

func TestHandleKeyFocusSwitch(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar

	// Tab should cycle to TrackList
	msg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := m.Update(msg)
	model := result.(Model)
	if model.focusPane != PaneTrackList {
		t.Errorf("focusPane = %v, want PaneTrackList", model.focusPane)
	}

	// Tab again should cycle back to Sidebar
	result, _ = model.Update(msg)
	model = result.(Model)
	if model.focusPane != PaneSidebar {
		t.Errorf("focusPane = %v, want PaneSidebar", model.focusPane)
	}
}

// ---------------------------------------------------------------------------
// Key handling — Command mode
// ---------------------------------------------------------------------------

func TestHandleKeyCommandEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "partial"

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.cmdInput != "" {
		t.Errorf("cmdInput should be cleared, got %q", model.cmdInput)
	}
}

func TestHandleKeyCommandBackspaceUTF8(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "héllo"

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.cmdInput != "héll" {
		t.Errorf("cmdInput = %q, want %q", model.cmdInput, "héll")
	}
}

// ---------------------------------------------------------------------------
// Key handling — Filter mode
// ---------------------------------------------------------------------------

func TestHandleKeyFilterBackspaceUTF8(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = "日本語"

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.filterInput != "日本" {
		t.Errorf("filterInput = %q, want %q", model.filterInput, "日本")
	}
}

// ---------------------------------------------------------------------------
// Key handling — Help mode
// ---------------------------------------------------------------------------

func TestHandleKeyHelpEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeHelp

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
}

// ---------------------------------------------------------------------------
// Key handling — Now Playing mode
// ---------------------------------------------------------------------------

func TestHandleKeyNowPlayingEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	m.vinylMode = true

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.vinylMode {
		t.Error("vinylMode should be false after escaping NowPlaying")
	}
}

func TestHandleKeyNowPlayingVinylToggle(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if !model.vinylMode {
		t.Error("V should toggle vinyl mode on")
	}

	result, _ = model.Update(msg)
	model = result.(Model)

	if model.vinylMode {
		t.Error("V should toggle vinyl mode off")
	}
}

// ---------------------------------------------------------------------------
// Navigation stack
// ---------------------------------------------------------------------------

func TestPushPopNav(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
		{ID: "t2", Name: "Song B"},
	}
	m.tracklist.SetTracks(tracks, "First", "uri1")

	m.pushNav()

	newTracks := []source.Track{
		{ID: "t3", Name: "Song C"},
	}
	m.tracklist.SetTracks(newTracks, "Second", "uri2")

	if len(m.navStack) != 1 {
		t.Fatalf("navStack length = %d, want 1", len(m.navStack))
	}

	if !m.popNav() {
		t.Fatal("popNav should return true")
	}
	if len(m.navStack) != 0 {
		t.Errorf("navStack length = %d, want 0", len(m.navStack))
	}
}

func TestPopNavEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	if m.popNav() {
		t.Error("popNav on empty stack should return false")
	}
}

func TestNavStackCap(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	m.tracklist.SetTracks(tracks, "Test", "uri")

	for i := 0; i < 25; i++ {
		m.pushNav()
	}

	if len(m.navStack) > maxNavStack {
		t.Errorf("navStack length = %d, exceeds cap %d", len(m.navStack), maxNavStack)
	}
}

// ---------------------------------------------------------------------------
// Cache eviction
// ---------------------------------------------------------------------------

func TestCacheEviction(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.trackCache = make(map[string]cachedPlaylist)

	for i := 0; i < maxTrackCacheEntries+10; i++ {
		id := string(rune('A' + i%26))
		m.trackCache[id] = cachedPlaylist{
			fetchedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
	}

	m.evictTrackCache()

	if len(m.trackCache) > maxTrackCacheEntries {
		t.Errorf("cache size = %d, want <= %d", len(m.trackCache), maxTrackCacheEntries)
	}
}

// ---------------------------------------------------------------------------
// Progress interpolation
// ---------------------------------------------------------------------------

func TestProgressInterpolation(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = &source.Track{
		Playing:  true,
		Duration: 3 * time.Minute,
		Position: 30 * time.Second,
	}
	m.lastPollTime = time.Now().Add(-1 * time.Second)
	m.lastPollPos = 30 * time.Second

	msg := progressTickMsg(time.Now())
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.track.Position <= 30*time.Second {
		t.Error("position should be interpolated forward")
	}
	if model.track.Position > 32*time.Second {
		t.Error("position should not jump more than ~2s forward")
	}
}

func TestProgressInterpolationClampsAtDuration(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = &source.Track{
		Playing:  true,
		Duration: 30 * time.Second,
		Position: 29 * time.Second,
	}
	m.lastPollTime = time.Now().Add(-5 * time.Second)
	m.lastPollPos = 29 * time.Second

	msg := progressTickMsg(time.Now())
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.track.Position > model.track.Duration {
		t.Errorf("position %v should not exceed duration %v", model.track.Position, model.track.Duration)
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestViewQuitting(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.quitting = true

	if got := m.View(); got != "" {
		t.Errorf("View() when quitting should return empty, got %q", got)
	}
}

func TestViewZeroSize(t *testing.T) {
	stub := &StubSource{}
	m := NewModel(stub)

	got := m.View()
	if got != "" {
		t.Errorf("View() with zero size = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// GTracker reset on mode switch
// ---------------------------------------------------------------------------

func TestGTrackerResetOnModeSwitch(t *testing.T) {
	stub := &StubSource{}

	modes := []struct {
		name string
		key  string
		want Mode
	}{
		{"help", "?", ModeHelp},
		{"command", ":", ModeCommand},
		{"filter", "/", ModeFilter},
		{"search", "s", ModeSearch},
		{"nowplaying", "N", ModeNowPlaying},
	}

	for _, tt := range modes {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(stub)
			// Press g to start pending
			gMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
			result, _ := m.Update(gMsg)
			m = result.(Model)
			if !m.gtracker.Pending() {
				t.Fatal("gtracker should be pending after 'g'")
			}

			// Press mode-switch key
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			result, _ = m.Update(msg)
			m = result.(Model)

			if m.gtracker.Pending() {
				t.Errorf("gtracker should be reset after switching to %s mode", tt.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// executeCommand integration
// ---------------------------------------------------------------------------

func TestExecuteCommandVolume(t *testing.T) {
	var calledVol int
	stub := &StubSource{
		SetVolumeFn: func(_ context.Context, v int) error {
			calledVol = v
			return nil
		},
	}
	m := newTestModel(stub)

	// Enter command mode
	m.mode = ModeCommand
	m.cmdInput = "vol 80"

	// Press enter to execute
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.volume != 80 {
		t.Errorf("volume = %d, want 80", model.volume)
	}

	// Execute the returned cmd to verify it calls SetVolume
	if cmd != nil {
		cmd()
	}
	if calledVol != 80 {
		t.Errorf("SetVolume called with %d, want 80", calledVol)
	}
}

func TestExecuteCommandShuffle(t *testing.T) {
	var calledShuffle bool
	stub := &StubSource{
		SetShuffleFn: func(_ context.Context, s bool) error {
			calledShuffle = s
			return nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "shuffle"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if !model.shuffleOn {
		t.Error("shuffleOn should be true after :shuffle")
	}
	if cmd != nil {
		cmd()
	}
	if !calledShuffle {
		t.Error("SetShuffle should have been called with true")
	}
}

func TestExecuteCommandQuit(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "q"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	model := result.(Model)

	if !model.quitting {
		t.Error(":q should set quitting=true")
	}
}

// ---------------------------------------------------------------------------
// buildArtistTrackList
// ---------------------------------------------------------------------------

func TestBuildArtistTrackList(t *testing.T) {
	page := &source.ArtistPage{
		Name: "Test Artist",
		Tracks: []source.Track{
			{ID: "t1", Name: "Top Track 1"},
			{ID: "t2", Name: "Top Track 2"},
		},
		Albums: []source.ArtistAlbum{
			{ID: "a1", Name: "Album One", Year: "2020", Type: "Album"},
			{ID: "s1", Name: "Single One", Year: "2021", Type: "Single"},
			{ID: "a2", Name: "Album Two", Year: "2022", Type: "Album"},
		},
	}

	tracks := buildArtistTrackList(page)

	// Should have: 2 top tracks + separator + "Albums" header + 2 albums +
	//              separator + "Singles & EPs" header + 1 single = 9
	if len(tracks) != 9 {
		t.Fatalf("got %d tracks, want 9", len(tracks))
	}

	// First two are top tracks
	if tracks[0].ID != "t1" {
		t.Errorf("tracks[0].ID = %q, want t1", tracks[0].ID)
	}
	if tracks[1].ID != "t2" {
		t.Errorf("tracks[1].ID = %q, want t2", tracks[1].ID)
	}

	// Separator
	if !tracks[2].IsSeparator {
		t.Error("tracks[2] should be a separator")
	}

	// Albums header
	if !tracks[3].IsSeparator || tracks[3].Name != "Albums" {
		t.Errorf("tracks[3] should be Albums header, got %+v", tracks[3])
	}

	// Album rows
	if !tracks[4].IsAlbumRow || tracks[4].AlbumID != "a1" {
		t.Error("tracks[4] should be album row for a1")
	}
	if !tracks[5].IsAlbumRow || tracks[5].AlbumID != "a2" {
		t.Error("tracks[5] should be album row for a2")
	}

	// Singles section
	if !tracks[7].IsSeparator || tracks[7].Name != "Singles & EPs" {
		t.Errorf("tracks[7] should be Singles header, got %+v", tracks[7])
	}
	if !tracks[8].IsAlbumRow || tracks[8].AlbumID != "s1" {
		t.Error("tracks[8] should be album row for s1")
	}
}

func TestBuildArtistTrackListNoDiscography(t *testing.T) {
	page := &source.ArtistPage{
		Name: "New Artist",
		Tracks: []source.Track{
			{ID: "t1", Name: "Only Track"},
		},
	}

	tracks := buildArtistTrackList(page)
	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}
	if tracks[0].ID != "t1" {
		t.Errorf("tracks[0].ID = %q, want t1", tracks[0].ID)
	}
}

// ===========================================================================
// Group 1: Actions popup (actions.go)
// ===========================================================================

func TestNewTrackActions(t *testing.T) {
	popup := NewTrackActions("MySong", "MyArtist", "spotify:track:123", "artist1", "album1", false, 80, 40)
	if popup.title != "MySong — MyArtist" {
		t.Errorf("title = %q, want %q", popup.title, "MySong — MyArtist")
	}
	if popup.uri != "spotify:track:123" {
		t.Errorf("uri = %q", popup.uri)
	}
	if popup.artistID != "artist1" {
		t.Errorf("artistID = %q", popup.artistID)
	}
	if popup.albumID != "album1" {
		t.Errorf("albumID = %q", popup.albumID)
	}
	if len(popup.items) != 7 {
		t.Errorf("items count = %d, want 7", len(popup.items))
	}
	if popup.cursor != 0 {
		t.Errorf("cursor = %d, want 0", popup.cursor)
	}
}

func TestNewTrackActionsNoArtist(t *testing.T) {
	popup := NewTrackActions("MySong", "", "uri", "", "", false, 80, 40)
	if popup.title != "MySong" {
		t.Errorf("title = %q, want %q", popup.title, "MySong")
	}
}

func TestNewPlaylistActions(t *testing.T) {
	popup := NewPlaylistActions("My Playlist", "spotify:playlist:abc", 80, 40)
	if popup.title != "My Playlist" {
		t.Errorf("title = %q", popup.title)
	}
	if popup.uri != "spotify:playlist:abc" {
		t.Errorf("uri = %q", popup.uri)
	}
	if len(popup.items) != 3 {
		t.Errorf("items count = %d, want 3", len(popup.items))
	}
	if popup.items[0].Type != ActionPlayPlaylist {
		t.Errorf("first item type = %d, want ActionPlayPlaylist", popup.items[0].Type)
	}
}

func TestActionsMoveDownUp(t *testing.T) {
	popup := NewTrackActions("Song", "Artist", "uri", "", "", false, 80, 40)

	// MoveDown
	popup.MoveDown()
	if popup.cursor != 1 {
		t.Errorf("after MoveDown cursor = %d, want 1", popup.cursor)
	}

	// MoveDown to last
	for i := 0; i < 10; i++ {
		popup.MoveDown()
	}
	if popup.cursor != len(popup.items)-1 {
		t.Errorf("cursor should clamp at %d, got %d", len(popup.items)-1, popup.cursor)
	}

	// MoveUp
	popup.MoveUp()
	if popup.cursor != len(popup.items)-2 {
		t.Errorf("after MoveUp cursor = %d, want %d", popup.cursor, len(popup.items)-2)
	}

	// MoveUp to top
	for i := 0; i < 10; i++ {
		popup.MoveUp()
	}
	if popup.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", popup.cursor)
	}
}

func TestActionsSelected(t *testing.T) {
	popup := NewTrackActions("Song", "Artist", "uri", "", "", false, 80, 40)

	sel := popup.Selected()
	if sel.Type != ActionPlay {
		t.Errorf("Selected().Type = %d, want ActionPlay", sel.Type)
	}

	popup.MoveDown()
	sel = popup.Selected()
	if sel.Type != ActionQueue {
		t.Errorf("Selected().Type = %d, want ActionQueue", sel.Type)
	}

	// Out of bounds returns zero value
	popup.cursor = -1
	sel = popup.Selected()
	if sel.Label != "" {
		t.Errorf("out-of-bounds Selected should return zero ActionItem, got %+v", sel)
	}

	popup.cursor = 100
	sel = popup.Selected()
	if sel.Label != "" {
		t.Errorf("over-bounds Selected should return zero ActionItem, got %+v", sel)
	}
}

func TestActionsGetters(t *testing.T) {
	popup := NewTrackActions("Song", "Artist", "spotify:track:x", "art1", "alb1", false, 80, 40)
	if popup.URI() != "spotify:track:x" {
		t.Errorf("URI() = %q", popup.URI())
	}
	if popup.ArtistID() != "art1" {
		t.Errorf("ArtistID() = %q", popup.ArtistID())
	}
	if popup.AlbumID() != "alb1" {
		t.Errorf("AlbumID() = %q", popup.AlbumID())
	}
}

// ===========================================================================
// Group 2: Device picker (devices.go)
// ===========================================================================

func TestNewDevicePickerActiveDevice(t *testing.T) {
	devices := []source.Device{
		{ID: "d1", Name: "Speaker", Type: "Speaker", IsActive: false},
		{ID: "d2", Name: "Computer", Type: "Computer", IsActive: true},
		{ID: "d3", Name: "Phone", Type: "Smartphone", IsActive: false},
	}
	picker := NewDevicePicker(devices, 80, 40)
	if picker.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (active device)", picker.cursor)
	}
}

func TestNewDevicePickerNoActive(t *testing.T) {
	devices := []source.Device{
		{ID: "d1", Name: "Speaker", Type: "Speaker", IsActive: false},
		{ID: "d2", Name: "Computer", Type: "Computer", IsActive: false},
	}
	picker := NewDevicePicker(devices, 80, 40)
	if picker.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (default when no active)", picker.cursor)
	}
}

func TestDevicePickerMoveDownUp(t *testing.T) {
	devices := []source.Device{
		{ID: "d1", Name: "A"},
		{ID: "d2", Name: "B"},
		{ID: "d3", Name: "C"},
	}
	picker := NewDevicePicker(devices, 80, 40)

	picker.MoveDown()
	if picker.cursor != 1 {
		t.Errorf("cursor = %d, want 1", picker.cursor)
	}

	// Clamp at end
	for i := 0; i < 10; i++ {
		picker.MoveDown()
	}
	if picker.cursor != 2 {
		t.Errorf("cursor should clamp at 2, got %d", picker.cursor)
	}

	// MoveUp
	picker.MoveUp()
	if picker.cursor != 1 {
		t.Errorf("cursor = %d, want 1", picker.cursor)
	}

	// Clamp at top
	for i := 0; i < 10; i++ {
		picker.MoveUp()
	}
	if picker.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", picker.cursor)
	}
}

func TestDevicePickerSelected(t *testing.T) {
	devices := []source.Device{
		{ID: "d1", Name: "Speaker"},
		{ID: "d2", Name: "Computer"},
	}
	picker := NewDevicePicker(devices, 80, 40)

	sel := picker.Selected()
	if sel == nil || sel.ID != "d1" {
		t.Errorf("Selected() = %v, want device d1", sel)
	}

	picker.MoveDown()
	sel = picker.Selected()
	if sel == nil || sel.ID != "d2" {
		t.Errorf("Selected() = %v, want device d2", sel)
	}

	// Empty picker
	empty := NewDevicePicker(nil, 80, 40)
	if empty.Selected() != nil {
		t.Error("Selected() on empty picker should be nil")
	}
}

func TestDeviceIcon(t *testing.T) {
	tests := []struct {
		deviceType string
		want       string
	}{
		{"Computer", "[PC]"},
		{"Smartphone", "[Phone]"},
		{"Speaker", "[Speaker]"},
		{"TV", "[TV]"},
		{"CastAudio", "[Cast]"},
		{"CastVideo", "[Cast]"},
		{"Tablet", "[Tablet]"},
		{"Unknown", "[Unknown]"},
	}
	for _, tt := range tests {
		t.Run(tt.deviceType, func(t *testing.T) {
			got := deviceIcon(tt.deviceType)
			if got != tt.want {
				t.Errorf("deviceIcon(%q) = %q, want %q", tt.deviceType, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Group 3: View rendering
// ===========================================================================

func TestViewHelp(t *testing.T) {
	got := ViewHelp(80, 40)
	if got == "" {
		t.Fatal("ViewHelp should return non-empty string")
	}
	if !strings.Contains(got, "Keybindings") {
		t.Error("ViewHelp output should contain 'Keybindings'")
	}
}

func TestViewModeHelp(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeHelp

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeHelp should be non-empty")
	}
	if !strings.Contains(got, "Keybindings") {
		t.Error("View() in ModeHelp should contain 'Keybindings'")
	}
}

func TestViewModeNowPlaying(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeNowPlaying should be non-empty")
	}
}

func TestViewModeNowPlayingWithTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	m.track = &source.Track{
		Name:     "Test Song",
		Artist:   "Test Artist",
		Album:    "Test Album",
		Duration: 3 * time.Minute,
		Position: 30 * time.Second,
	}

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeNowPlaying with track should be non-empty")
	}
}

func TestViewModeSearch(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	s := NewSearch(m.width, m.height)
	m.search = &s
	m.mode = ModeSearch

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeSearch should be non-empty")
	}
}

func TestViewModeActions(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	popup := NewTrackActions("Song", "Artist", "uri", "", "", false, m.width, m.height)
	m.actions = &popup
	m.mode = ModeActions

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeActions should be non-empty")
	}
}

func TestViewModeDevices(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	devices := []source.Device{
		{ID: "d1", Name: "Speaker", Type: "Speaker"},
	}
	picker := NewDevicePicker(devices, m.width, m.height)
	m.devices = &picker
	m.mode = ModeDevices

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeDevices should be non-empty")
	}
}

func TestViewNormalMode(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNormal

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeNormal should be non-empty")
	}
}

func TestViewNormalModeWithTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNormal
	m.track = &source.Track{
		Name:     "Playing Song",
		Artist:   "Artist",
		Album:    "Album",
		Duration: 3 * time.Minute,
		Position: 1 * time.Minute,
		Playing:  true,
	}

	got := m.View()
	if got == "" {
		t.Fatal("View() in ModeNormal with track should be non-empty")
	}
}

// ===========================================================================
// Group 4: Key handling in modes
// ===========================================================================

func TestHandleKeySearchEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	s := NewSearch(m.width, m.height)
	m.search = &s
	m.mode = ModeSearch

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.search != nil {
		t.Error("search should be nil after Escape")
	}
}

func TestHandleKeySearchNilSearch(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeSearch
	m.search = nil

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	// Should not panic, mode should stay
	if model.mode != ModeSearch {
		t.Errorf("mode = %v, want ModeSearch (nil search returns early)", model.mode)
	}
}

func TestHandleKeyActionsNavigation(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	popup := NewTrackActions("Song", "Artist", "uri", "", "", false, m.width, m.height)
	m.actions = &popup
	m.mode = ModeActions

	// j moves down
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	result, _ := m.Update(msg)
	model := result.(Model)
	if model.actions.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", model.actions.cursor)
	}

	// k moves up
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	result, _ = model.Update(msg)
	model = result.(Model)
	if model.actions.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", model.actions.cursor)
	}
}

func TestHandleKeyActionsEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	popup := NewTrackActions("Song", "Artist", "uri", "", "", false, m.width, m.height)
	m.actions = &popup
	m.mode = ModeActions

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.actions != nil {
		t.Error("actions should be nil after Escape")
	}
}

func TestHandleKeyActionsQuit(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	popup := NewTrackActions("Song", "Artist", "uri", "", "", false, m.width, m.height)
	m.actions = &popup
	m.mode = ModeActions

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.actions != nil {
		t.Error("actions should be nil after q")
	}
}

func TestHandleKeyActionsNilActions(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeActions
	m.actions = nil

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	result, _ := m.Update(msg)
	_ = result.(Model) // should not panic
}

func TestHandleKeyDevicesNavigation(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	devices := []source.Device{
		{ID: "d1", Name: "A"},
		{ID: "d2", Name: "B"},
		{ID: "d3", Name: "C"},
	}
	picker := NewDevicePicker(devices, m.width, m.height)
	m.devices = &picker
	m.mode = ModeDevices

	// j moves down
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	result, _ := m.Update(msg)
	model := result.(Model)
	if model.devices.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", model.devices.cursor)
	}

	// k moves up
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	result, _ = model.Update(msg)
	model = result.(Model)
	if model.devices.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", model.devices.cursor)
	}
}

func TestHandleKeyDevicesEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	devices := []source.Device{{ID: "d1", Name: "A"}}
	picker := NewDevicePicker(devices, m.width, m.height)
	m.devices = &picker
	m.mode = ModeDevices

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.devices != nil {
		t.Error("devices should be nil after Escape")
	}
}

func TestHandleKeyDevicesQuit(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	devices := []source.Device{{ID: "d1", Name: "A"}}
	picker := NewDevicePicker(devices, m.width, m.height)
	m.devices = &picker
	m.mode = ModeDevices

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
}

func TestHandleKeyDevicesNilDevices(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeDevices
	m.devices = nil

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	result, _ := m.Update(msg)
	_ = result.(Model) // should not panic
}

func TestHandleKeyFilterEnterClearsEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = ""

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.filterInput != "" {
		t.Errorf("filterInput = %q, want empty", model.filterInput)
	}
}

func TestHandleKeyFilterEnterWithText(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = "rock"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Enter", model.mode)
	}
}

func TestHandleKeyFilterEscape(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = "test"

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal after Escape", model.mode)
	}
	if model.filterInput != "" {
		t.Errorf("filterInput should be cleared, got %q", model.filterInput)
	}
}

func TestHandleKeyFilterBackspace(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = "abc"

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.filterInput != "ab" {
		t.Errorf("filterInput = %q, want %q", model.filterInput, "ab")
	}
}

func TestHandleKeyFilterRunesAppend(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = "he"

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.filterInput != "hel" {
		t.Errorf("filterInput = %q, want %q", model.filterInput, "hel")
	}
}

func TestHandleKeyFilterSpace(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.filterInput = "foo"

	msg := tea.KeyMsg{Type: tea.KeySpace}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.filterInput != "foo " {
		t.Errorf("filterInput = %q, want %q", model.filterInput, "foo ")
	}
}

func TestHandleKeyNormalFocusSwitch(t *testing.T) {
	stub := &StubSource{}

	// h focuses left (sidebar)
	m := newTestModel(stub)
	m.focusPane = PaneTrackList
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}
	result, _ := m.Update(msg)
	model := result.(Model)
	if model.focusPane != PaneSidebar {
		t.Errorf("h key: focusPane = %v, want PaneSidebar", model.focusPane)
	}

	// l focuses right (tracklist)
	m = newTestModel(stub)
	m.focusPane = PaneSidebar
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	result, _ = m.Update(msg)
	model = result.(Model)
	if model.focusPane != PaneTrackList {
		t.Errorf("l key: focusPane = %v, want PaneTrackList", model.focusPane)
	}
}

func TestHandleKeyNormalSectionSwitch(t *testing.T) {
	stub := &StubSource{}

	// 1 switches to Library
	m := newTestModel(stub)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}
	result, _ := m.Update(msg)
	model := result.(Model)
	if model.focusPane != PaneSidebar {
		t.Errorf("1 key: focusPane = %v, want PaneSidebar", model.focusPane)
	}

	// 2 switches to Queue
	m = newTestModel(stub)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}
	result, _ = m.Update(msg)
	model = result.(Model)
	if model.focusPane != PaneSidebar {
		t.Errorf("2 key: focusPane = %v, want PaneSidebar", model.focusPane)
	}
}

func TestHandleKeyNormalPlayPause(t *testing.T) {
	var pauseCalled, playCalled bool
	stub := &StubSource{
		PauseFn: func(_ context.Context) error {
			pauseCalled = true
			return nil
		},
		PlayFn: func(_ context.Context) error {
			playCalled = true
			return nil
		},
	}

	// Space with playing track => pause
	m := newTestModel(stub)
	m.track = &source.Track{Playing: true}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
	_, cmd := m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !pauseCalled {
		t.Error("space with playing track should call Pause")
	}

	// Space with paused track => play
	pauseCalled = false
	m = newTestModel(stub)
	m.track = &source.Track{Playing: false}
	_, cmd = m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !playCalled {
		t.Error("space with paused track should call Play")
	}
}

func TestHandleKeyNormalNextPrev(t *testing.T) {
	var nextCalled, prevCalled bool
	stub := &StubSource{
		NextFn: func(_ context.Context) error {
			nextCalled = true
			return nil
		},
		PreviousFn: func(_ context.Context) error {
			prevCalled = true
			return nil
		},
	}

	// n => next
	m := newTestModel(stub)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	_, cmd := m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !nextCalled {
		t.Error("n should call Next")
	}

	// p => previous
	m = newTestModel(stub)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	_, cmd = m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !prevCalled {
		t.Error("p should call Previous")
	}
}

func TestHandleKeyNormalBack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	// Back with empty nav stack shows toast
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	result, _ := m.Update(msg)
	model := result.(Model)
	if !model.toast.Visible() {
		t.Error("Back with empty stack should show toast")
	}
}

func TestHandleKeyNormalG(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	// 'G' goes to bottom
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")}
	result, _ := m.Update(msg)
	_ = result.(Model) // should not panic
}

// ===========================================================================
// Group 5: Command execution
// ===========================================================================

func TestExecuteCommandRepeatOff(t *testing.T) {
	var calledMode source.RepeatMode
	stub := &StubSource{
		SetRepeatFn: func(_ context.Context, mode source.RepeatMode) error {
			calledMode = mode
			return nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "repeat off"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.repeatMode != source.RepeatOff {
		t.Errorf("repeatMode = %q, want %q", model.repeatMode, source.RepeatOff)
	}
	if cmd != nil {
		cmd()
	}
	if calledMode != source.RepeatOff {
		t.Errorf("SetRepeat called with %q, want %q", calledMode, source.RepeatOff)
	}
}

func TestExecuteCommandRepeatAll(t *testing.T) {
	var calledMode source.RepeatMode
	stub := &StubSource{
		SetRepeatFn: func(_ context.Context, mode source.RepeatMode) error {
			calledMode = mode
			return nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "repeat all"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.repeatMode != source.RepeatContext {
		t.Errorf("repeatMode = %q, want %q", model.repeatMode, source.RepeatContext)
	}
	if cmd != nil {
		cmd()
	}
	if calledMode != source.RepeatContext {
		t.Errorf("SetRepeat called with %q, want %q", calledMode, source.RepeatContext)
	}
}

func TestExecuteCommandDevice(t *testing.T) {
	stub := &StubSource{
		DevicesFn: func(_ context.Context) ([]source.Device, error) {
			return []source.Device{{ID: "d1", Name: "Test"}}, nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "device"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)
	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}

	// Execute the cmd which should fetch devices
	if cmd != nil {
		cmdMsg := cmd()
		// Process the devicesLoadedMsg
		result, _ = model.Update(cmdMsg)
		model = result.(Model)
		if model.mode != ModeDevices {
			t.Errorf("after loading devices, mode = %v, want ModeDevices", model.mode)
		}
	}
}

func TestExecuteCommandRecent(t *testing.T) {
	stub := &StubSource{
		RecentlyPlayedFn: func(_ context.Context) ([]source.Track, error) {
			return []source.Track{{ID: "r1", Name: "Recent Track"}}, nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "recent"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

	if cmd != nil {
		cmdMsg := cmd()
		if _, ok := cmdMsg.(recentTracksLoadedMsg); !ok {
			t.Errorf("expected recentTracksLoadedMsg, got %T", cmdMsg)
		}
	}
}

func TestExecuteCommandSearchQuery(t *testing.T) {
	stub := &StubSource{
		SearchFn: func(_ context.Context, query string) (*source.SearchResults, error) {
			return &source.SearchResults{
				Tracks: []source.Track{{ID: "s1", Name: query}},
			}, nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "search test query"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeSearch {
		t.Errorf("mode = %v, want ModeSearch", model.mode)
	}
	if model.search == nil {
		t.Fatal("search should not be nil")
	}

	// Execute the search command
	if cmd != nil {
		cmdMsg := cmd()
		if srm, ok := cmdMsg.(searchResultsMsg); ok {
			if srm.query != "test query" {
				t.Errorf("search query = %q, want %q", srm.query, "test query")
			}
		} else {
			t.Errorf("expected searchResultsMsg, got %T", cmdMsg)
		}
	}
}

func TestExecuteCommandUnknown(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "unknowncmd"

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	model := result.(Model)

	if !model.toast.Visible() {
		t.Error("unknown command should show toast error")
	}
}

func TestControlCmdSuccess(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	cmd := m.controlCmd(func(_ context.Context) error {
		return nil
	})

	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
}

func TestControlCmdError(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	cmd := m.controlCmd(func(_ context.Context) error {
		return errors.New("test error")
	})

	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestHandleAddQueueNoTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	// No tracks loaded, so SelectedTrack() returns nil

	result, _ := m.handleAddQueue()
	if !result.toast.Visible() {
		t.Error("handleAddQueue with no track should show toast")
	}
}

func TestHandleAddQueueWithTrack(t *testing.T) {
	var queuedID string
	stub := &StubSource{
		AddToQueueFn: func(_ context.Context, id string) error {
			queuedID = id
			return nil
		},
	}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A", Artist: "Artist", Duration: 3 * time.Minute},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	_, cmd := m.handleAddQueue()
	if cmd != nil {
		msg := cmd()
		if qdm, ok := msg.(queueDoneMsg); ok {
			if qdm.trackName != "Song A" {
				t.Errorf("trackName = %q, want %q", qdm.trackName, "Song A")
			}
		} else {
			t.Errorf("expected queueDoneMsg, got %T", msg)
		}
	}
	if queuedID != "t1" {
		t.Errorf("AddToQueue called with %q, want %q", queuedID, "t1")
	}
}

func TestHandleAddQueueSeparatorRow(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := []source.Track{
		{Name: "---", IsSeparator: true},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	result, _ := m.handleAddQueue()
	if !result.toast.Visible() {
		t.Error("handleAddQueue with separator row should show toast")
	}
}

func TestJumpToCurrentTrackNoTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	result, _ := m.jumpToCurrentTrack()
	if !result.toast.Visible() {
		t.Error("jumpToCurrentTrack with no track should show toast")
	}
}

func TestJumpToCurrentTrackFound(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t2", Name: "Current Song"}
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
		{ID: "t2", Name: "Current Song"},
		{ID: "t3", Name: "Song C"},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")

	result, _ := m.jumpToCurrentTrack()
	if result.focusPane != PaneTrackList {
		t.Errorf("focusPane = %v, want PaneTrackList", result.focusPane)
	}
	if !result.toast.Visible() {
		t.Error("jumpToCurrentTrack should show toast")
	}
}

func TestJumpToCurrentTrackNotInView(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t99", Name: "Not Here"}
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")

	result, _ := m.jumpToCurrentTrack()
	if !result.toast.Visible() {
		t.Error("jumpToCurrentTrack when track not found should show toast")
	}
}

// ===========================================================================
// Group 6: Fetch commands
// ===========================================================================

func TestFetchCurrentTrackSuccess(t *testing.T) {
	stub := &StubSource{
		CurrentPlaybackFn: func(_ context.Context) (*source.PlaybackState, error) {
			return &source.PlaybackState{
				Track:      &source.Track{ID: "t1", Name: "Song"},
				Volume:     65,
				ShuffleOn:  true,
				RepeatMode: source.RepeatTrack,
			}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchCurrentTrack()
	msg := cmd()
	tum, ok := msg.(trackUpdateMsg)
	if !ok {
		t.Fatalf("expected trackUpdateMsg, got %T", msg)
	}
	if tum.track.ID != "t1" {
		t.Errorf("track.ID = %q, want t1", tum.track.ID)
	}
	if tum.volume != 65 {
		t.Errorf("volume = %d, want 65", tum.volume)
	}
	if !tum.shuffleOn {
		t.Error("shuffleOn should be true")
	}
	if tum.repeatMode != source.RepeatTrack {
		t.Errorf("repeatMode = %q, want %q", tum.repeatMode, source.RepeatTrack)
	}
}

func TestFetchCurrentTrackNilPlayback(t *testing.T) {
	stub := &StubSource{
		CurrentPlaybackFn: func(_ context.Context) (*source.PlaybackState, error) {
			return nil, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchCurrentTrack()
	msg := cmd()
	tum, ok := msg.(trackUpdateMsg)
	if !ok {
		t.Fatalf("expected trackUpdateMsg, got %T", msg)
	}
	if tum.track != nil {
		t.Error("track should be nil for nil playback")
	}
}

func TestFetchCurrentTrackError(t *testing.T) {
	stub := &StubSource{
		CurrentPlaybackFn: func(_ context.Context) (*source.PlaybackState, error) {
			return nil, errors.New("network error")
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchCurrentTrack()
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestFetchPlaylists(t *testing.T) {
	stub := &StubSource{
		PlaylistsFn: func(_ context.Context) ([]source.Playlist, error) {
			return []source.Playlist{
				{ID: "p1", Name: "Playlist 1"},
				{ID: "p2", Name: "Playlist 2"},
			}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchPlaylists()
	msg := cmd()
	plm, ok := msg.(playlistsLoadedMsg)
	if !ok {
		t.Fatalf("expected playlistsLoadedMsg, got %T", msg)
	}
	if len(plm.playlists) != 2 {
		t.Errorf("playlists count = %d, want 2", len(plm.playlists))
	}
}

func TestFetchPlaylistsError(t *testing.T) {
	stub := &StubSource{
		PlaylistsFn: func(_ context.Context) ([]source.Playlist, error) {
			return nil, errors.New("fail")
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchPlaylists()
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestFetchQueue(t *testing.T) {
	stub := &StubSource{
		QueueFn: func(_ context.Context) ([]source.Track, error) {
			return []source.Track{
				{ID: "q1", Name: "Queue Track 1"},
			}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchQueue()
	msg := cmd()
	qlm, ok := msg.(queueLoadedMsg)
	if !ok {
		t.Fatalf("expected queueLoadedMsg, got %T", msg)
	}
	if len(qlm.tracks) != 1 {
		t.Errorf("queue tracks = %d, want 1", len(qlm.tracks))
	}
}

func TestFetchQueueError(t *testing.T) {
	stub := &StubSource{
		QueueFn: func(_ context.Context) ([]source.Track, error) {
			return nil, errors.New("fail")
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchQueue()
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestFetchRecentlyPlayed(t *testing.T) {
	stub := &StubSource{
		RecentlyPlayedFn: func(_ context.Context) ([]source.Track, error) {
			return []source.Track{
				{ID: "r1", Name: "Recent 1"},
				{ID: "r2", Name: "Recent 2"},
			}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchRecentlyPlayed()
	msg := cmd()
	rtm, ok := msg.(recentTracksLoadedMsg)
	if !ok {
		t.Fatalf("expected recentTracksLoadedMsg, got %T", msg)
	}
	if len(rtm.tracks) != 2 {
		t.Errorf("recent tracks = %d, want 2", len(rtm.tracks))
	}
}

func TestFetchRecentlyPlayedError(t *testing.T) {
	stub := &StubSource{
		RecentlyPlayedFn: func(_ context.Context) ([]source.Track, error) {
			return nil, errors.New("fail")
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchRecentlyPlayed()
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestFetchDevices(t *testing.T) {
	stub := &StubSource{
		DevicesFn: func(_ context.Context) ([]source.Device, error) {
			return []source.Device{
				{ID: "d1", Name: "Speaker", Type: "Speaker"},
			}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchDevices()
	msg := cmd()
	dlm, ok := msg.(devicesLoadedMsg)
	if !ok {
		t.Fatalf("expected devicesLoadedMsg, got %T", msg)
	}
	if len(dlm.devices) != 1 {
		t.Errorf("devices count = %d, want 1", len(dlm.devices))
	}
}

func TestFetchDevicesError(t *testing.T) {
	stub := &StubSource{
		DevicesFn: func(_ context.Context) ([]source.Device, error) {
			return nil, errors.New("fail")
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchDevices()
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestDoSearch(t *testing.T) {
	stub := &StubSource{
		SearchFn: func(_ context.Context, query string) (*source.SearchResults, error) {
			return &source.SearchResults{
				Tracks: []source.Track{{ID: "s1", Name: "Found"}},
			}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.doSearch("test")
	msg := cmd()
	srm, ok := msg.(searchResultsMsg)
	if !ok {
		t.Fatalf("expected searchResultsMsg, got %T", msg)
	}
	if srm.query != "test" {
		t.Errorf("query = %q, want %q", srm.query, "test")
	}
	if len(srm.results.Tracks) != 1 {
		t.Errorf("results tracks = %d, want 1", len(srm.results.Tracks))
	}
}

func TestDoSearchError(t *testing.T) {
	stub := &StubSource{
		SearchFn: func(_ context.Context, query string) (*source.SearchResults, error) {
			return nil, errors.New("search failed")
		},
	}
	m := newTestModel(stub)

	cmd := m.doSearch("test")
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestPlayTrackWithContext(t *testing.T) {
	var calledCtx, calledTrack string
	stub := &StubSource{
		PlayTrackFn: func(_ context.Context, contextURI, trackURI string) error {
			calledCtx = contextURI
			calledTrack = trackURI
			return nil
		},
	}
	m := newTestModel(stub)

	cmd := m.playTrack("spotify:track:123", "spotify:playlist:abc")
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledCtx != "spotify:playlist:abc" {
		t.Errorf("contextURI = %q", calledCtx)
	}
	if calledTrack != "spotify:track:123" {
		t.Errorf("trackURI = %q", calledTrack)
	}
}

func TestPlayTrackDirect(t *testing.T) {
	var calledTrack string
	stub := &StubSource{
		PlayTrackDirectFn: func(_ context.Context, trackURI string) error {
			calledTrack = trackURI
			return nil
		},
	}
	m := newTestModel(stub)

	cmd := m.playTrack("spotify:track:123", "")
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledTrack != "spotify:track:123" {
		t.Errorf("trackURI = %q", calledTrack)
	}
}

func TestPlayTrackNoURI(t *testing.T) {
	var playCalled bool
	stub := &StubSource{
		PlayFn: func(_ context.Context) error {
			playCalled = true
			return nil
		},
	}
	m := newTestModel(stub)

	cmd := m.playTrack("", "")
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if !playCalled {
		t.Error("playTrack with empty URIs should call Play")
	}
}

func TestPlayTrackError(t *testing.T) {
	stub := &StubSource{
		PlayTrackDirectFn: func(_ context.Context, trackURI string) error {
			return errors.New("play error")
		},
	}
	m := newTestModel(stub)

	cmd := m.playTrack("spotify:track:123", "")
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

func TestSeekRelativeNilTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = nil

	cmd := m.seekRelative(5 * time.Second)
	if cmd != nil {
		t.Error("seekRelative with nil track should return nil")
	}
}

func TestSeekRelativeForward(t *testing.T) {
	var seekPos time.Duration
	stub := &StubSource{
		SeekFn: func(_ context.Context, pos time.Duration) error {
			seekPos = pos
			return nil
		},
	}
	m := newTestModel(stub)
	m.track = &source.Track{Position: 30 * time.Second, Duration: 3 * time.Minute}

	cmd := m.seekRelative(5 * time.Second)
	if cmd == nil {
		t.Fatal("seekRelative should return a command")
	}
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if seekPos != 35*time.Second {
		t.Errorf("seek position = %v, want 35s", seekPos)
	}
}

func TestSeekRelativeBackwardClampZero(t *testing.T) {
	var seekPos time.Duration
	stub := &StubSource{
		SeekFn: func(_ context.Context, pos time.Duration) error {
			seekPos = pos
			return nil
		},
	}
	m := newTestModel(stub)
	m.track = &source.Track{Position: 2 * time.Second, Duration: 3 * time.Minute}

	cmd := m.seekRelative(-5 * time.Second)
	if cmd == nil {
		t.Fatal("seekRelative should return a command")
	}
	cmd()
	if seekPos != 0 {
		t.Errorf("seek position = %v, want 0 (clamped)", seekPos)
	}
}

// ===========================================================================
// Additional message handling coverage
// ===========================================================================

func TestUpdateDevicesLoadedEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := devicesLoadedMsg{devices: nil}
	result, _ := m.Update(msg)
	model := result.(Model)

	// Empty device list should show error toast, not open picker
	if !model.toast.Visible() {
		t.Error("empty devices should show toast")
	}
	if model.mode == ModeDevices {
		t.Error("mode should not be ModeDevices with empty device list")
	}
}

func TestUpdateDevicesLoadedNonEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := devicesLoadedMsg{devices: []source.Device{{ID: "d1", Name: "Test"}}}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeDevices {
		t.Errorf("mode = %v, want ModeDevices", model.mode)
	}
	if model.devices == nil {
		t.Error("devices picker should be set")
	}
}

func TestUpdateControlDoneMsg(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := controlDoneMsg{}
	result, _ := m.Update(msg)
	_ = result.(Model) // should not panic
}

func TestUpdateQueueDoneMsg(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := queueDoneMsg{trackName: "My Song"}
	result, _ := m.Update(msg)
	model := result.(Model)

	if !model.toast.Visible() {
		t.Error("queueDoneMsg should show toast")
	}
}

func TestUpdateCmdFlashMsg(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := cmdFlashMsg{text: "Volume: 80%"}
	result, _ := m.Update(msg)
	model := result.(Model)

	if !model.toast.Visible() {
		t.Error("cmdFlashMsg should show toast")
	}
}

func TestUpdateClearToastMsg(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.toast.Show("test", "", ToastInfo)

	msg := clearToastMsg{}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.toast.Visible() {
		t.Error("clearToastMsg should hide toast")
	}
}

func TestUpdateRecentTracksLoadedMsg(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := recentTracksLoadedMsg{tracks: []source.Track{
		{ID: "r1", Name: "Recent Song"},
	}}
	result, _ := m.Update(msg)
	model := result.(Model)

	if len(model.tracklist.tracks) != 1 {
		t.Errorf("tracklist tracks = %d, want 1", len(model.tracklist.tracks))
	}
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	msg := tea.WindowSizeMsg{Width: 160, Height: 50}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.width != 160 {
		t.Errorf("width = %d, want 160", model.width)
	}
	if model.height != 50 {
		t.Errorf("height = %d, want 50", model.height)
	}
}

// ===========================================================================
// Actions and Devices View rendering
// ===========================================================================

func TestActionsView(t *testing.T) {
	popup := NewTrackActions("Song", "Artist", "uri", "art1", "alb1", false, 80, 40)
	got := popup.View()
	if got == "" {
		t.Fatal("ActionsPopup.View() should be non-empty")
	}
	if !strings.Contains(got, "Actions") {
		t.Error("ActionsPopup.View() should contain 'Actions'")
	}
}

func TestDevicePickerView(t *testing.T) {
	devices := []source.Device{
		{ID: "d1", Name: "Speaker", Type: "Speaker", IsActive: true},
		{ID: "d2", Name: "Computer", Type: "Computer"},
	}
	picker := NewDevicePicker(devices, 80, 40)
	got := picker.View()
	if got == "" {
		t.Fatal("DevicePicker.View() should be non-empty")
	}
	if !strings.Contains(got, "Devices") {
		t.Error("DevicePicker.View() should contain 'Devices'")
	}
}

func TestDevicePickerViewEmpty(t *testing.T) {
	picker := NewDevicePicker(nil, 80, 40)
	got := picker.View()
	if got == "" {
		t.Fatal("DevicePicker.View() with no devices should be non-empty")
	}
}

// ===========================================================================
// handleKeyNowPlaying additional coverage
// ===========================================================================

func TestHandleKeyNowPlayingPlayPause(t *testing.T) {
	var pauseCalled bool
	stub := &StubSource{
		PauseFn: func(_ context.Context) error {
			pauseCalled = true
			return nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	m.track = &source.Track{Playing: true}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
	_, cmd := m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !pauseCalled {
		t.Error("space in NowPlaying with playing track should call Pause")
	}
}

func TestHandleKeyNowPlayingNextPrev(t *testing.T) {
	var nextCalled, prevCalled bool
	stub := &StubSource{
		NextFn: func(_ context.Context) error {
			nextCalled = true
			return nil
		},
		PreviousFn: func(_ context.Context) error {
			prevCalled = true
			return nil
		},
	}

	m := newTestModel(stub)
	m.mode = ModeNowPlaying

	// Next
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	_, cmd := m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !nextCalled {
		t.Error("n in NowPlaying should call Next")
	}

	// Previous
	m = newTestModel(stub)
	m.mode = ModeNowPlaying
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	_, cmd = m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if !prevCalled {
		t.Error("p in NowPlaying should call Previous")
	}
}

func TestHandleKeyNowPlayingSeek(t *testing.T) {
	var seekPos time.Duration
	stub := &StubSource{
		SeekFn: func(_ context.Context, pos time.Duration) error {
			seekPos = pos
			return nil
		},
	}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	m.track = &source.Track{Position: 30 * time.Second, Duration: 3 * time.Minute}

	// Seek forward with ]
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")}
	_, cmd := m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if seekPos != 35*time.Second {
		t.Errorf("seek forward: pos = %v, want 35s", seekPos)
	}

	// Seek backward with [
	m = newTestModel(stub)
	m.mode = ModeNowPlaying
	m.track = &source.Track{Position: 30 * time.Second, Duration: 3 * time.Minute}
	seekPos = 0

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")}
	_, cmd = m.Update(msg)
	if cmd != nil {
		cmd()
	}
	if seekPos != 25*time.Second {
		t.Errorf("seek backward: pos = %v, want 25s", seekPos)
	}
}

func TestHandleKeyNowPlayingQuit(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	m.vinylMode = true

	// q should close NowPlaying and reset vinyl mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.vinylMode {
		t.Error("vinylMode should be false after q in NowPlaying")
	}
}

// ===========================================================================
// handleKeyCommand additional coverage
// ===========================================================================

func TestHandleKeyCommandSpace(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "vol"

	msg := tea.KeyMsg{Type: tea.KeySpace}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.cmdInput != "vol " {
		t.Errorf("cmdInput = %q, want %q", model.cmdInput, "vol ")
	}
}

func TestHandleKeyCommandRunes(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = "vo"

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.cmdInput != "vol" {
		t.Errorf("cmdInput = %q, want %q", model.cmdInput, "vol")
	}
}

func TestHandleKeyCommandBackspaceEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeCommand
	m.cmdInput = ""

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.cmdInput != "" {
		t.Errorf("cmdInput = %q, want empty", model.cmdInput)
	}
}

// ===========================================================================
// Search model tests
// ===========================================================================

func TestSearchView(t *testing.T) {
	s := NewSearch(80, 40)
	got := s.View()
	if got == "" {
		t.Fatal("Search.View() should be non-empty")
	}
}

func TestSearchSelectedTrack(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks: []source.Track{
			{ID: "t1", Name: "Track 1"},
			{ID: "t2", Name: "Track 2"},
		},
	}
	s.cursor = 0
	if track := s.SelectedTrack(); track == nil || track.ID != "t1" {
		t.Errorf("SelectedTrack() at cursor 0 = %v, want t1", track)
	}
	s.cursor = 1
	if track := s.SelectedTrack(); track == nil || track.ID != "t2" {
		t.Errorf("SelectedTrack() at cursor 1 = %v, want t2", track)
	}
	// Out of range
	s.cursor = 10
	if track := s.SelectedTrack(); track != nil {
		t.Errorf("SelectedTrack() out of range should be nil, got %v", track)
	}
}

func TestSearchSelectedArtist(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks:  []source.Track{{ID: "t1"}},
		Artists: []source.SearchArtist{{ID: "a1", Name: "Artist 1"}},
	}
	// Cursor 0 is on tracks, not artists
	s.cursor = 0
	if artist := s.SelectedArtist(); artist != nil {
		t.Error("SelectedArtist() should be nil when cursor is on track")
	}
	// Cursor 1 is on first artist
	s.cursor = 1
	if artist := s.SelectedArtist(); artist == nil || artist.ID != "a1" {
		t.Errorf("SelectedArtist() at cursor 1 = %v, want a1", artist)
	}
}

func TestSearchSelectedAlbum(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks:  []source.Track{{ID: "t1"}},
		Artists: []source.SearchArtist{{ID: "a1"}},
		Albums:  []source.SearchAlbum{{ID: "al1", Name: "Album 1"}},
	}
	// Cursor 2 = first album (1 track + 1 artist)
	s.cursor = 2
	if album := s.SelectedAlbum(); album == nil || album.ID != "al1" {
		t.Errorf("SelectedAlbum() at cursor 2 = %v, want al1", album)
	}
	// Not on album
	s.cursor = 0
	if album := s.SelectedAlbum(); album != nil {
		t.Error("SelectedAlbum() should be nil when cursor is not on album")
	}
}

func TestSearchViewWithResults(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks: []source.Track{
			{ID: "t1", Name: "Found Song", Artist: "Artist", Duration: 3 * time.Minute},
		},
		Artists: []source.SearchArtist{
			{ID: "a1", Name: "Found Artist"},
		},
		Albums: []source.SearchAlbum{
			{ID: "al1", Name: "Found Album", Artist: "Album Artist"},
		},
	}
	got := s.View()
	if got == "" {
		t.Fatal("Search.View() with results should be non-empty")
	}
}

// ===========================================================================
// Fetch commands: fetchArtistPage, fetchAlbumPage
// ===========================================================================

func TestFetchArtistPage(t *testing.T) {
	stub := &StubSource{
		GetArtistFn: func(_ context.Context, id string) (*source.ArtistPage, error) {
			return &source.ArtistPage{Name: "Test Artist"}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchArtistPage("artist123")
	msg := cmd()
	apm, ok := msg.(artistPageLoadedMsg)
	if !ok {
		t.Fatalf("expected artistPageLoadedMsg, got %T", msg)
	}
	if apm.page.Name != "Test Artist" {
		t.Errorf("artist name = %q", apm.page.Name)
	}
}

func TestFetchAlbumPage(t *testing.T) {
	stub := &StubSource{
		GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
			return &source.AlbumPage{Name: "Test Album", ID: id}, nil
		},
	}
	m := newTestModel(stub)

	cmd := m.fetchAlbumPage("album123")
	msg := cmd()
	apm, ok := msg.(albumPageLoadedMsg)
	if !ok {
		t.Fatalf("expected albumPageLoadedMsg, got %T", msg)
	}
	if apm.page.Name != "Test Album" {
		t.Errorf("album name = %q", apm.page.Name)
	}
}

// ===========================================================================
// handleKeyFilter on sidebar pane
// ===========================================================================

func TestHandleKeyFilterSidebarPane(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.mode = ModeFilter
	m.focusPane = PaneSidebar
	m.filterInput = ""

	// Type a character
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	result, _ := m.Update(msg)
	model := result.(Model)
	if model.filterInput != "a" {
		t.Errorf("filterInput = %q, want %q", model.filterInput, "a")
	}

	// Escape clears sidebar filter
	msg = tea.KeyMsg{Type: tea.KeyEscape}
	result, _ = model.Update(msg)
	model = result.(Model)
	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
}

// ===========================================================================
// handleKeyDevices Enter
// ===========================================================================

func TestHandleKeyDevicesEnter(t *testing.T) {
	var transferredID string
	stub := &StubSource{
		TransferPlaybackFn: func(_ context.Context, id string) error {
			transferredID = id
			return nil
		},
	}
	m := newTestModel(stub)
	devices := []source.Device{
		{ID: "d1", Name: "My Speaker"},
	}
	picker := NewDevicePicker(devices, m.width, m.height)
	m.devices = &picker
	m.mode = ModeDevices

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.devices != nil {
		t.Error("devices should be nil after selection")
	}

	// Execute the transfer command
	if cmd != nil {
		cmdMsg := cmd()
		if fm, ok := cmdMsg.(cmdFlashMsg); ok {
			if !strings.Contains(fm.text, "My Speaker") {
				t.Errorf("flash text = %q, should mention device name", fm.text)
			}
		}
	}
	if transferredID != "d1" {
		t.Errorf("transferred device ID = %q, want d1", transferredID)
	}
}

// ===========================================================================
// handleKeyActions Enter
// ===========================================================================

func TestHandleKeyActionsEnterPlay(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A", URI: "spotify:track:t1"},
	}
	m.tracklist.SetTracks(tracks, "Test", "spotify:playlist:abc")
	m.focusPane = PaneTrackList

	popup := NewTrackActions("Song A", "Artist", "spotify:track:t1", "", "", false, m.width, m.height)
	m.actions = &popup
	m.mode = ModeActions

	// Enter on first item (Play)
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.actions != nil {
		t.Error("actions should be nil after Enter")
	}
}

// ===========================================================================
// g-prefix motions in normal mode
// ===========================================================================

func TestGTrackerMotionGG(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
		{ID: "t2", Name: "Song B"},
		{ID: "t3", Name: "Song C"},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	// Press g, then g
	g1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	result, _ := m.Update(g1)
	model := result.(Model)
	if !model.gtracker.Pending() {
		t.Fatal("gtracker should be pending after first g")
	}

	g2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	result, _ = model.Update(g2)
	model = result.(Model)
	if model.gtracker.Pending() {
		t.Error("gtracker should not be pending after gg")
	}
}

func TestGTrackerMotionGL(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneTrackList

	g1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	result, _ := m.Update(g1)
	model := result.(Model)

	g2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	result, _ = model.Update(g2)
	model = result.(Model)

	if model.focusPane != PaneSidebar {
		t.Errorf("gl should switch to PaneSidebar, got %v", model.focusPane)
	}
}

func TestGTrackerMotionGQ(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	g1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	result, _ := m.Update(g1)
	model := result.(Model)

	g2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	result, _ = model.Update(g2)
	model = result.(Model)

	if model.focusPane != PaneSidebar {
		t.Errorf("gq should switch to PaneSidebar, got %v", model.focusPane)
	}
}

func TestGTrackerMotionGC(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	// No track playing
	m.track = nil

	g1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	result, _ := m.Update(g1)
	model := result.(Model)

	g2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	result, _ = model.Update(g2)
	model = result.(Model)

	// Should show toast "No track playing"
	if !model.toast.Visible() {
		t.Error("gc with no track should show toast")
	}
}

func TestGTrackerMotionGR(t *testing.T) {
	stub := &StubSource{
		RecentlyPlayedFn: func(_ context.Context) ([]source.Track, error) {
			return []source.Track{{ID: "r1", Name: "Recent"}}, nil
		},
	}
	m := newTestModel(stub)

	g1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	result, _ := m.Update(g1)
	model := result.(Model)

	g2 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	result, cmd := model.Update(g2)
	_ = result.(Model)

	// Should have fetched recently played
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(recentTracksLoadedMsg); !ok {
			t.Errorf("gr should fetch recently played, got %T", msg)
		}
	}
}

// ===========================================================================
// 1. handleEnter
// ===========================================================================

func TestHandleEnterSidebarPlaylist(t *testing.T) {
	var calledID string
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, id string, offset, limit int) ([]source.Track, int, error) {
			calledID = id
			return []source.Track{{ID: "t1", Name: "Song"}}, 1, nil
		},
	}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	m.sidebar.SetPlaylists([]source.Playlist{
		{ID: "pl1", URI: "spotify:playlist:pl1", Name: "My Playlist", TrackCount: 5},
	})

	result, cmd := m.handleEnter()
	_ = result

	if cmd == nil {
		t.Fatal("handleEnter on sidebar playlist should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(tracksLoadedMsg); !ok {
		t.Errorf("expected tracksLoadedMsg, got %T", msg)
	}
	if calledID != "pl1" {
		t.Errorf("called with ID %q, want %q", calledID, "pl1")
	}
}

func TestHandleEnterTracklistRegularTrack(t *testing.T) {
	var calledTrackURI string
	stub := &StubSource{
		PlayTrackFn: func(_ context.Context, contextURI, trackURI string) error {
			calledTrackURI = trackURI
			return nil
		},
	}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", URI: "spotify:track:t1", Name: "Song A", Artist: "Artist"},
	}
	m.tracklist.SetTracks(tracks, "Test", "spotify:playlist:abc")
	m.focusPane = PaneTrackList

	_, cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on regular track should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledTrackURI != "spotify:track:t1" {
		t.Errorf("trackURI = %q, want %q", calledTrackURI, "spotify:track:t1")
	}
}

func TestHandleEnterSeparatorRow(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := []source.Track{
		{Name: "---", IsSeparator: true},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	_, cmd := m.handleEnter()
	if cmd != nil {
		t.Error("handleEnter on separator should return nil cmd")
	}
}

func TestHandleEnterAlbumRow(t *testing.T) {
	var calledAlbumID string
	stub := &StubSource{
		GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
			calledAlbumID = id
			return &source.AlbumPage{ID: id, Name: "Test Album"}, nil
		},
	}
	m := newTestModel(stub)
	tracks := []source.Track{
		{Name: "Album X", AlbumID: "alb1", IsAlbumRow: true},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	_, cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on album row should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(albumPageLoadedMsg); !ok {
		t.Errorf("expected albumPageLoadedMsg, got %T", msg)
	}
	if calledAlbumID != "alb1" {
		t.Errorf("album ID = %q, want %q", calledAlbumID, "alb1")
	}
}

func TestHandleEnterSidebarQueue(t *testing.T) {
	var calledTrackURI string
	stub := &StubSource{
		PlayTrackDirectFn: func(_ context.Context, trackURI string) error {
			calledTrackURI = trackURI
			return nil
		},
	}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	m.sidebar.SetSection(SectionQueue)
	m.sidebar.SetQueueTracks([]source.Track{
		{ID: "qt1", URI: "spotify:track:qt1", Name: "Queue Track"},
	})

	_, cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on queue track should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledTrackURI != "spotify:track:qt1" {
		t.Errorf("trackURI = %q, want %q", calledTrackURI, "spotify:track:qt1")
	}
}

// ===========================================================================
// 2. openActions
// ===========================================================================

func TestOpenActionsTracklistValidTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", URI: "spotify:track:t1", Name: "Song A", Artist: "Artist", ArtistID: "art1", AlbumID: "alb1"},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	result, _ := m.openActions()
	if result.mode != ModeActions {
		t.Errorf("mode = %v, want ModeActions", result.mode)
	}
	if result.actions == nil {
		t.Fatal("actions popup should be set")
	}
}

func TestOpenActionsTracklistNoTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneTrackList
	// No tracks loaded

	result, _ := m.openActions()
	if !result.toast.Visible() {
		t.Error("openActions with no track should show toast error")
	}
}

func TestOpenActionsTracklistSeparator(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	tracks := []source.Track{
		{Name: "---", IsSeparator: true},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	result, _ := m.openActions()
	if !result.toast.Visible() {
		t.Error("openActions on separator should show toast error")
	}
}

func TestOpenActionsTracklistAlbumRow(t *testing.T) {
	var calledAlbumID string
	stub := &StubSource{
		GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
			calledAlbumID = id
			return &source.AlbumPage{ID: id, Name: "Album"}, nil
		},
	}
	m := newTestModel(stub)
	tracks := []source.Track{
		{Name: "Album X", AlbumID: "alb1", IsAlbumRow: true},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	_, cmd := m.openActions()
	if cmd == nil {
		t.Fatal("openActions on album row should return fetch cmd")
	}
	msg := cmd()
	if _, ok := msg.(albumPageLoadedMsg); !ok {
		t.Errorf("expected albumPageLoadedMsg, got %T", msg)
	}
	if calledAlbumID != "alb1" {
		t.Errorf("album ID = %q, want %q", calledAlbumID, "alb1")
	}
}

func TestOpenActionsSidebarPlaylist(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	m.sidebar.SetPlaylists([]source.Playlist{
		{ID: "pl1", URI: "spotify:playlist:pl1", Name: "My Playlist"},
	})

	result, _ := m.openActions()
	if result.mode != ModeActions {
		t.Errorf("mode = %v, want ModeActions", result.mode)
	}
	if result.actions == nil {
		t.Fatal("actions popup should be set for sidebar playlist")
	}
}

func TestOpenActionsSidebarNoPlaylist(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	// No playlists loaded

	result, _ := m.openActions()
	if !result.toast.Visible() {
		t.Error("openActions with no sidebar playlist should show toast error")
	}
}

// ===========================================================================
// 3. executeAction
// ===========================================================================

func TestExecuteActionPlay(t *testing.T) {
	var calledTrackURI string
	stub := &StubSource{
		PlayTrackFn: func(_ context.Context, contextURI, trackURI string) error {
			calledTrackURI = trackURI
			return nil
		},
	}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", URI: "spotify:track:t1", Name: "Song A"},
	}
	m.tracklist.SetTracks(tracks, "Test", "spotify:playlist:abc")
	m.focusPane = PaneTrackList

	action := ActionItem{Type: ActionPlay, Label: "Play"}
	_, cmd := m.executeAction(action, "spotify:track:t1", "", "")
	if cmd == nil {
		t.Fatal("executeAction Play should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledTrackURI != "spotify:track:t1" {
		t.Errorf("trackURI = %q, want %q", calledTrackURI, "spotify:track:t1")
	}
}

func TestExecuteActionQueue(t *testing.T) {
	var queuedID string
	stub := &StubSource{
		AddToQueueFn: func(_ context.Context, id string) error {
			queuedID = id
			return nil
		},
	}
	m := newTestModel(stub)
	tracks := []source.Track{
		{ID: "t1", URI: "spotify:track:t1", Name: "Song A"},
	}
	m.tracklist.SetTracks(tracks, "Test", "uri")
	m.focusPane = PaneTrackList

	action := ActionItem{Type: ActionQueue, Label: "Add to Queue"}
	_, cmd := m.executeAction(action, "spotify:track:t1", "", "")
	if cmd == nil {
		t.Fatal("executeAction Queue should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(queueDoneMsg); !ok {
		t.Errorf("expected queueDoneMsg, got %T", msg)
	}
	if queuedID != "t1" {
		t.Errorf("queued ID = %q, want %q", queuedID, "t1")
	}
}

func TestExecuteActionGoArtistValid(t *testing.T) {
	var calledArtistID string
	stub := &StubSource{
		GetArtistFn: func(_ context.Context, id string) (*source.ArtistPage, error) {
			calledArtistID = id
			return &source.ArtistPage{Name: "Test Artist"}, nil
		},
	}
	m := newTestModel(stub)
	// Need existing tracks so pushNav works
	m.tracklist.SetTracks([]source.Track{{ID: "t1", Name: "Song"}}, "Test", "uri")

	action := ActionItem{Type: ActionGoArtist, Label: "Go to Artist"}
	result, cmd := m.executeAction(action, "", "artist1", "")
	if cmd == nil {
		t.Fatal("executeAction GoArtist should return a cmd")
	}
	_ = result
	msg := cmd()
	if _, ok := msg.(artistPageLoadedMsg); !ok {
		t.Errorf("expected artistPageLoadedMsg, got %T", msg)
	}
	if calledArtistID != "artist1" {
		t.Errorf("artist ID = %q, want %q", calledArtistID, "artist1")
	}
}

func TestExecuteActionGoArtistEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	action := ActionItem{Type: ActionGoArtist, Label: "Go to Artist"}
	result, _ := m.executeAction(action, "", "", "")
	if !result.toast.Visible() {
		t.Error("executeAction GoArtist with empty ID should show toast error")
	}
}

func TestExecuteActionGoAlbumValid(t *testing.T) {
	var calledAlbumID string
	stub := &StubSource{
		GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
			calledAlbumID = id
			return &source.AlbumPage{ID: id, Name: "Test Album"}, nil
		},
	}
	m := newTestModel(stub)
	m.tracklist.SetTracks([]source.Track{{ID: "t1", Name: "Song"}}, "Test", "uri")

	action := ActionItem{Type: ActionGoAlbum, Label: "Go to Album"}
	_, cmd := m.executeAction(action, "", "", "album1")
	if cmd == nil {
		t.Fatal("executeAction GoAlbum should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(albumPageLoadedMsg); !ok {
		t.Errorf("expected albumPageLoadedMsg, got %T", msg)
	}
	if calledAlbumID != "album1" {
		t.Errorf("album ID = %q, want %q", calledAlbumID, "album1")
	}
}

func TestExecuteActionGoAlbumEmpty(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)

	action := ActionItem{Type: ActionGoAlbum, Label: "Go to Album"}
	result, _ := m.executeAction(action, "", "", "")
	if !result.toast.Visible() {
		t.Error("executeAction GoAlbum with empty ID should show toast error")
	}
}

func TestExecuteActionPlayPlaylist(t *testing.T) {
	var calledContextURI string
	stub := &StubSource{
		PlayTrackFn: func(_ context.Context, contextURI, trackURI string) error {
			calledContextURI = contextURI
			return nil
		},
	}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	m.sidebar.SetPlaylists([]source.Playlist{
		{ID: "pl1", URI: "spotify:playlist:pl1", Name: "My Playlist"},
	})

	action := ActionItem{Type: ActionPlayPlaylist, Label: "Play Playlist"}
	_, cmd := m.executeAction(action, "spotify:playlist:pl1", "", "")
	if cmd == nil {
		t.Fatal("executeAction PlayPlaylist should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledContextURI != "spotify:playlist:pl1" {
		t.Errorf("contextURI = %q, want %q", calledContextURI, "spotify:playlist:pl1")
	}
}

func TestExecuteActionLoadTracks(t *testing.T) {
	var calledID string
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, id string, offset, limit int) ([]source.Track, int, error) {
			calledID = id
			return []source.Track{{ID: "t1", Name: "Song"}}, 1, nil
		},
	}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	m.sidebar.SetPlaylists([]source.Playlist{
		{ID: "pl1", URI: "spotify:playlist:pl1", Name: "My Playlist"},
	})

	action := ActionItem{Type: ActionLoadTracks, Label: "Load Tracks"}
	_, cmd := m.executeAction(action, "spotify:playlist:pl1", "", "")
	if cmd == nil {
		t.Fatal("executeAction LoadTracks should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(tracksLoadedMsg); !ok {
		t.Errorf("expected tracksLoadedMsg, got %T", msg)
	}
	if calledID != "pl1" {
		t.Errorf("playlist ID = %q, want %q", calledID, "pl1")
	}
}

// ===========================================================================
// 4. handleKeySearch
// ===========================================================================

func TestHandleKeySearchEnterTrack(t *testing.T) {
	var calledTrackURI string
	stub := &StubSource{
		PlayTrackDirectFn: func(_ context.Context, trackURI string) error {
			calledTrackURI = trackURI
			return nil
		},
	}
	m := newTestModel(stub)
	s := NewSearch(120, 40)
	s.results = &source.SearchResults{
		Tracks:  []source.Track{{ID: "t1", URI: "spotify:track:t1", Name: "Song"}},
		Artists: []source.SearchArtist{{ID: "a1", Name: "Artist"}},
		Albums:  []source.SearchAlbum{{ID: "al1", Name: "Album"}},
	}
	s.cursor = 0 // on first track
	m.search = &s
	m.mode = ModeSearch

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.search != nil {
		t.Error("search should be nil after selecting track")
	}
	if cmd == nil {
		t.Fatal("should return playTrack cmd")
	}
	cmdMsg := cmd()
	if _, ok := cmdMsg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", cmdMsg)
	}
	if calledTrackURI != "spotify:track:t1" {
		t.Errorf("trackURI = %q, want %q", calledTrackURI, "spotify:track:t1")
	}
}

func TestHandleKeySearchEnterArtist(t *testing.T) {
	var calledArtistID string
	stub := &StubSource{
		GetArtistFn: func(_ context.Context, id string) (*source.ArtistPage, error) {
			calledArtistID = id
			return &source.ArtistPage{Name: "Test Artist"}, nil
		},
	}
	m := newTestModel(stub)
	s := NewSearch(120, 40)
	s.results = &source.SearchResults{
		Tracks:  []source.Track{{ID: "t1", URI: "spotify:track:t1", Name: "Song"}},
		Artists: []source.SearchArtist{{ID: "a1", Name: "Artist"}},
		Albums:  []source.SearchAlbum{{ID: "al1", Name: "Album"}},
	}
	s.cursor = 1 // on first artist (1 track before)
	m.search = &s
	m.mode = ModeSearch

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.search != nil {
		t.Error("search should be nil after selecting artist")
	}
	if cmd == nil {
		t.Fatal("should return fetchArtistPage cmd")
	}
	cmdMsg := cmd()
	if _, ok := cmdMsg.(artistPageLoadedMsg); !ok {
		t.Errorf("expected artistPageLoadedMsg, got %T", cmdMsg)
	}
	if calledArtistID != "a1" {
		t.Errorf("artist ID = %q, want %q", calledArtistID, "a1")
	}
}

func TestHandleKeySearchEnterAlbum(t *testing.T) {
	var calledAlbumID string
	stub := &StubSource{
		GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
			calledAlbumID = id
			return &source.AlbumPage{ID: id, Name: "Test Album"}, nil
		},
	}
	m := newTestModel(stub)
	s := NewSearch(120, 40)
	s.results = &source.SearchResults{
		Tracks:  []source.Track{{ID: "t1", URI: "spotify:track:t1", Name: "Song"}},
		Artists: []source.SearchArtist{{ID: "a1", Name: "Artist"}},
		Albums:  []source.SearchAlbum{{ID: "al1", Name: "Album"}},
	}
	s.cursor = 2 // on first album (1 track + 1 artist before)
	m.search = &s
	m.mode = ModeSearch

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.mode != ModeNormal {
		t.Errorf("mode = %v, want ModeNormal", model.mode)
	}
	if model.search != nil {
		t.Error("search should be nil after selecting album")
	}
	if cmd == nil {
		t.Fatal("should return fetchAlbumPage cmd")
	}
	cmdMsg := cmd()
	if _, ok := cmdMsg.(albumPageLoadedMsg); !ok {
		t.Errorf("expected albumPageLoadedMsg, got %T", cmdMsg)
	}
	if calledAlbumID != "al1" {
		t.Errorf("album ID = %q, want %q", calledAlbumID, "al1")
	}
}

func TestHandleKeySearchEnterNoResults(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	s := NewSearch(120, 40)
	s.results = nil
	m.search = &s
	m.mode = ModeSearch

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(msg)
	model := result.(Model)

	// Should stay in search mode with no results
	if model.search == nil {
		t.Error("search should not be nil when no results and Enter")
	}
	if cmd != nil {
		t.Error("should return nil cmd when no search results")
	}
}

// ===========================================================================
// 5. fetchPlaylistTracks and fetchMoreTracks
// ===========================================================================

func TestFetchPlaylistTracksCacheHit(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.trackCache["pl1"] = cachedPlaylist{
		tracks:    []source.Track{{ID: "t1", Name: "Cached Song"}},
		total:     1,
		fetchedAt: time.Now(), // fresh cache
	}

	pl := source.Playlist{ID: "pl1", URI: "spotify:playlist:pl1", Name: "Cached Playlist", ImageURL: "http://img.test/1.jpg"}
	cmd := m.fetchPlaylistTracks(pl)
	if cmd == nil {
		t.Fatal("fetchPlaylistTracks should return cmd even for cache hit")
	}
	msg := cmd()
	tlm, ok := msg.(tracksLoadedMsg)
	if !ok {
		t.Fatalf("expected tracksLoadedMsg, got %T", msg)
	}
	if len(tlm.tracks) != 1 || tlm.tracks[0].ID != "t1" {
		t.Errorf("cached track = %+v, want t1", tlm.tracks)
	}
	if tlm.title != "Cached Playlist" {
		t.Errorf("title = %q, want %q", tlm.title, "Cached Playlist")
	}
}

func TestFetchPlaylistTracksCacheMiss(t *testing.T) {
	var calledID string
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, id string, offset, limit int) ([]source.Track, int, error) {
			calledID = id
			return []source.Track{{ID: "t2", Name: "API Song"}}, 1, nil
		},
	}
	m := newTestModel(stub)

	pl := source.Playlist{ID: "pl2", URI: "spotify:playlist:pl2", Name: "API Playlist"}
	cmd := m.fetchPlaylistTracks(pl)
	if cmd == nil {
		t.Fatal("fetchPlaylistTracks should return cmd for cache miss")
	}
	msg := cmd()
	tlm, ok := msg.(tracksLoadedMsg)
	if !ok {
		t.Fatalf("expected tracksLoadedMsg, got %T", msg)
	}
	if calledID != "pl2" {
		t.Errorf("called ID = %q, want %q", calledID, "pl2")
	}
	if len(tlm.tracks) != 1 || tlm.tracks[0].ID != "t2" {
		t.Errorf("API track = %+v, want t2", tlm.tracks)
	}
}

func TestFetchMoreTracksWithPagination(t *testing.T) {
	var calledOffset int
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, id string, offset, limit int) ([]source.Track, int, error) {
			calledOffset = offset
			return []source.Track{{ID: "t3", Name: "Page 2 Song"}}, 200, nil
		},
	}
	m := newTestModel(stub)
	m.pagination = &paginationState{
		playlistID: "pl1",
		total:      200,
		loaded:     100,
	}

	cmd := m.fetchMoreTracks()
	if cmd == nil {
		t.Fatal("fetchMoreTracks should return cmd with active pagination")
	}
	msg := cmd()
	mtm, ok := msg.(moreTracksLoadedMsg)
	if !ok {
		t.Fatalf("expected moreTracksLoadedMsg, got %T", msg)
	}
	if calledOffset != 100 {
		t.Errorf("offset = %d, want 100", calledOffset)
	}
	if len(mtm.tracks) != 1 {
		t.Errorf("tracks count = %d, want 1", len(mtm.tracks))
	}
}

func TestFetchMoreTracksNilPagination(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.pagination = nil

	cmd := m.fetchMoreTracks()
	if cmd != nil {
		t.Error("fetchMoreTracks with nil pagination should return nil")
	}
}

// ===========================================================================
// 6. Sidebar methods
// ===========================================================================

func TestSidebarSetQueueTracks(t *testing.T) {
	sb := NewSidebar(30, 20)
	tracks := []source.Track{
		{ID: "q1", Name: "Song A", Artist: "Artist A"},
		{ID: "q2", Name: "Song B", Artist: "Artist B"},
		{ID: "q3", Name: "Song C", Artist: "Artist C"},
	}
	sb.SetQueueTracks(tracks)

	items := sb.list.Items()
	if len(items) != 3 {
		t.Errorf("items count = %d, want 3", len(items))
	}
}

func TestSidebarSelectedPlaylist(t *testing.T) {
	sb := NewSidebar(30, 20)
	playlists := []source.Playlist{
		{ID: "pl1", Name: "Playlist 1"},
		{ID: "pl2", Name: "Playlist 2"},
	}
	sb.SetPlaylists(playlists)

	pl := sb.SelectedPlaylist()
	if pl == nil {
		t.Fatal("SelectedPlaylist should not be nil")
	}
	if pl.ID != "pl1" {
		t.Errorf("SelectedPlaylist().ID = %q, want %q", pl.ID, "pl1")
	}
}

func TestSidebarSelectedPlaylistEmpty(t *testing.T) {
	sb := NewSidebar(30, 20)
	if sb.SelectedPlaylist() != nil {
		t.Error("SelectedPlaylist on empty sidebar should be nil")
	}
}

func TestSidebarSectionSetSection(t *testing.T) {
	sb := NewSidebar(30, 20)
	if sb.Section() != SectionLibrary {
		t.Errorf("initial section = %v, want SectionLibrary", sb.Section())
	}

	sb.SetSection(SectionQueue)
	if sb.Section() != SectionQueue {
		t.Errorf("section = %v, want SectionQueue", sb.Section())
	}

	sb.SetSection(SectionLibrary)
	if sb.Section() != SectionLibrary {
		t.Errorf("section = %v, want SectionLibrary", sb.Section())
	}
}

func TestSidebarSetFilterClearFilter(t *testing.T) {
	sb := NewSidebar(30, 20)
	playlists := []source.Playlist{
		{ID: "pl1", Name: "Rock Mix"},
		{ID: "pl2", Name: "Pop Hits"},
		{ID: "pl3", Name: "Rock Classics"},
	}
	sb.SetPlaylists(playlists)

	sb.SetFilter("rock")
	items := sb.list.Items()
	if len(items) != 2 {
		t.Errorf("filtered items = %d, want 2", len(items))
	}

	sb.ClearFilter()
	items = sb.list.Items()
	if len(items) != 3 {
		t.Errorf("after clear, items = %d, want 3", len(items))
	}
}

func TestSidebarSetFilterEmpty(t *testing.T) {
	sb := NewSidebar(30, 20)
	playlists := []source.Playlist{
		{ID: "pl1", Name: "Rock Mix"},
		{ID: "pl2", Name: "Pop Hits"},
	}
	sb.SetPlaylists(playlists)

	// Empty filter should clear
	sb.SetFilter("")
	items := sb.list.Items()
	if len(items) != 2 {
		t.Errorf("items after empty filter = %d, want 2", len(items))
	}
}

func TestSidebarItemTitleDescriptionFilterValue(t *testing.T) {
	item := sidebarItem{playlist: source.Playlist{Name: "My Playlist", TrackCount: 42}}

	title := item.Title()
	if !strings.Contains(title, "My Playlist") {
		t.Errorf("Title() = %q, should contain playlist name", title)
	}

	desc := item.Description()
	if desc != "42 tracks" {
		t.Errorf("Description() = %q, want %q", desc, "42 tracks")
	}

	fv := item.FilterValue()
	if fv != "My Playlist" {
		t.Errorf("FilterValue() = %q, want %q", fv, "My Playlist")
	}
}

func TestSidebarItemTitleWithIcon(t *testing.T) {
	item := sidebarItem{
		playlist: source.Playlist{Name: "My Playlist"},
		icon:     "ICON",
	}

	title := item.Title()
	if !strings.HasPrefix(title, "ICON ") {
		t.Errorf("Title() = %q, should start with icon", title)
	}
}

func TestSidebarIconsSurviveQueueToggle(t *testing.T) {
	sb := NewSidebar(30, 20)
	playlists := []source.Playlist{
		{ID: "pl1", Name: "Playlist 1"},
		{ID: "pl2", Name: "Playlist 2"},
	}
	sb.SetPlaylists(playlists)

	// Simulate icons arriving
	sb.SetPlaylistIcons(map[string]string{
		"pl1": "ICON1",
		"pl2": "ICON2",
	})

	// Verify icons are set
	items := sb.list.Items()
	if si, ok := items[0].(sidebarItem); !ok || si.icon != "ICON1" {
		t.Fatalf("before toggle: item 0 icon = %q, want %q", si.icon, "ICON1")
	}

	// Simulate the Bubbletea update cycle: value-receiver Update returns
	// a new Sidebar which replaces the old one (just like the real app does
	// with m.sidebar, _ = m.sidebar.Update(msg)).
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Switch to queue (key "2")
	sb.SetSection(SectionQueue)
	sb.SetQueueTracks([]source.Track{{ID: "t1", Name: "Track 1", Artist: "Artist"}})

	// Simulate more updates while on queue view
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Switch back to library (key "1")
	sb.SetSection(SectionLibrary)

	// Icons must still be present
	items = sb.list.Items()
	if len(items) != 2 {
		t.Fatalf("after toggle: item count = %d, want 2", len(items))
	}
	for i, item := range items {
		si, ok := item.(sidebarItem)
		if !ok {
			t.Fatalf("item %d is not sidebarItem", i)
		}
		if si.icon == "" {
			t.Errorf("item %d (%s) lost its icon after queue toggle", i, si.playlist.Name)
		}
	}
}

// ===========================================================================
// 7. renderVinyl
// ===========================================================================

func TestRenderVinylSmallImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	got := renderVinyl(img, 0.0, 10, 5, nil)
	if got == "" {
		t.Fatal("renderVinyl with valid image should return non-empty string")
	}
	if !strings.Contains(got, "▀") {
		t.Error("output should contain half-block character")
	}
}

func TestRenderVinylZeroDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if got := renderVinyl(img, 0.0, 0, 0, nil); got != "" {
		t.Errorf("renderVinyl(0,0) = %q, want empty", got)
	}
	if got := renderVinyl(img, 0.0, 0, 5, nil); got != "" {
		t.Errorf("renderVinyl(0,5) = %q, want empty", got)
	}
	if got := renderVinyl(img, 0.0, 5, 0, nil); got != "" {
		t.Errorf("renderVinyl(5,0) = %q, want empty", got)
	}
}

func TestRenderVinylNilImage(t *testing.T) {
	if got := renderVinyl(nil, 0.0, 10, 5, nil); got != "" {
		t.Errorf("renderVinyl(nil) = %q, want empty", got)
	}
}

// ===========================================================================
// 8. computeBgRows and bilinearBlend
// ===========================================================================

func TestComputeBgRowsSmallImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	rows := computeBgRows(img, 20)
	if len(rows) != 20 {
		t.Errorf("computeBgRows returned %d rows, want 20", len(rows))
	}
}

func TestComputeBgRowsSingleRow(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	rows := computeBgRows(img, 1)
	if len(rows) != 1 {
		t.Errorf("computeBgRows returned %d rows, want 1", len(rows))
	}
}

func TestBilinearBlendKnownValues(t *testing.T) {
	// All same => should return same value
	got := bilinearBlend(100, 100, 100, 100, 0.5, 0.5)
	if got != 100 {
		t.Errorf("bilinearBlend(100,100,100,100,0.5,0.5) = %d, want 100", got)
	}

	// Top-left corner
	got = bilinearBlend(200, 0, 0, 0, 0.0, 0.0)
	if got != 200 {
		t.Errorf("bilinearBlend(200,0,0,0,0,0) = %d, want 200", got)
	}

	// Bottom-right corner
	got = bilinearBlend(0, 0, 0, 200, 1.0, 1.0)
	if got != 200 {
		t.Errorf("bilinearBlend(0,0,0,200,1,1) = %d, want 200", got)
	}

	// Midpoint between 0 and 200
	got = bilinearBlend(0, 200, 0, 200, 0.5, 0.5)
	if got != 100 {
		t.Errorf("bilinearBlend(0,200,0,200,0.5,0.5) = %d, want 100", got)
	}
}

// ===========================================================================
// 9. Search.Update and clampCursor
// ===========================================================================

func TestClampCursorNoResults(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = nil
	s.cursor = 5
	s.clampCursor()
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0 with no results", s.cursor)
	}
}

func TestClampCursorNegative(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks: []source.Track{{ID: "t1"}},
	}
	s.cursor = -5
	s.clampCursor()
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0 for negative", s.cursor)
	}
}

func TestClampCursorPastEnd(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks:  []source.Track{{ID: "t1"}, {ID: "t2"}},
		Artists: []source.SearchArtist{{ID: "a1"}},
	}
	s.cursor = 100
	s.clampCursor()
	// total = 2 tracks + 1 artist = 3, last valid = 2
	if s.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (total-1)", s.cursor)
	}
}

func TestSearchUpdateDown(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks: []source.Track{{ID: "t1"}, {ID: "t2"}},
	}
	s.cursor = 0

	msg := tea.KeyMsg{Type: tea.KeyDown}
	s, _ = s.Update(msg)
	if s.cursor != 1 {
		t.Errorf("cursor after Down = %d, want 1", s.cursor)
	}
}

func TestSearchUpdateUp(t *testing.T) {
	s := NewSearch(80, 40)
	s.results = &source.SearchResults{
		Tracks: []source.Track{{ID: "t1"}, {ID: "t2"}},
	}
	s.cursor = 1

	msg := tea.KeyMsg{Type: tea.KeyUp}
	s, _ = s.Update(msg)
	if s.cursor != 0 {
		t.Errorf("cursor after Up = %d, want 0", s.cursor)
	}
}

func TestSearchUpdateResultsMsg(t *testing.T) {
	s := NewSearch(80, 40)
	s.input.SetValue("test")
	s.cursor = 5

	results := &source.SearchResults{
		Tracks: []source.Track{{ID: "t1"}},
	}
	msg := searchResultsMsg{results: results, query: "test"}
	s, _ = s.Update(msg)

	if s.results == nil {
		t.Fatal("results should be set after searchResultsMsg")
	}
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after new results", s.cursor)
	}
}

func TestSearchUpdateResultsMsgMismatch(t *testing.T) {
	s := NewSearch(80, 40)
	s.input.SetValue("current")
	s.cursor = 2

	results := &source.SearchResults{
		Tracks: []source.Track{{ID: "t1"}},
	}
	msg := searchResultsMsg{results: results, query: "old"}
	s, _ = s.Update(msg)

	// Should not update results since query doesn't match
	if s.results != nil {
		t.Error("results should remain nil when query doesn't match input")
	}
	if s.cursor != 2 {
		t.Errorf("cursor = %d, should remain 2", s.cursor)
	}
}

// ===========================================================================
// 10. ShortHelp/FullHelp
// ===========================================================================

func TestShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.ShortHelp()
	if len(bindings) == 0 {
		t.Error("ShortHelp() should return non-empty slice")
	}
}

func TestFullHelp(t *testing.T) {
	km := DefaultKeyMap()
	groups := km.FullHelp()
	if len(groups) == 0 {
		t.Error("FullHelp() should return non-empty slice of slices")
	}
	for i, group := range groups {
		if len(group) == 0 {
			t.Errorf("FullHelp() group %d is empty", i)
		}
	}
}

// ===========================================================================
// 11. playPlaylistFromStart
// ===========================================================================

func TestPlayPlaylistFromStart(t *testing.T) {
	var calledContextURI, calledTrackURI string
	stub := &StubSource{
		PlayTrackFn: func(_ context.Context, contextURI, trackURI string) error {
			calledContextURI = contextURI
			calledTrackURI = trackURI
			return nil
		},
	}
	m := newTestModel(stub)

	cmd := m.playPlaylistFromStart("spotify:playlist:abc")
	if cmd == nil {
		t.Fatal("playPlaylistFromStart should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(controlDoneMsg); !ok {
		t.Errorf("expected controlDoneMsg, got %T", msg)
	}
	if calledContextURI != "spotify:playlist:abc" {
		t.Errorf("contextURI = %q, want %q", calledContextURI, "spotify:playlist:abc")
	}
	if calledTrackURI != "" {
		t.Errorf("trackURI = %q, want empty", calledTrackURI)
	}
}

func TestPlayPlaylistFromStartError(t *testing.T) {
	stub := &StubSource{
		PlayTrackFn: func(_ context.Context, contextURI, trackURI string) error {
			return errors.New("play error")
		},
	}
	m := newTestModel(stub)

	cmd := m.playPlaylistFromStart("spotify:playlist:abc")
	msg := cmd()
	if _, ok := msg.(trackErrorMsg); !ok {
		t.Errorf("expected trackErrorMsg, got %T", msg)
	}
}

// ===========================================================================
// 12. isAuthError
// ===========================================================================

func TestIsAuthErrorNonAuth(t *testing.T) {
	err := errors.New("network timeout")
	if isAuthError(err) {
		t.Error("isAuthError should return false for non-auth error")
	}
}

type mockHTTPStatusError struct {
	status int
}

func (e mockHTTPStatusError) Error() string   { return "http error" }
func (e mockHTTPStatusError) HTTPStatus() int { return e.status }

func TestIsAuthErrorHTTP401(t *testing.T) {
	err := mockHTTPStatusError{status: 401}
	if !isAuthError(err) {
		t.Error("isAuthError should return true for 401 status")
	}
}

func TestIsAuthErrorHTTP403(t *testing.T) {
	err := mockHTTPStatusError{status: 403}
	if isAuthError(err) {
		t.Error("isAuthError should return false for 403 status")
	}
}

// ===========================================================================
// 13. StatusBar.ViewNowPlayingWithArt nil track
// ===========================================================================

func TestStatusBarViewNowPlayingWithArtNilTrack(t *testing.T) {
	sb := NewStatusBar(120)
	art := PlaceholderArt(ArtWidth, ArtHeight)
	got := sb.ViewNowPlayingWithArt(nil, false, source.RepeatOff, false, art, 50, "", ModeNormal, "", "", "")
	if got == "" {
		t.Fatal("ViewNowPlayingWithArt with nil track should be non-empty")
	}
	if !strings.Contains(got, "No track playing") {
		t.Error("output should contain 'No track playing'")
	}
}

func TestStatusBarViewNowPlayingWithArtZeroWidth(t *testing.T) {
	sb := NewStatusBar(0)
	got := sb.ViewNowPlayingWithArt(nil, false, source.RepeatOff, false, "", 50, "", ModeNormal, "", "", "")
	if got != "" {
		t.Errorf("ViewNowPlayingWithArt with zero width = %q, want empty", got)
	}
}

// ===========================================================================
// 14. TrackList methods
// ===========================================================================

func TestTrackListSetFilterClearFilter(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Rock Song", Artist: "Rock Artist"},
		{ID: "t2", Name: "Pop Song", Artist: "Pop Artist"},
		{ID: "t3", Name: "Rock Ballad", Artist: "Ballad Artist"},
	}
	tl.SetTracks(tracks, "Test", "uri")

	tl.SetFilter("rock")
	if tl.FilterText() != "rock" {
		t.Errorf("FilterText() = %q, want %q", tl.FilterText(), "rock")
	}
	display := tl.displayTracks()
	if len(display) != 2 {
		t.Errorf("filtered tracks = %d, want 2", len(display))
	}

	tl.ClearFilter()
	if tl.FilterText() != "" {
		t.Errorf("FilterText() after clear = %q, want empty", tl.FilterText())
	}
	display = tl.displayTracks()
	if len(display) != 3 {
		t.Errorf("display tracks after clear = %d, want 3", len(display))
	}
}

func TestTrackListSetFilterEmpty(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
	}
	tl.SetTracks(tracks, "Test", "uri")

	tl.SetFilter("")
	display := tl.displayTracks()
	if len(display) != 1 {
		t.Errorf("display tracks = %d, want 1", len(display))
	}
}

func TestTrackListJumpToTrackFound(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
		{ID: "t2", Name: "Song B"},
		{ID: "t3", Name: "Song C"},
	}
	tl.SetTracks(tracks, "Test", "uri")

	found := tl.JumpToTrack("t2")
	if !found {
		t.Error("JumpToTrack should return true for existing track")
	}
	if tl.table.Cursor() != 1 {
		t.Errorf("cursor = %d, want 1", tl.table.Cursor())
	}
}

func TestTrackListJumpToTrackNotFound(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
	}
	tl.SetTracks(tracks, "Test", "uri")

	found := tl.JumpToTrack("nonexistent")
	if found {
		t.Error("JumpToTrack should return false for nonexistent track")
	}
}

func TestTrackListAppendTracks(t *testing.T) {
	tl := NewTrackList(80, 30)
	initial := []source.Track{
		{ID: "t1", Name: "Song A"},
	}
	tl.SetTracks(initial, "Test", "uri")

	more := []source.Track{
		{ID: "t2", Name: "Song B"},
		{ID: "t3", Name: "Song C"},
	}
	tl.AppendTracks(more)

	if len(tl.tracks) != 3 {
		t.Errorf("tracks count = %d, want 3", len(tl.tracks))
	}
}

func TestTrackListSetNowPlayingSameID(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A"},
	}
	tl.SetTracks(tracks, "Test", "uri")

	tl.SetNowPlaying("t1")
	if tl.nowPlaying != "t1" {
		t.Errorf("nowPlaying = %q, want %q", tl.nowPlaying, "t1")
	}

	// Setting same ID again should be a no-op (not crash or rebuild)
	tl.SetNowPlaying("t1")
	if tl.nowPlaying != "t1" {
		t.Errorf("nowPlaying after same = %q, want %q", tl.nowPlaying, "t1")
	}
}

func TestTrackListFilterExcludesSeparatorsAndAlbumRows(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Rock Song", Artist: "Artist"},
		{Name: "---", IsSeparator: true},
		{Name: "Rock Album", IsAlbumRow: true, AlbumID: "a1"},
		{ID: "t2", Name: "Another Rock", Artist: "Artist"},
	}
	tl.SetTracks(tracks, "Test", "uri")

	tl.SetFilter("rock")
	display := tl.displayTracks()
	// Should only include the 2 regular tracks, not separator or album row
	if len(display) != 2 {
		t.Errorf("filtered tracks = %d, want 2 (separators/album rows excluded)", len(display))
	}
}

// ===========================================================================
// Additional message handling: artistPageLoadedMsg and albumPageLoadedMsg
// ===========================================================================

func TestUpdateArtistPageLoaded(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t1", Name: "Current"}

	page := &source.ArtistPage{
		Name:   "Test Artist",
		Genres: []string{"Rock", "Pop"},
		Tracks: []source.Track{
			{ID: "at1", Name: "Artist Song 1"},
			{ID: "at2", Name: "Artist Song 2"},
		},
	}

	msg := artistPageLoadedMsg{page: page}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.focusPane != PaneTrackList {
		t.Errorf("focusPane = %v, want PaneTrackList", model.focusPane)
	}
	if len(model.tracklist.tracks) != 2 {
		t.Errorf("tracklist tracks = %d, want 2", len(model.tracklist.tracks))
	}
}

func TestUpdateAlbumPageLoaded(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t1", Name: "Current"}

	page := &source.AlbumPage{
		ID:     "alb1",
		Name:   "Test Album",
		Artist: "Artist",
		Year:   "2023",
		Tracks: []source.Track{
			{ID: "at1", Name: "Album Track 1"},
			{ID: "at2", Name: "Album Track 2"},
		},
	}

	msg := albumPageLoadedMsg{page: page}
	result, _ := m.Update(msg)
	model := result.(Model)

	if model.focusPane != PaneTrackList {
		t.Errorf("focusPane = %v, want PaneTrackList", model.focusPane)
	}
	if len(model.tracklist.tracks) != 2 {
		t.Errorf("tracklist tracks = %d, want 2", len(model.tracklist.tracks))
	}
	if model.tracklist.contextURI != "spotify:album:alb1" {
		t.Errorf("contextURI = %q, want %q", model.tracklist.contextURI, "spotify:album:alb1")
	}
}

// ===========================================================================
// StatusBar.ViewNowPlaying additional coverage
// ===========================================================================

func TestStatusBarViewNowPlayingZeroWidth(t *testing.T) {
	sb := NewStatusBar(0)
	got := sb.ViewNowPlaying(nil, false, source.RepeatOff, false)
	if got != "" {
		t.Errorf("ViewNowPlaying with zero width = %q, want empty", got)
	}
}

func TestStatusBarViewNowPlayingNilTrack(t *testing.T) {
	sb := NewStatusBar(80)
	got := sb.ViewNowPlaying(nil, false, source.RepeatOff, false)
	if !strings.Contains(got, "No track playing") {
		t.Error("should contain 'No track playing'")
	}
}

func TestStatusBarViewNowPlayingWithIndicators(t *testing.T) {
	sb := NewStatusBar(120)
	track := &source.Track{
		Name:     "Song",
		Artist:   "Artist",
		Album:    "Album",
		Duration: 3 * time.Minute,
		Position: 1 * time.Minute,
		Playing:  true,
	}
	got := sb.ViewNowPlaying(track, true, source.RepeatTrack, false)
	if got == "" {
		t.Fatal("ViewNowPlaying with indicators should be non-empty")
	}
}

func TestStatusBarViewModeLineZeroWidth(t *testing.T) {
	sb := NewStatusBar(0)
	got := sb.ViewModeLine(ModeNormal, "", "", "", 50, "")
	if got != "" {
		t.Errorf("ViewModeLine with zero width = %q, want empty", got)
	}
}

// ===========================================================================
// TrackList SelectedTrack and ContextURI
// ===========================================================================

func TestTrackListSelectedTrackEmpty(t *testing.T) {
	tl := NewTrackList(80, 30)
	if tl.SelectedTrack() != nil {
		t.Error("SelectedTrack on empty tracklist should be nil")
	}
}

func TestTrackListContextURI(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	tl.SetTracks(tracks, "Test", "spotify:playlist:abc")
	if tl.ContextURI() != "spotify:playlist:abc" {
		t.Errorf("ContextURI() = %q, want %q", tl.ContextURI(), "spotify:playlist:abc")
	}
}

// ===========================================================================
// TrackList SetLoading and Resize
// ===========================================================================

func TestTrackListSetLoading(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	tl.SetTracks(tracks, "Test", "uri")

	tl.SetLoading("Loading...")
	if !tl.loading {
		t.Error("loading should be true")
	}
	if tl.title != "Loading..." {
		t.Errorf("title = %q, want %q", tl.title, "Loading...")
	}
	if tl.tracks != nil {
		t.Error("tracks should be nil while loading")
	}
}

func TestTrackListResize(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	tl.SetTracks(tracks, "Test", "uri")

	tl.Resize(120, 50)
	if tl.width != 120 {
		t.Errorf("width = %d, want 120", tl.width)
	}
	if tl.height != 50 {
		t.Errorf("height = %d, want 50", tl.height)
	}
}

// ===========================================================================
// Sidebar View and Resize
// ===========================================================================

func TestSidebarView(t *testing.T) {
	sb := NewSidebar(30, 20)
	got := sb.View(true)
	if got == "" {
		t.Fatal("Sidebar.View(true) should be non-empty")
	}
	got = sb.View(false)
	if got == "" {
		t.Fatal("Sidebar.View(false) should be non-empty")
	}
}

func TestSidebarResize(t *testing.T) {
	sb := NewSidebar(30, 20)
	sb.Resize(40, 30)
	if sb.width != 40 {
		t.Errorf("width = %d, want 40", sb.width)
	}
	if sb.height != 30 {
		t.Errorf("height = %d, want 30", sb.height)
	}
}

// ===========================================================================
// Edge case: handleEnter with no selected track in tracklist
// ===========================================================================

func TestHandleEnterTracklistNoTrack(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneTrackList
	// No tracks set

	_, cmd := m.handleEnter()
	if cmd != nil {
		t.Error("handleEnter with no tracks should return nil cmd")
	}
}

func TestHandleEnterSidebarNoPlaylist(t *testing.T) {
	stub := &StubSource{}
	m := newTestModel(stub)
	m.focusPane = PaneSidebar
	// No playlists set

	_, cmd := m.handleEnter()
	if cmd != nil {
		t.Error("handleEnter with no sidebar playlist should return nil cmd")
	}
}

// ===========================================================================
// TrackList View
// ===========================================================================

func TestTrackListView(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{
		{ID: "t1", Name: "Song A", Artist: "Artist", Duration: 3 * time.Minute},
	}
	tl.SetTracks(tracks, "Test Playlist", "uri")

	got := tl.View(true)
	if got == "" {
		t.Fatal("TrackList.View(true) should be non-empty")
	}

	got = tl.View(false)
	if got == "" {
		t.Fatal("TrackList.View(false) should be non-empty")
	}
}

func TestTrackListViewLoading(t *testing.T) {
	tl := NewTrackList(80, 30)
	tl.SetLoading("Loading Playlist...")

	got := tl.View(true)
	if got == "" {
		t.Fatal("TrackList.View while loading should be non-empty")
	}
}

func TestTrackListViewWithSubtitle(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	tl.SetTracks(tracks, "Artist", "uri")
	tl.SetSubtitle("Rock, Pop")

	got := tl.View(true)
	if got == "" {
		t.Fatal("TrackList.View with subtitle should be non-empty")
	}
}

func TestTrackListViewWithHeaderInfo(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	tl.SetTracks(tracks, "Test", "uri")
	tl.SetHeaderInfo("3 tracks · 12m")

	got := tl.View(true)
	if got == "" {
		t.Fatal("TrackList.View with headerInfo should be non-empty")
	}
}

func TestTrackListViewWithArt(t *testing.T) {
	tl := NewTrackList(80, 30)
	tracks := []source.Track{{ID: "t1", Name: "Song"}}
	tl.SetTracks(tracks, "Test", "uri")
	tl.SetArt("FAKE_ART_BLOCK")

	got := tl.View(true)
	if got == "" {
		t.Fatal("TrackList.View with art should be non-empty")
	}
}
