package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

func cacheTestPlaylists() []source.Playlist {
	return []source.Playlist{
		{ID: likedPlaylistID, URI: "spotify:collection:tracks", Name: "♥ Liked Songs", TrackCount: 300},
		{ID: "mine", URI: "spotify:playlist:mine", Name: "My Mix", Snapshot: "v1"},
	}
}

// cacheModel is a model with the snapshot and page cache in a temp dir, and
// a source that counts live page fetches.
func cacheModel(t *testing.T, live []source.Track) (Model, *atomic.Int32, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.json")
	var fetches atomic.Int32
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, _ string, _, _ int) ([]source.Track, int, error) {
			fetches.Add(1)
			return live, len(live), nil
		},
	}
	return newTestModel(stub).WithOptions(Options{SnapshotPath: path}), &fetches, path
}

func writeTestPage(t *testing.T, m Model, id, version string, tracks ...string) {
	t.Helper()
	p := diskPage{Version: version, Total: len(tracks)}
	for _, tr := range tracks {
		p.Tracks = append(p.Tracks, source.Track{ID: tr, Name: tr})
	}
	if err := writePage(m.pageDir(), id, p); err != nil {
		t.Fatal(err)
	}
}

func shownIDs(m Model) []string {
	ids := make([]string, 0, len(m.tracklist.tracks))
	for _, tr := range m.tracklist.tracks {
		ids = append(ids, tr.ID)
	}
	return ids
}

func TestSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waxon", "library.json")
	if err := writeSnapshot(path, librarySnapshot{Playlists: cacheTestPlaylists(), Last: "mine", Cursor: 7}); err != nil {
		t.Fatal(err)
	}
	got, err := readSnapshot(path)
	if err != nil || got == nil || got.Last != "mine" || got.Cursor != 7 || len(got.Playlists) != 2 {
		t.Fatalf("readSnapshot = %+v, %v", got, err)
	}
	if runtime.GOOS != "windows" { // no Unix permission bits
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("snapshot should be 0600, got %v (%v)", info.Mode().Perm(), err)
		}
	}
}

func TestSnapshotFromOtherVersionIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"playlists":[{"ID":"x"}],"tracks":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readSnapshot(path); got != nil || err != nil {
		t.Errorf("readSnapshot = %v, %v; want nil, nil", got, err)
	}
}

// The point of the cache: the last playlist opens at once, from disk, and
// isn't fetched again when the live library says it hasn't changed.
func TestStartupOpensLastPlaylistWithoutFetching(t *testing.T) {
	m, fetches, _ := cacheModel(t, nil)
	writeTestPage(t, m, "mine", "s:v1", "a", "b", "c", "d", "e", "f")
	res, cmd := m.Update(snapshotLoadedMsg{snap: &librarySnapshot{Playlists: cacheTestPlaylists(), Last: "mine", Cursor: 4}})
	m = pumpModel(res.(Model), cmd)
	if got := shownIDs(m); len(got) != 6 || m.tracklist.contextURI != "spotify:playlist:mine" {
		t.Fatalf("startup view = %v in %q, want My Mix from disk", got, m.tracklist.contextURI)
	}
	if m.tracklist.Cursor() != 4 {
		t.Errorf("cursor = %d, want the saved 4", m.tracklist.Cursor())
	}
	res, cmd = m.Update(playlistsLoadedMsg{playlists: cacheTestPlaylists(), initial: true})
	m = pumpModel(res.(Model), cmd)
	if n := fetches.Load(); n != 0 {
		t.Errorf("an unchanged playlist was fetched %d times", n)
	}
}

func TestStartupRefreshesPlaylistThatChanged(t *testing.T) {
	m, fetches, _ := cacheModel(t, []source.Track{{ID: "new"}})
	writeTestPage(t, m, "mine", "s:v1", "old")
	res, cmd := m.Update(snapshotLoadedMsg{snap: &librarySnapshot{Playlists: cacheTestPlaylists(), Last: "mine"}})
	m = pumpModel(res.(Model), cmd)
	live := cacheTestPlaylists()
	live[1].Snapshot = "v2" // edited elsewhere since
	res, cmd = m.Update(playlistsLoadedMsg{playlists: live, initial: true})
	m = pumpModel(res.(Model), cmd)
	if fetches.Load() != 1 || len(m.tracklist.tracks) != 1 || m.tracklist.tracks[0].ID != "new" {
		t.Errorf("fetches=%d view=%v; want the changed playlist refreshed", fetches.Load(), shownIDs(m))
	}
	if p, _ := readPage(m.pageDir(), "mine"); p == nil || p.Version != "s:v2" {
		t.Errorf("the refreshed page should be cached with the new version, got %+v", p)
	}
}

func TestOpeningStalePageShowsItThenReplaces(t *testing.T) {
	m, fetches, _ := cacheModel(t, []source.Track{{ID: "live"}})
	writeTestPage(t, m, "mine", "s:old", "cached")
	pl := cacheTestPlaylists()[1]
	res, next := m.Update(m.fetchPlaylistTracks(pl)()) // the disk page arrives first
	m = res.(Model)
	if got := shownIDs(m); len(got) != 1 || got[0] != "cached" {
		t.Fatalf("the cached copy should show at once, got %v", got)
	}
	m = pumpModel(m, next)
	if fetches.Load() != 1 || shownIDs(m)[0] != "live" {
		t.Errorf("fetches=%d view=%v; want the live page to replace it", fetches.Load(), shownIDs(m))
	}
}

func TestRevalidationDoesNotReplaceViewOpenedMeanwhile(t *testing.T) {
	m, _, _ := cacheModel(t, []source.Track{{ID: "live"}})
	writeTestPage(t, m, "mine", "s:old", "cached")
	pl := cacheTestPlaylists()[1]
	res, next := m.Update(m.fetchPlaylistTracks(pl)())
	m = res.(Model)
	// The user opens something else before the live page arrives.
	m.tracklist.SetTracks([]source.Track{{ID: "other"}}, "Other", "spotify:playlist:other")
	m = pumpModel(m, next)
	if got := shownIDs(m); len(got) != 1 || got[0] != "other" {
		t.Errorf("the revalidation replaced the view the user opened: %v", got)
	}
	if p, _ := readPage(m.pageDir(), "mine"); p == nil || p.Tracks[0].ID != "live" {
		t.Error("its result should still be cached")
	}
}

func TestOpeningUncachedPlaylistFetchesAndCaches(t *testing.T) {
	m, fetches, _ := cacheModel(t, []source.Track{{ID: "live"}})
	m = pumpModel(m, m.fetchPlaylistTracks(cacheTestPlaylists()[1]))
	if fetches.Load() != 1 || len(m.tracklist.tracks) == 0 || shownIDs(m)[0] != "live" {
		t.Fatalf("fetches=%d view=%v", fetches.Load(), shownIDs(m))
	}
	if p, _ := readPage(m.pageDir(), "mine"); p == nil || p.Version != "s:v1" {
		t.Errorf("page not cached: %+v", p)
	}
	if m.lastPlaylistID != "mine" {
		t.Errorf("lastPlaylistID = %q", m.lastPlaylistID)
	}
}

// Liked Songs has no version, so its cached page shows at once but is
// always refreshed (a like and an unlike elsewhere leave its count alone).
func TestLikedSongsAlwaysRevalidates(t *testing.T) {
	if v := pageVersion(cacheTestPlaylists()[0]); v != "" {
		t.Errorf("Liked Songs version = %q, want none", v)
	}
	m, fetches, _ := cacheModel(t, []source.Track{{ID: "live"}})
	writeTestPage(t, m, likedPlaylistID, "", "cached")
	m = pumpModel(m, m.fetchPlaylistTracks(cacheTestPlaylists()[0]))
	if fetches.Load() != 1 || shownIDs(m)[0] != "live" {
		t.Errorf("fetches=%d view=%v; want the cached copy refreshed", fetches.Load(), shownIDs(m))
	}
}

func TestLikingForgetsLikedPage(t *testing.T) {
	m, _, _ := cacheModel(t, nil)
	writeTestPage(t, m, likedPlaylistID, "n:300", "a")
	m = pumpModel(m, func() tea.Msg { return trackLikedMsg{trackID: "z", liked: true} })
	if _, err := readPage(m.pageDir(), likedPlaylistID); err == nil {
		t.Error("liking a track should drop the cached Liked Songs page")
	}
}

func TestQuitSavesCursor(t *testing.T) {
	m, _, path := cacheModel(t, nil)
	m.libraryLive = true
	m.playlists = cacheTestPlaylists()
	m.lastPlaylistID = "mine"
	m.tracklist.SetTracks([]source.Track{{ID: "a"}, {ID: "b"}, {ID: "c"}}, "My Mix", "spotify:playlist:mine")
	m.tracklist.GotoRow(2)
	m.saveSnapshotOnQuit()
	got, err := readSnapshot(path)
	if err != nil || got == nil || got.Last != "mine" || got.Cursor != 2 {
		t.Errorf("snapshot after quit = %+v, %v", got, err)
	}
}

func TestSnapshotIgnoredAfterLiveLibrary(t *testing.T) {
	m, _, _ := cacheModel(t, nil)
	live := []source.Playlist{{ID: likedPlaylistID, URI: "spotify:collection:tracks", Name: "♥ Liked Songs"}}
	res, _ := m.Update(playlistsLoadedMsg{playlists: live, initial: true})
	res, _ = res.(Model).Update(snapshotLoadedMsg{snap: &librarySnapshot{Playlists: cacheTestPlaylists()}})
	if m = res.(Model); len(m.playlists) != 1 {
		t.Errorf("a snapshot arriving after the live library must be dropped; playlists=%d", len(m.playlists))
	}
}

func TestConcurrentSnapshotSavesKeepNewest(t *testing.T) {
	m, _, path := cacheModel(t, nil)
	m.libraryLive = true
	m.playlists = cacheTestPlaylists()
	m.lastPlaylistID = "old"
	older := m.saveSnapshot()
	m.lastPlaylistID = "mine"
	newer := m.saveSnapshot()
	newer()
	older() // finishes last, but must not win
	if got, _ := readSnapshot(path); got == nil || got.Last != "mine" {
		t.Fatalf("snapshot = %+v; want the newer save", got)
	}
}

func TestRemoveSnapshotForgetsPages(t *testing.T) {
	m, _, path := cacheModel(t, nil)
	m.libraryLive, m.playlists = true, cacheTestPlaylists()
	m.saveSnapshot()()
	writeTestPage(t, m, "mine", "s:v1", "a")
	if err := RemoveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.pageDir()); !os.IsNotExist(err) {
		t.Error("another account's cached pages must go with the snapshot")
	}
	if err := RemoveSnapshot(path); err != nil {
		t.Errorf("removing again should succeed, got %v", err)
	}
}

func TestPrunePagesKeepsNewest(t *testing.T) {
	m, _, _ := cacheModel(t, nil)
	for i := 0; i < maxDiskPages+5; i++ {
		writeTestPage(t, m, "p"+string(rune('a'+i%26))+string(rune('a'+i/26)), "s:1", "x")
	}
	prunePages(m.pageDir())
	entries, _ := os.ReadDir(m.pageDir())
	if len(entries) != maxDiskPages {
		t.Errorf("%d pages kept, want %d", len(entries), maxDiskPages)
	}
}

func TestNoSnapshotWithoutPath(t *testing.T) {
	m := newTestModel(&StubSource{})
	if m.loadSnapshot() != nil || m.saveSnapshot() != nil || m.pageDir() != "" {
		t.Error("with no SnapshotPath, nothing should be read or written")
	}
}

func TestFailedRefreshOfCachedPageIsQuiet(t *testing.T) {
	m, _, _ := cacheModel(t, nil)
	m.source.(*StubSource).PlaylistTracksPageFn = func(context.Context, string, int, int) ([]source.Track, int, error) {
		return nil, 0, errors.New("network is unreachable")
	}
	writeTestPage(t, m, "mine", "s:old", "cached")
	m = pumpModel(m, m.fetchPlaylistTracks(cacheTestPlaylists()[1]))
	if m.toast.Visible() {
		t.Errorf("a failed background refresh showed a toast: %q", m.toast.View(120))
	}
	if got := shownIDs(m); len(got) != 1 || got[0] != "cached" {
		t.Errorf("the cached copy should stay on screen, got %v", got)
	}
}

func TestResumeCursorOnlyForSavedPlaylist(t *testing.T) {
	m, _, _ := cacheModel(t, []source.Track{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}, {ID: "6"}})
	res, _ := m.Update(snapshotLoadedMsg{snap: &librarySnapshot{Playlists: cacheTestPlaylists(), Last: "mine", Cursor: 4}})
	m = res.(Model)
	// Before My Mix arrives, the user opens Liked Songs.
	m = pumpModel(m, m.livePlaylistPage(cacheTestPlaylists()[0], false))
	if m.tracklist.Cursor() != 0 {
		t.Errorf("the saved cursor was applied to another playlist: %d", m.tracklist.Cursor())
	}
}

func TestRevalidationDoesNotReplaceViewStillLoading(t *testing.T) {
	m, _, _ := cacheModel(t, []source.Track{{ID: "live"}})
	writeTestPage(t, m, "mine", "s:old", "cached")
	res, next := m.Update(m.fetchPlaylistTracks(cacheTestPlaylists()[1])())
	m = res.(Model)
	m.tracklist.SetLoading("An album") // another view is on its way
	m = pumpModel(m, next)
	if !m.tracklist.loading {
		t.Error("the refresh replaced a view that was loading")
	}
}

func TestUnchangedRefreshKeepsScrollAndLoadedPages(t *testing.T) {
	m, _, _ := cacheModel(t, []source.Track{{ID: "a"}, {ID: "b"}})
	writeTestPage(t, m, likedPlaylistID, "", "a", "b")
	res, next := m.Update(m.fetchPlaylistTracks(cacheTestPlaylists()[0])())
	m = res.(Model)
	m.tracklist.AppendTracks([]source.Track{{ID: "c"}, {ID: "d"}}) // scrolled in more
	m.tracklist.GotoRow(3)
	m = pumpModel(m, next)
	if len(m.tracklist.tracks) != 4 || m.tracklist.Cursor() != 3 {
		t.Errorf("an unchanged refresh reset the view: %d tracks, cursor %d", len(m.tracklist.tracks), m.tracklist.Cursor())
	}
}

func TestStaleMorePagesDroppedAfterRefresh(t *testing.T) {
	m, _, _ := cacheModel(t, nil)
	m.tracklist.SetTracks([]source.Track{{ID: "a"}}, "My Mix", "spotify:playlist:mine")
	old := &paginationState{playlistID: "mine", total: 10, loaded: 1}
	m.pagination = &paginationState{playlistID: "mine", total: 10, loaded: 1} // the refreshed list
	res, _ := m.Update(moreTracksLoadedMsg{tracks: []source.Track{{ID: "x"}}, playlistID: "mine", offset: 1, total: 10, pagination: old})
	if m = res.(Model); len(m.tracklist.tracks) != 1 {
		t.Errorf("a page fetched for the old list was appended: %v", shownIDs(m))
	}
}
