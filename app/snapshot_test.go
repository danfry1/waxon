package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

func snapshotFixture() *librarySnapshot {
	return &librarySnapshot{
		Version: snapshotVersion,
		Playlists: []source.Playlist{
			{ID: likedPlaylistID, URI: "spotify:collection:tracks", Name: "♥ Liked Songs", TrackCount: 300},
			{ID: "mine", URI: "spotify:playlist:mine", Name: "My Mix"},
		},
		Tracks: []source.Track{{ID: "t1", Name: "Cached One"}, {ID: "t2", Name: "Cached Two"}},
		Total:  300,
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waxon", "library.json")
	if err := writeSnapshot(path, *snapshotFixture()); err != nil {
		t.Fatal(err)
	}
	got, err := readSnapshot(path)
	if err != nil || got == nil {
		t.Fatalf("readSnapshot = %v, %v", got, err)
	}
	if len(got.Playlists) != 2 || len(got.Tracks) != 2 || got.Total != 300 {
		t.Errorf("round trip lost data: %+v", got)
	}
	if runtime.GOOS != "windows" { // no Unix permission bits
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("snapshot should be 0600, got %v (%v)", info.Mode().Perm(), err)
		}
	}

	if err := RemoveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSnapshot(path); err != nil {
		t.Errorf("removing a missing snapshot should succeed, got %v", err)
	}
}

func TestSnapshotFromOtherVersionIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"playlists":[{"ID":"x"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readSnapshot(path); got != nil || err != nil {
		t.Errorf("readSnapshot = %v, %v; want nil, nil", got, err)
	}
}

func TestSnapshotFillsLibraryBeforeLiveLoad(t *testing.T) {
	m := newTestModel(&StubSource{})
	res, cmd := m.Update(snapshotLoadedMsg{snap: snapshotFixture()})
	m = res.(Model)
	runCmdTree(cmd)

	if len(m.playlists) != 2 {
		t.Fatalf("snapshot playlists not applied: %d", len(m.playlists))
	}
	if got := len(m.tracklist.tracks); got != 2 || m.tracklist.contextURI != "spotify:collection:tracks" {
		t.Errorf("track list should open on the snapshot's Liked Songs page, got %d tracks in %q", got, m.tracklist.contextURI)
	}
	if _, cached := m.trackCache[likedPlaylistID]; cached {
		t.Error("snapshot tracks must not enter the track cache, or the live fetch would be skipped")
	}
}

func TestSnapshotIgnoredAfterLiveLibrary(t *testing.T) {
	m := newTestModel(&StubSource{})
	live := []source.Playlist{{ID: likedPlaylistID, URI: "spotify:collection:tracks", Name: "♥ Liked Songs"}}
	res, _ := m.Update(playlistsLoadedMsg{playlists: live, initial: true})
	res, _ = res.(Model).Update(snapshotLoadedMsg{snap: snapshotFixture()})
	m = res.(Model)
	if len(m.playlists) != 1 || len(m.tracklist.tracks) != 0 {
		t.Errorf("a snapshot arriving after the live library must be dropped; playlists=%d tracks=%d",
			len(m.playlists), len(m.tracklist.tracks))
	}
}

func TestLiveLibraryRefreshesSnapshotPageOnlyIfStillShown(t *testing.T) {
	for _, tc := range []struct {
		name      string
		moveAway  bool
		wantFetch int32
	}{
		{"still on snapshot page", false, 1},
		{"user opened another playlist", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var likedFetches atomic.Int32
			stub := &StubSource{
				PlaylistTracksPageFn: func(_ context.Context, id string, _, _ int) ([]source.Track, int, error) {
					if id == likedPlaylistID {
						likedFetches.Add(1)
					}
					return nil, 0, nil
				},
			}
			m := newTestModel(stub)
			res, _ := m.Update(snapshotLoadedMsg{snap: snapshotFixture()})
			m = res.(Model)
			if tc.moveAway {
				m.tracklist.SetTracks([]source.Track{{ID: "x"}}, "My Mix", "spotify:playlist:mine")
			}
			_, cmd := m.Update(playlistsLoadedMsg{playlists: snapshotFixture().Playlists, initial: true})
			runCmdTree(cmd)
			if got := likedFetches.Load(); got != tc.wantFetch {
				t.Errorf("Liked Songs fetched %d times, want %d", got, tc.wantFetch)
			}
		})
	}
}

func TestLiveFirstPlaylistPageIsSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	m := newTestModel(&StubSource{}).WithOptions(Options{SnapshotPath: path})
	live := snapshotFixture().Playlists
	res, cmd := m.Update(playlistsLoadedMsg{playlists: live})
	runCmdTree(cmd)
	fresh := []source.Track{{ID: "new", Name: "Fresh"}}
	_, cmd = res.(Model).Update(tracksLoadedMsg{
		tracks: fresh, title: live[0].Name, contextURI: live[0].URI, playlistID: live[0].ID, total: 300,
	})
	runCmdTree(cmd)

	got, err := readSnapshot(path)
	if err != nil || got == nil {
		t.Fatalf("snapshot not written: %v, %v", got, err)
	}
	if len(got.Playlists) != 2 || len(got.Tracks) != 1 || got.Tracks[0].ID != "new" || got.Total != 300 {
		t.Errorf("saved snapshot = %+v", got)
	}
}

func TestNoSnapshotWithoutPath(t *testing.T) {
	m := newTestModel(&StubSource{})
	if m.loadSnapshot() != nil || m.saveSnapshot() != nil {
		t.Error("with no SnapshotPath, nothing should be read or written")
	}
}

func TestSnapshotBeforeFirstResizeStillShown(t *testing.T) {
	m := NewModel(&StubSource{}) // before the first WindowSizeMsg
	res, _ := m.Update(snapshotLoadedMsg{snap: snapshotFixture()})
	res, _ = res.(Model).Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = res.(Model)
	if len(m.tracklist.tracks) != 2 || !m.snapshotShown {
		t.Fatalf("snapshot page lost to the first resize: %d tracks, shown=%v", len(m.tracklist.tracks), m.snapshotShown)
	}
}

func TestSnapshotPageLoadsMoreLive(t *testing.T) {
	var offsets []int
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, _ string, offset, _ int) ([]source.Track, int, error) {
			offsets = append(offsets, offset)
			return nil, 300, nil
		},
	}
	m := newTestModel(stub)
	res, _ := m.Update(snapshotLoadedMsg{snap: snapshotFixture()})
	m = res.(Model)
	if m.pagination == nil || m.pagination.loaded != 2 || m.pagination.total != 300 {
		t.Fatalf("snapshot page should paginate past its 2 of 300 tracks, got %+v", m.pagination)
	}
	runCmdTree(m.fetchMoreTracks())
	if len(offsets) != 1 || offsets[0] != 2 {
		t.Errorf("load-more requested offsets %v, want [2]", offsets)
	}
}

func TestLiveReloadDoesNotReplaceViewOpenedMeanwhile(t *testing.T) {
	m := newTestModel(&StubSource{})
	res, _ := m.Update(snapshotLoadedMsg{snap: snapshotFixture()})
	live := snapshotFixture().Playlists
	res, _ = res.(Model).Update(playlistsLoadedMsg{playlists: live, initial: true})
	m = res.(Model)
	// The user opens another playlist while Liked Songs reloads.
	m.tracklist.SetLoading("My Mix")
	res, _ = m.Update(tracksLoadedMsg{
		tracks: []source.Track{{ID: "fresh"}}, title: live[0].Name, contextURI: live[0].URI, playlistID: live[0].ID,
	})
	m = res.(Model)
	if !m.tracklist.loading || m.tracklist.title != "My Mix" {
		t.Errorf("the reload replaced the playlist being opened (title %q)", m.tracklist.title)
	}
	if _, ok := m.trackCache[likedPlaylistID]; !ok {
		t.Error("a superseded reload should still be cached")
	}
}

func TestConcurrentSnapshotSavesKeepNewest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	m := newTestModel(&StubSource{}).WithOptions(Options{SnapshotPath: path})
	m.libraryLive = true
	m.playlists = snapshotFixture().Playlists
	m.snapTracks = []source.Track{{ID: "old"}}
	older := m.saveSnapshot()
	m.snapTracks = []source.Track{{ID: "new"}}
	newer := m.saveSnapshot()
	newer()
	older() // finishes last, but must not win
	got, err := readSnapshot(path)
	if err != nil || got == nil || got.Tracks[0].ID != "new" {
		t.Fatalf("snapshot = %+v, %v; want the newer save", got, err)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}
