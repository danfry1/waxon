package app

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func TestCollectionsStayOutOfSidebar(t *testing.T) {
	m := newTestModel(&StubSource{})
	res, _ := m.Update(playlistsLoadedMsg{playlists: []source.Playlist{{ID: likedPlaylistID, Name: "Liked"}, {ID: "p1", Name: "Mine"}}})
	m = res.(Model)
	if n := len(m.sidebar.list.Items()); n != 2 {
		t.Errorf("sidebar has %d items, want just the 2 playlists", n)
	}
}

func TestCollectionCommandsAndMotions(t *testing.T) {
	for _, keys := range [][]string{{"g", "a"}, {":", "a", "l", "b", "u", "m", "s", "enter"}} {
		m := newTestModel(&StubSource{SavedAlbumsFn: func(context.Context) ([]source.SavedAlbum, error) {
			return []source.SavedAlbum{{ID: "al1", Name: "Blue"}}, nil
		}})
		var cmd tea.Cmd
		for _, k := range keys {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
			if k == "enter" {
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			}
			var res tea.Model
			res, cmd = m.Update(msg)
			m = res.(Model)
		}
		if cmd == nil {
			t.Fatalf("%v: expected a fetch", keys)
		}
		res, _ := m.Update(cmd())
		m = res.(Model)
		if m.tracklist.title != "Saved Albums" || m.focusPane != PaneTrackList {
			t.Errorf("%v: title %q focus %v", keys, m.tracklist.title, m.focusPane)
		}
	}
}

func openCollection(t *testing.T, stub *StubSource, id string) Model {
	t.Helper()
	m := newTestModel(stub)
	cmd := m.openCollection(id)
	res, _ := m.Update(cmd())
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

func TestPodcastsOpenShowEpisodes(t *testing.T) {
	gotOffset := -1
	stub := &StubSource{
		SavedShowsFn: func(context.Context) ([]source.Show, error) {
			return []source.Show{{ID: "sh1", Name: "Sleeve Notes", Publisher: "Waxon Radio"}}, nil
		},
		ShowEpisodesPageFn: func(_ context.Context, id string, offset, _ int) ([]source.Track, int, error) {
			gotOffset = offset
			if id != "sh1" {
				t.Errorf("episodes for %q", id)
			}
			ep := source.Track{ID: "spotify:episode:e1", URI: "spotify:episode:e1", Name: "Ep 1", IsEpisode: true}
			return []source.Track{ep}, 120, nil
		},
	}
	m := openCollection(t, stub, collectionPodcastsID)
	m.focusPane = PaneTrackList
	m, cmd := m.handleEnter()
	res, _ := m.Update(cmd())
	m = res.(Model)
	if m.tracklist.ContextURI() != "spotify:show:sh1" || gotOffset != 0 {
		t.Fatalf("context %q offset %d", m.tracklist.ContextURI(), gotOffset)
	}
	if m.pagination == nil || m.pagination.playlistID != showKeyPrefix+"sh1" {
		t.Fatal("a long show should page in like a playlist")
	}
	m.pagination.loadingMore = false
	more := m.fetchMoreTracks()
	if _, ok := more().(moreTracksLoadedMsg); !ok || gotOffset != 1 {
		t.Errorf("next page should come from the show's episodes at offset 1, got %d", gotOffset)
	}
}

func TestEpisodeQueueAndLikeUseURI(t *testing.T) {
	var queued, saved string
	stub := &StubSource{
		AddToQueueFn:   func(_ context.Context, id string) error { queued = id; return nil },
		SaveTrackFn:    func(_ context.Context, id string) error { saved = id; return nil },
		IsTrackSavedFn: func(context.Context, string) (bool, error) { return false, nil },
	}
	m := newTestModel(stub)
	m.focusPane = PaneTrackList
	ep := source.Track{ID: "spotify:episode:e1", URI: "spotify:episode:e1", Name: "Ep", IsEpisode: true}
	m.tracklist.SetTracks([]source.Track{ep}, "Show", "spotify:show:sh1")
	m, cmd := m.handleAddQueue()
	runCmdTree(cmd)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	runCmdTree(cmd)
	if queued != "spotify:episode:e1" || saved != "spotify:episode:e1" {
		t.Errorf("queued %q saved %q, want the episode URI", queued, saved)
	}
}

func TestEpisodeSkipsLyricsLookup(t *testing.T) {
	called := false
	stub := &StubSource{LyricsFn: func(context.Context, source.Track) (*source.Lyrics, error) { called = true; return nil, nil }}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "spotify:episode:e1", IsEpisode: true}
	if cmd := m.ensureLyrics(); cmd != nil {
		cmd()
	}
	if called || m.lyricsLoading {
		t.Error("episodes have no lyrics to look up")
	}
}

func TestEpisodeLikeRefusalKeepsTrackChecks(t *testing.T) {
	m := newTestModel(&StubSource{})
	res, _ := m.Update(likeStatusErrorMsg{err: source.ErrForbidden, episode: true})
	if res.(Model).likeUnavailable {
		t.Error("an episode's refusal must not disable like checks for tracks")
	}
	res, _ = m.Update(likeStatusErrorMsg{err: source.ErrForbidden})
	if !res.(Model).likeUnavailable {
		t.Error("a track's refusal still disables them")
	}
}

func TestQueuedEpisodePlaysByURI(t *testing.T) {
	if got := queuedURI(source.Playlist{ID: "spotify:episode:e1", URI: "spotify:episode:e1"}); got != "spotify:episode:e1" {
		t.Errorf("queued episode plays %q", got)
	}
	if got := queuedURI(source.Playlist{ID: "t1"}); got != "spotify:track:t1" {
		t.Errorf("queued track plays %q", got)
	}
}
