package app

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// Sentinel sidebar IDs for the library sections that aren't playlists. They
// sit under Liked Songs and open as a list of albums or artists.
const (
	collectionAlbumsID  = "collection:albums"
	collectionArtistsID = "collection:artists"
)

func isCollection(id string) bool {
	return id == collectionAlbumsID || id == collectionArtistsID
}

// withCollections returns the sidebar's library list: the playlists with the
// saved-albums and followed-artists entries after Liked Songs.
func withCollections(pls []source.Playlist) []source.Playlist {
	entries := []source.Playlist{
		{ID: collectionAlbumsID, Name: "Saved Albums"},
		{ID: collectionArtistsID, Name: "Followed Artists"},
	}
	// Every source lists Liked Songs first (its ID differs between them).
	at := min(1, len(pls))
	out := make([]source.Playlist, 0, len(pls)+len(entries))
	out = append(out, pls[:at]...)
	out = append(out, entries...)
	return append(out, pls[at:]...)
}

// collectionLoadedMsg carries a saved-albums or followed-artists list as
// navigable rows. notice is set instead of rows when the list can't be shown.
type collectionLoadedMsg struct {
	title  string
	rows   []source.Track
	info   string
	notice string
}

// fetchCollection loads one of the collection sections.
func (m Model) fetchCollection(id, title string) tea.Cmd {
	src, ctx := m.source, m.ctx
	return func() tea.Msg {
		var rows []source.Track
		var info string
		var err error
		switch id {
		case collectionAlbumsID:
			var albums []source.SavedAlbum
			albums, err = src.SavedAlbums(ctx)
			for _, a := range albums {
				rows = append(rows, source.Track{Name: a.Name, Album: a.Artist, Artist: a.Year, AlbumID: a.ID, IsAlbumRow: true})
			}
			info = fmt.Sprintf("%d %s", len(rows), pluralize(len(rows), "album", "albums"))
		case collectionArtistsID:
			var artists []source.SearchArtist
			artists, err = src.FollowedArtists(ctx)
			for _, a := range artists {
				rows = append(rows, source.Track{Name: a.Name, ArtistID: a.ID, IsAlbumRow: true})
			}
			info = fmt.Sprintf("%d %s", len(rows), pluralize(len(rows), "artist", "artists"))
		}
		msg := collectionLoadedMsg{title: title, rows: rows, info: info}
		switch {
		case errors.Is(err, source.ErrInsufficientScope):
			msg.rows, msg.info = nil, ""
			msg.notice = "waxon needs one more permission to list these.\n\nRun 'waxon auth' once to grant it."
		case err != nil && len(rows) == 0:
			return trackErrorMsg{err}
		case len(rows) == 0:
			msg.notice = "Nothing here yet."
		}
		return msg
	}
}

// openRow navigates into a row that stands for a page rather than a track:
// a discography or saved album, or a followed artist.
func (m *Model) openRow(t source.Track) (tea.Cmd, bool) {
	if !t.IsAlbumRow {
		return nil, false
	}
	switch {
	case t.AlbumID != "":
		m.pushNav()
		m.tracklist.SetLoading(t.Name)
		return m.fetchAlbumPage(t.AlbumID), true
	case t.ArtistID != "":
		m.pushNav()
		m.tracklist.SetLoading(t.Name)
		return m.fetchArtistPage(t.ArtistID), true
	}
	return nil, false
}
