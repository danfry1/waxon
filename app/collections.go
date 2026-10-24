package app

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// IDs of the library collections that aren't playlists. Each opens in the
// track list as navigable rows (albums, artists or shows).
const (
	collectionBrowseID   = "collection:browse"
	collectionRecentID   = "collection:recent"
	collectionAlbumsID   = "collection:albums"
	collectionArtistsID  = "collection:artists"
	collectionPodcastsID = "collection:podcasts"
)

// libraryRows are the rows waxon adds to the sidebar's Library after Liked
// Songs: one, which opens the collections (the playlists, used far more,
// stay near the top).
var libraryRows = []source.Playlist{
	{ID: collectionBrowseID, Name: "Albums · Artists · Podcasts"},
}

// collections are what the browse row lists, in order.
var collections = []source.Playlist{
	{ID: collectionRecentID, Name: "Recently Played"},
	{ID: collectionAlbumsID, Name: "Albums"},
	{ID: collectionArtistsID, Name: "Artists"},
	{ID: collectionPodcastsID, Name: "Podcasts"},
}

// libraryRowStyle is how a collection row looks in the sidebar: a tile the
// size of a playlist's cover, in its own colour (symbols are drawn at
// different widths by different terminals, and wouldn't line up), and a
// line under the name as playlists have their track count.
type libraryRowStyle struct {
	top, bottom color.RGBA // the tile's two halves
	desc        string
}

var libraryRowStyles = map[string]libraryRowStyle{
	collectionBrowseID:   {color.RGBA{0xa3, 0x7c, 0xf0, 0xff}, color.RGBA{0x4c, 0xa8, 0xf2, 0xff}, "recently played and more"},
	collectionRecentID:   {color.RGBA{0x2a, 0xc9, 0x6a, 0xff}, color.RGBA{0x14, 0x7a, 0x3f, 0xff}, "the tracks you played last"},
	collectionAlbumsID:   {color.RGBA{0xa3, 0x7c, 0xf0, 0xff}, color.RGBA{0x5b, 0x3d, 0xb0, 0xff}, "the albums you've saved"},
	collectionArtistsID:  {color.RGBA{0xf2, 0x9e, 0x4c, 0xff}, color.RGBA{0xb0, 0x5a, 0x1c, 0xff}, "the artists you follow"},
	collectionPodcastsID: {color.RGBA{0x4c, 0xa8, 0xf2, 0xff}, color.RGBA{0x1f, 0x62, 0xb0, 0xff}, "the shows you follow"},
}

// libraryRowTile renders the collection id's tile (half-blocks, as covers).
func libraryRowTile(id string) string {
	st := libraryRowStyles[id]
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for x := range 3 {
		img.SetRGBA(x, 0, st.top)
		img.SetRGBA(x, 1, st.bottom)
	}
	return renderHalfBlocks(img, 3, 1)
}

// isCollection reports whether id is one of the library collections rather
// than a playlist.
func isCollection(id string) bool {
	_, ok := libraryRowStyles[id]
	return ok
}

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

// openCollection shows recently played, saved albums, followed artists or
// podcasts in the track list: from their Library rows, or :recent /
// :albums / :artists / :podcasts.
func (m *Model) openCollection(id string) tea.Cmd {
	if id == collectionBrowseID {
		// The collections themselves, as rows that open them.
		rows := make([]source.Track, len(collections))
		for i, c := range collections {
			rows[i] = source.Track{ID: c.ID, Name: c.Name, Album: libraryRowStyles[c.ID].desc, IsAlbumRow: true}
		}
		m.pushNav()
		m.focusPane = PaneTrackList
		m.tracklist.SetTracks(rows, "Your Library", "")
		return nil
	}
	if id == collectionRecentID {
		m.pushNav()
		m.focusPane = PaneTrackList
		m.tracklist.SetLoading("Recently Played")
		return m.fetchRecentlyPlayed()
	}
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
	case isCollection(t.ID):
		return m.openCollection(t.ID), true
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
