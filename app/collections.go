package app

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// IDs of the library collections that aren't playlists. Each opens in the
// track list as navigable rows (albums, artists or shows).
const (
	collectionAlbumsID   = "collection:albums"
	collectionArtistsID  = "collection:artists"
	collectionPodcastsID = "collection:podcasts"
)

// showKeyPrefix keys a show's episode list in the track cache and pagination
// state, alongside playlist IDs: "show:<id>". fetchMoreTracks routes on it.
const showKeyPrefix = "show:"

// showPageSize is Spotify's maximum page for a show's episodes.
const showPageSize = 50

// collectionTitles names each collection view.
var collectionTitles = map[string]string{
	collectionAlbumsID:   "Saved Albums",
	collectionArtistsID:  "Followed Artists",
	collectionPodcastsID: "Podcasts",
}

// openCollection shows saved albums, followed artists or podcasts in the
// track list. They're reached by ga / gA / gp and :albums / :artists /
// :podcasts rather than sitting in the sidebar, which stays playlists.
func (m *Model) openCollection(id string) tea.Cmd {
	title := collectionTitles[id]
	m.pushNav()
	m.focusPane = PaneTrackList
	m.tracklist.SetLoading(title)
	return m.fetchCollection(id, title)
}

// collectionLoadedMsg carries a saved-albums or followed-artists list as
// navigable rows. notice is set instead of rows when the list can't be shown.
type collectionLoadedMsg struct {
	title  string
	rows   []source.Track
	info   string
	notice string
	err    error // set when the list is missing or incomplete
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
			info = countLabel(len(rows), "album", "albums")
		case collectionArtistsID:
			var artists []source.SearchArtist
			artists, err = src.FollowedArtists(ctx)
			for _, a := range artists {
				rows = append(rows, source.Track{Name: a.Name, ArtistID: a.ID, IsAlbumRow: true})
			}
			info = countLabel(len(rows), "artist", "artists")
		case collectionPodcastsID:
			var shows []source.Show
			shows, err = src.SavedShows(ctx)
			for _, s := range shows {
				rows = append(rows, source.Track{Name: s.Name, Album: s.Publisher, ShowID: s.ID, IsAlbumRow: true})
			}
			info = countLabel(len(rows), "podcast", "podcasts")
		}
		msg := collectionLoadedMsg{title: title, rows: rows, info: info, err: err}
		switch {
		case errors.Is(err, source.ErrInsufficientScope):
			msg.rows, msg.info, msg.err = nil, "", nil
			msg.notice = "waxon needs one more permission to list these.\n\nRun 'waxon auth' once to grant it."
		case err != nil && len(rows) == 0:
			// Show why in place of the list rather than leaving a spinner.
			msg.notice = "Couldn't load this from Spotify.\n\nGo back with b and try again in a moment."
		case err != nil:
			msg.info += " (incomplete: Spotify stopped responding)"
		case len(rows) == 0:
			msg.notice = "Nothing here yet."
		}
		return msg
	}
}

// countLabel is "N albums", or "N+ albums" when the fetch hit its cap.
func countLabel(n int, one, many string) string {
	if n >= maxCollectionItems {
		return fmt.Sprintf("%d+ %s", n, many)
	}
	return fmt.Sprintf("%d %s", n, pluralize(n, one, many))
}

// maxCollectionItems mirrors the Spotify client's cap on collection lists.
const maxCollectionItems = 2000

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
	case t.ShowID != "":
		m.pushNav()
		m.tracklist.SetLoading(t.Name)
		return m.fetchShowEpisodes(t.ShowID, t.Name), true
	}
	return nil, false
}

// fetchShowEpisodes loads a show's newest episodes as a playable list in the
// show's context; older ones page in like a long playlist.
func (m Model) fetchShowEpisodes(showID, title string) tea.Cmd {
	src, ctx := m.source, m.ctx
	return func() tea.Msg {
		eps, total, err := src.ShowEpisodesPage(ctx, showID, 0, showPageSize)
		if err != nil {
			return trackErrorMsg{err}
		}
		for i := range eps {
			if eps[i].IsEpisode && eps[i].Artist == "" {
				eps[i].Artist = title // so actions and Now Playing name the show
			}
		}
		return tracksLoadedMsg{
			tracks:     eps,
			title:      title,
			contextURI: "spotify:show:" + showID,
			playlistID: showKeyPrefix + showID,
			total:      total,
		}
	}
}
