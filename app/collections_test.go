package app

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func TestWithCollectionsAfterLiked(t *testing.T) {
	got := withCollections([]source.Playlist{{ID: likedPlaylistID}, {ID: "p1"}})
	ids := []string{}
	for _, pl := range got {
		ids = append(ids, pl.ID)
	}
	want := fmt.Sprint([]string{likedPlaylistID, collectionAlbumsID, collectionArtistsID, "p1"})
	if fmt.Sprint(ids) != want {
		t.Errorf("order %v, want %s", ids, want)
	}
}

func openCollection(t *testing.T, stub *StubSource, id string) Model {
	t.Helper()
	m := newTestModel(stub)
	res, _ := m.Update(playlistsLoadedMsg{playlists: []source.Playlist{{ID: likedPlaylistID, Name: "Liked"}}})
	m = res.(Model)
	m.focusPane = PaneSidebar
	for i := 0; i < 3; i++ {
		if pl := m.sidebar.SelectedPlaylist(); pl != nil && pl.ID == id {
			break
		}
		m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("expected a fetch")
	}
	res, _ = m.Update(cmd())
	return res.(Model)
}

func TestSavedAlbumsOpenAlbumPage(t *testing.T) {
	var opened string
	stub := &StubSource{
		SavedAlbumsFn: func(context.Context) ([]source.SavedAlbum, error) {
			return []source.SavedAlbum{{ID: "al1", Name: "Blue", Artist: "Joni", Year: "1971"}}, nil
		},
		GetAlbumFn: func(_ context.Context, id string) (*source.AlbumPage, error) {
			opened = id
			return &source.AlbumPage{Name: "Blue"}, nil
		},
	}
	m := openCollection(t, stub, collectionAlbumsID)
	if m.tracklist.title != "Saved Albums" || m.tracklist.headerInfo != "1 album" {
		t.Fatalf("title %q info %q", m.tracklist.title, m.tracklist.headerInfo)
	}
	m.focusPane = PaneTrackList
	_, cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("Enter on an album should open it")
	}
	cmd()
	if opened != "al1" {
		t.Errorf("opened %q, want al1", opened)
	}
}

func TestFollowedArtistsOpenArtistPage(t *testing.T) {
	var opened string
	stub := &StubSource{
		FollowedArtistsFn: func(context.Context) ([]source.SearchArtist, error) {
			return []source.SearchArtist{{ID: "ar1", Name: "Joni Mitchell"}}, nil
		},
		GetArtistFn: func(_ context.Context, id string) (*source.ArtistPage, error) {
			opened = id
			return &source.ArtistPage{Name: "Joni"}, nil
		},
	}
	m := openCollection(t, stub, collectionArtistsID)
	m.focusPane = PaneTrackList
	_, cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("Enter on an artist should open it")
	}
	cmd()
	if opened != "ar1" {
		t.Errorf("opened %q, want ar1", opened)
	}
}

func TestFollowedArtistsMissingScopeExplains(t *testing.T) {
	stub := &StubSource{
		FollowedArtistsFn: func(context.Context) ([]source.SearchArtist, error) {
			return nil, fmt.Errorf("%w: 403", source.ErrInsufficientScope)
		},
	}
	m := openCollection(t, stub, collectionArtistsID)
	if !m.tracklist.HasNotice() {
		t.Error("a missing scope should show how to fix it, not an empty list")
	}
}

func TestFilterMatchesAlbumRows(t *testing.T) {
	tl := NewTrackList(80, 30)
	tl.SetTracks([]source.Track{
		{Name: "Blue", Album: "Joni Mitchell", AlbumID: "a", IsAlbumRow: true},
		{Name: "Hejira", Album: "Joni Mitchell", AlbumID: "b", IsAlbumRow: true},
		{Name: "Kid A", Album: "Radiohead", AlbumID: "c", IsAlbumRow: true},
	}, "Saved Albums", "")
	tl.SetFilter("joni")
	if n := len(tl.displayTracks()); n != 2 {
		t.Errorf("filter on album rows matched %d, want 2", n)
	}
}

func TestCollectionErrorShowsNotice(t *testing.T) {
	stub := &StubSource{SavedAlbumsFn: func(context.Context) ([]source.SavedAlbum, error) {
		return nil, fmt.Errorf("boom")
	}}
	m := openCollection(t, stub, collectionAlbumsID)
	if m.tracklist.loading || !m.tracklist.HasNotice() {
		t.Error("a failed load should replace the spinner with a notice")
	}
}

func TestCollectionPartialIsMarked(t *testing.T) {
	stub := &StubSource{SavedAlbumsFn: func(context.Context) ([]source.SavedAlbum, error) {
		return []source.SavedAlbum{{ID: "a", Name: "A"}}, fmt.Errorf("429")
	}}
	m := openCollection(t, stub, collectionAlbumsID)
	if got := m.tracklist.headerInfo; got != "1 album (incomplete: Spotify stopped responding)" {
		t.Errorf("header %q", got)
	}
}
