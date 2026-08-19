package app

import (
	"context"
	"fmt"
	"image"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

const playlistPageSize = 100 // tracks per Spotify API page

// --- Fetch Commands ---

func (m Model) fetchCurrentTrack() tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		ps, err := src.CurrentPlayback(ctx)
		if err != nil {
			return pollErrorMsg{err}
		}
		if ps == nil {
			return noPlaybackMsg{}
		}
		return trackUpdateMsg{
			track:      ps.Track,
			volume:     ps.Volume,
			shuffleOn:  ps.ShuffleOn,
			repeatMode: ps.RepeatMode,
			contextURI: ps.ContextURI,
		}
	}
}

func (m Model) fetchPlaylists() tea.Cmd {
	return m.loadPlaylists(true)
}

// refreshPlaylists reloads the library list (names, counts, new playlists)
// without disturbing the current tracklist — used after playlist edits.
func (m Model) refreshPlaylists() tea.Cmd {
	return m.loadPlaylists(false)
}

func (m Model) loadPlaylists(initial bool) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		playlists, err := src.Playlists(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistsLoadedMsg{playlists: playlists, initial: initial}
	}
}

func (m Model) fetchPlaylistTracks(pl source.Playlist) tea.Cmd {
	// Return cached tracks if fresh
	if cached, ok := m.trackCache[pl.ID]; ok && time.Since(cached.fetchedAt) < trackCacheTTL {
		return func() tea.Msg {
			return tracksLoadedMsg{
				tracks:     cached.tracks,
				title:      pl.Name,
				contextURI: pl.URI,
				imageURL:   pl.ImageURL,
				playlistID: pl.ID,
				total:      cached.total,
			}
		}
	}
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		tracks, total, err := src.PlaylistTracksPage(ctx, pl.ID, 0, playlistPageSize)
		if err != nil {
			return trackErrorMsg{err}
		}
		return tracksLoadedMsg{
			tracks:     tracks,
			title:      pl.Name,
			contextURI: pl.URI,
			imageURL:   pl.ImageURL,
			playlistID: pl.ID,
			total:      total,
		}
	}
}

func (m Model) fetchMoreTracks() tea.Cmd {
	if m.pagination == nil {
		return nil
	}
	p := m.pagination
	src := m.source
	ctx := m.ctx
	id := p.playlistID
	offset := p.loaded
	return func() tea.Msg {
		tracks, _, err := src.PlaylistTracksPage(ctx, id, offset, playlistPageSize)
		if err != nil {
			return trackErrorMsg{err}
		}
		return moreTracksLoadedMsg{tracks: tracks, playlistID: id}
	}
}

// fetchLyrics loads lyrics for the given track off the main loop. A nil result
// (no lyrics found) is reported as a successful lyricsLoadedMsg with nil lyrics,
// not an error, so the UI can show an empty state.
func (m Model) fetchLyrics(track source.Track) tea.Cmd {
	ctx := m.ctx
	src := m.source
	return func() tea.Msg {
		lyr, err := src.Lyrics(ctx, track)
		if err != nil {
			return lyricsErrorMsg{trackID: track.ID, err: err}
		}
		return lyricsLoadedMsg{trackID: track.ID, lyrics: lyr}
	}
}

func (m Model) fetchNPArt(url string) tea.Cmd {
	ctx := m.ctx
	ap := m.artworkProvider
	return func() tea.Msg {
		var img image.Image
		var err error
		if ap != nil {
			if i, ok := ap.ArtworkImage(url); ok {
				img = i
			}
		}
		if img == nil {
			img, err = FetchImage(ctx, url)
			if err != nil {
				return trackErrorMsg{err}
			}
		}
		return npArtLoadedMsg{url: url, img: img}
	}
}

func (m Model) fetchQueue() tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		tracks, err := src.Queue(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return queueLoadedMsg{tracks}
	}
}

func (m Model) fetchRecentlyPlayed() tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		tracks, err := src.RecentlyPlayed(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return recentTracksLoadedMsg{tracks}
	}
}

func (m Model) fetchDevices() tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		devices, err := src.Devices(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return devicesLoadedMsg{devices}
	}
}

func (m Model) toggleLike(trackID string, currentlyLiked bool) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		var err error
		if currentlyLiked {
			err = src.RemoveTrack(ctx, trackID)
		} else {
			err = src.SaveTrack(ctx, trackID)
		}
		if err != nil {
			return trackErrorMsg{err}
		}
		return trackLikedMsg{trackID: trackID, liked: !currentlyLiked}
	}
}

// toggleLikeByLookup toggles the saved state of a track whose current state is
// unknown: it asks Spotify first, then saves or removes accordingly. Used for
// tracks other than the one playing (whose state is tracked in m.liked).
func (m Model) toggleLikeByLookup(trackID string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		saved, err := src.IsTrackSaved(ctx, trackID)
		if err != nil {
			return trackErrorMsg{err}
		}
		if saved {
			err = src.RemoveTrack(ctx, trackID)
		} else {
			err = src.SaveTrack(ctx, trackID)
		}
		if err != nil {
			return trackErrorMsg{err}
		}
		return trackLikedMsg{trackID: trackID, liked: !saved}
	}
}

// likeToggleCmd picks the right toggle for a track: the playing track's state
// is known (m.liked), anything else is looked up first so "f" on a track that
// is already saved removes it instead of re-saving.
func (m Model) likeToggleCmd(trackID string) tea.Cmd {
	if liked, known := m.likedStatus(trackID); known {
		return m.toggleLike(trackID, liked)
	}
	return m.toggleLikeByLookup(trackID)
}

// currentPlaylistContext describes the playlist the tracklist is showing, if
// it is one of the user's playlists, so actions can offer "remove from".
func (m Model) currentPlaylistContext() TrackActionContext {
	const prefix = "spotify:playlist:"
	uri := m.tracklist.ContextURI()
	if !strings.HasPrefix(uri, prefix) {
		return TrackActionContext{}
	}
	id := strings.TrimPrefix(uri, prefix)
	for _, pl := range m.playlists {
		if pl.ID == id {
			return TrackActionContext{PlaylistID: pl.ID, PlaylistName: pl.Name, Editable: pl.Editable}
		}
	}
	return TrackActionContext{}
}

// openPlaylistPicker shows the "Add to playlist" chooser for a track.
func (m Model) openPlaylistPicker(trackID, trackName string) (Model, tea.Cmd) {
	picker := NewPlaylistPicker(m.playlists, trackID, trackName, m.width, m.height)
	m.playlistPick = &picker
	m.mode = ModePlaylistPick
	return m, textinput.Blink
}

// addToPlaylist appends a track to a playlist.
func (m Model) addToPlaylist(pl source.Playlist, trackID, trackName string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		if err := src.AddToPlaylist(ctx, pl.ID, trackID); err != nil {
			return trackErrorMsg{err}
		}
		return playlistChangedMsg{playlistID: pl.ID, playlistName: pl.Name, trackName: trackName, added: true}
	}
}

// removeFromPlaylist removes a track from a playlist.
func (m Model) removeFromPlaylist(playlistID, trackID, trackName string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	name := playlistID
	for _, pl := range m.playlists {
		if pl.ID == playlistID {
			name = pl.Name
		}
	}
	return func() tea.Msg {
		if err := src.RemoveFromPlaylist(ctx, playlistID, trackID); err != nil {
			return trackErrorMsg{err}
		}
		return playlistChangedMsg{playlistID: playlistID, playlistName: name, trackName: trackName, added: false}
	}
}

// createPlaylist creates an empty playlist and reloads the library.
func (m Model) createPlaylist(name string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		pl, err := src.CreatePlaylist(ctx, name)
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistCreatedMsg{playlist: pl}
	}
}

// likedPlaylistID is the sentinel playlist ID the sidebar uses for Liked Songs
// (mirrors spotify.LikedPlaylistID).
const likedPlaylistID = "liked"

// noteLikedTracks records that every track in a Liked Songs page is saved, so
// like/unlike on those rows doesn't need a lookup.
func (m *Model) noteLikedTracks(playlistID string, tracks []source.Track) {
	if playlistID != likedPlaylistID {
		return
	}
	for _, t := range tracks {
		if t.ID != "" {
			m.likedCache[t.ID] = true
		}
	}
}

// likedStatus reports whether trackID is saved, and whether that is actually
// known (playing track, or a status we've fetched/changed this session).
func (m Model) likedStatus(trackID string) (liked, known bool) {
	if m.track != nil && m.track.ID == trackID {
		return m.liked, true
	}
	liked, known = m.likedCache[trackID]
	return liked, known
}

func (m Model) checkLikeStatus(trackID string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		saved, err := src.IsTrackSaved(ctx, trackID)
		if err != nil {
			return trackErrorMsg{err}
		}
		return trackLikeStatusMsg{trackID: trackID, liked: saved}
	}
}

func (m Model) doSearch(query string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		results, err := src.Search(ctx, query)
		if err != nil {
			return trackErrorMsg{err}
		}
		return searchResultsMsg{results: results, query: query}
	}
}

// fetchSidebarIcons returns one command per playlist cover so icons stream
// into the sidebar as each arrives, instead of all at once after the slowest
// download. FetchImage bounds concurrency and serves repeats from cache, so
// a large library doesn't flood the network.
func (m Model) fetchSidebarIcons(playlists []source.Playlist) tea.Cmd {
	ctx := m.ctx
	ap := m.artworkProvider
	cmds := make([]tea.Cmd, 0, len(playlists))
	for _, pl := range playlists {
		if pl.ImageURL == "" {
			continue
		}
		id, url := pl.ID, pl.ImageURL
		cmds = append(cmds, func() tea.Msg {
			var img image.Image
			if ap != nil {
				if i, ok := ap.ArtworkImage(url); ok {
					img = i
				}
			}
			if img == nil {
				var err error
				img, err = FetchImage(ctx, url)
				if err != nil {
					return nil // icon stays as the ♫ fallback
				}
			}
			return sidebarIconLoadedMsg{id: id, icon: renderHalfBlocks(img, 3, 1)}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) fetchArtwork(url string) tea.Cmd {
	ctx := m.ctx
	ap := m.artworkProvider
	return func() tea.Msg {
		var img image.Image
		var err error
		if ap != nil {
			if i, ok := ap.ArtworkImage(url); ok {
				img = i
			}
		}
		if img == nil {
			img, err = FetchImage(ctx, url)
			if err != nil {
				return artErrorMsg{url: url, err: err}
			}
		}
		return artworkLoadedMsg{url: url, img: img}
	}
}

func (m Model) fetchPlaylistArt(url string) tea.Cmd {
	ctx := m.ctx
	ap := m.artworkProvider
	return func() tea.Msg {
		var img image.Image
		var err error
		if ap != nil {
			if i, ok := ap.ArtworkImage(url); ok {
				img = i
			}
		}
		if img == nil {
			img, err = FetchImage(ctx, url)
			if err != nil {
				return artErrorMsg{url: url, err: err}
			}
		}
		return playlistArtLoadedMsg{url: url, img: img}
	}
}

func (m Model) fetchArtistPage(artistID string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		page, err := src.GetArtist(ctx, artistID)
		if err != nil {
			return trackErrorMsg{err}
		}
		return artistPageLoadedMsg{page}
	}
}

func (m Model) fetchAlbumPage(albumID string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		page, err := src.GetAlbum(ctx, albumID)
		if err != nil {
			return trackErrorMsg{err}
		}
		return albumPageLoadedMsg{page}
	}
}

// --- Playback Control Commands ---

func (m Model) playTrack(trackURI, contextURI string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return deviceAware(func() tea.Msg {
		var err error
		if contextURI != "" {
			err = src.PlayTrack(ctx, contextURI, trackURI)
		} else if trackURI != "" {
			err = src.PlayTrackDirect(ctx, trackURI)
		} else {
			err = src.Play(ctx)
		}
		if err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	})
}

func (m Model) controlCmd(fn func(context.Context) error) tea.Cmd {
	ctx := m.ctx
	return deviceAware(func() tea.Msg {
		if err := fn(ctx); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	})
}

func (m Model) seekRelative(delta time.Duration) tea.Cmd {
	if m.track == nil {
		return nil
	}
	pos := m.track.Position + delta
	if pos < 0 {
		pos = 0
	}
	src := m.source
	ctx := m.ctx
	return deviceAware(func() tea.Msg {
		if err := src.Seek(ctx, pos); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	})
}

func (m Model) transferPlayback(deviceID, deviceName string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		if err := src.TransferPlayback(ctx, deviceID); err != nil {
			return trackErrorMsg{err}
		}
		return cmdFlashMsg{"Playback → " + deviceName}
	}
}

func (m Model) playPlaylistFromStart(contextURI string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return deviceAware(func() tea.Msg {
		if err := src.PlayTrack(ctx, contextURI, ""); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	})
}

// --- Command/Action Execution ---

func (m *Model) executeCommand(input string) tea.Cmd {
	cmd, err := ParseCommand(input)
	if err != nil {
		m.toast.Show(err.Error(), "", ToastError)
		return scheduleAutoDismiss()
	}

	src := m.source
	ctx := m.ctx
	switch cmd.Type {
	case CmdQuit:
		m.cancel()
		m.quitting = true
		return tea.Quit
	case CmdVolume:
		m.volume = cmd.IntArg
		vol := cmd.IntArg
		return deviceAware(func() tea.Msg {
			if err := src.SetVolume(ctx, vol); err != nil {
				return trackErrorMsg{err}
			}
			return cmdFlashMsg{fmt.Sprintf("Volume: %d%%", vol)}
		})
	case CmdShuffle:
		m.shuffleOn = !m.shuffleOn
		state := m.shuffleOn
		label := "Shuffle on"
		if !state {
			label = "Shuffle off"
		}
		return deviceAware(func() tea.Msg {
			if err := src.SetShuffle(ctx, state); err != nil {
				return trackErrorMsg{err}
			}
			return cmdFlashMsg{label}
		})
	case CmdRepeat:
		m.repeatMode = source.RepeatMode(cmd.StrArg)
		mode := source.RepeatMode(cmd.StrArg)
		return deviceAware(func() tea.Msg {
			if err := src.SetRepeat(ctx, mode); err != nil {
				return trackErrorMsg{err}
			}
			return cmdFlashMsg{"Repeat: " + cmd.StrArg}
		})
	case CmdDevice:
		return m.fetchDevices()
	case CmdRecent:
		m.pushNav()
		return m.fetchRecentlyPlayed()
	case CmdTheme:
		return m.setTheme(cmd.StrArg)
	case CmdNewPlaylist:
		return m.createPlaylist(cmd.StrArg)
	case CmdSearch:
		s := NewSearch(m.width, m.height)
		m.search = &s
		m.mode = ModeSearch
		if cmd.StrArg != "" {
			m.search.input.SetValue(cmd.StrArg)
			return m.doSearch(cmd.StrArg)
		}
		return nil
	}
	return nil
}

func (m Model) openActions() (Model, tea.Cmd) {
	if m.focusPane == PaneTrackList {
		track := m.tracklist.SelectedTrack()
		if track == nil || track.IsSeparator {
			m.toast.Show("No track selected", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		// For album rows in discography, navigate directly to the album
		if track.IsAlbumRow && track.AlbumID != "" {
			m.pushNav()
			m.tracklist.SetLoading(track.Name)
			return m, m.fetchAlbumPage(track.AlbumID)
		}
		liked, known := m.likedStatus(track.ID)
		popup := NewTrackActionsIn(track.Name, track.Artist, track.URI, m.tracklist.ContextURI(), track.ArtistID, track.AlbumID, liked, m.currentPlaylistContext(), m.width, m.height)
		m.actions = &popup
		m.actionsReturn = ModeNormal
		m.mode = ModeActions
		if !known && track.ID != "" {
			// Look the state up so the popup's label is right; the action itself
			// re-checks, so this is purely cosmetic.
			return m, m.checkLikeStatus(track.ID)
		}
		return m, nil
	}

	// Sidebar pane
	pl := m.sidebar.SelectedPlaylist()
	if pl == nil {
		m.toast.Show("No playlist selected", "", ToastError)
		return m, scheduleAutoDismiss()
	}
	popup := NewPlaylistActions(pl.Name, pl.URI, m.width, m.height)
	m.actions = &popup
	m.actionsReturn = ModeNormal
	m.mode = ModeActions
	return m, nil
}

// actionSubject is the track an actions popup operates on. It is captured when
// the popup opens so actions target that specific track — not whatever the
// cursor happens to be over when the action runs (which lets the Now Playing
// view act on the currently playing track).
type actionSubject struct {
	trackID    string
	name       string
	uri        string
	contextURI string
	artistID   string
	albumID    string
	playlistID string // playlist the track is listed in (for remove), may be empty
}

func (m Model) executeAction(action ActionItem, subj actionSubject) (Model, tea.Cmd) {
	// m.mode is already set to the popup's return mode by the caller; the
	// navigation actions below override it with ModeNormal so they land on the
	// loaded tracklist rather than re-opening the view the popup came from.
	switch action.Type {
	case ActionPlay:
		if subj.uri != "" {
			return m, m.playTrack(subj.uri, subj.contextURI)
		}
	case ActionQueue:
		if subj.trackID == "" {
			m.toast.Show("No track to queue", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		return m, m.queueTrack(subj.trackID, subj.name)
	case ActionLike:
		if subj.trackID == "" {
			m.toast.Show("No track selected", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		return m, m.likeToggleCmd(subj.trackID)
	case ActionGoArtist:
		if subj.artistID == "" {
			m.toast.Show("No artist info available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		m.mode = ModeNormal
		m.pushNav()
		m.tracklist.SetLoading("Loading artist...")
		return m, m.fetchArtistPage(subj.artistID)
	case ActionGoAlbum:
		if subj.albumID == "" {
			m.toast.Show("No album info available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		m.mode = ModeNormal
		m.pushNav()
		m.tracklist.SetLoading("Loading album...")
		return m, m.fetchAlbumPage(subj.albumID)
	case ActionOpenSpotify:
		if subj.uri == "" {
			m.toast.Show("No URI available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		if err := openInSpotify(subj.uri); err != nil {
			m.toast.Show("Failed to open Spotify", err.Error(), ToastError)
			return m, scheduleAutoDismiss()
		}
		m.toast.Show("Opened in Spotify", "", ToastSuccess)
		return m, scheduleAutoDismiss()
	case ActionAddToPlaylist:
		if subj.trackID == "" {
			m.toast.Show("No track selected", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		return m.openPlaylistPicker(subj.trackID, subj.name)
	case ActionRemoveFromPlaylist:
		if subj.trackID == "" || subj.playlistID == "" {
			m.toast.Show("Nothing to remove", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		return m, m.removeFromPlaylist(subj.playlistID, subj.trackID, subj.name)
	case ActionCopyURI:
		if err := copyToClipboard(subj.uri); err != nil {
			m.toast.Show("Copy failed", err.Error(), ToastError)
			return m, scheduleAutoDismiss()
		}
		m.toast.Show("Copied to clipboard", subj.uri, ToastSuccess)
		return m, scheduleAutoDismiss()
	case ActionPlayPlaylist:
		pl := m.sidebar.SelectedPlaylist()
		if pl != nil {
			return m, m.playPlaylistFromStart(pl.URI)
		}
	case ActionOpenPlaylistSpotify:
		pl := m.sidebar.SelectedPlaylist()
		if pl != nil && pl.URI != "" {
			if err := openInSpotify(pl.URI); err != nil {
				m.toast.Show("Failed to open Spotify", err.Error(), ToastError)
				return m, scheduleAutoDismiss()
			}
			m.toast.Show("Opened in Spotify", pl.Name, ToastSuccess)
			return m, scheduleAutoDismiss()
		}
	case ActionLoadTracks:
		pl := m.sidebar.SelectedPlaylist()
		if pl != nil {
			return m, m.fetchPlaylistTracks(*pl)
		}
	}
	return m, nil
}

func (m Model) handleAddQueue() (Model, tea.Cmd) {
	track := m.tracklist.SelectedTrack()
	if track == nil || track.IsSeparator || track.IsAlbumRow {
		m.toast.Show("No track selected", "", ToastError)
		return m, scheduleAutoDismiss()
	}
	return m, m.queueTrack(track.ID, track.Name)
}

// queueTrack adds the given track to the playback queue.
func (m Model) queueTrack(trackID, name string) tea.Cmd {
	src := m.source
	ctx := m.ctx
	return deviceAware(func() tea.Msg {
		if err := src.AddToQueue(ctx, trackID); err != nil {
			return trackErrorMsg{err}
		}
		return queueDoneMsg{trackName: name}
	})
}

// jumpToCurrentTrack focuses the currently playing track. If it is in the
// current view, the cursor jumps to it. Otherwise the playback context
// (playlist/album/etc.) is loaded and the cursor jumps once it arrives.
func (m Model) jumpToCurrentTrack() (Model, tea.Cmd) {
	if m.track == nil {
		m.toast.Show("No track playing", "", ToastError)
		return m, scheduleAutoDismiss()
	}
	m.focusPane = PaneTrackList
	if m.tracklist.JumpToTrack(m.track.ID) {
		m.toast.Show("Jumped to current track", m.track.Name, ToastSuccess)
		return m, scheduleAutoDismiss()
	}
	// Not in the current view — navigate to the context the track plays from.
	cmd, ok := m.navigateToPlaybackContext()
	if !ok {
		// A non-empty context URI we couldn't resolve is an unsupported type
		// (e.g. a podcast show/episode), not merely an absent track.
		title := "Track not in current view"
		if m.playbackContextURI != "" {
			title = "Can't jump to this context"
		}
		m.toast.Show(title, m.track.Name, ToastInfo)
		return m, scheduleAutoDismiss()
	}
	m.pushNav()
	m.pendingJumpTrackID = m.track.ID
	m.tracklist.SetLoading(m.track.Name)
	return m, cmd
}

// navigateToPlaybackContext returns a command that loads the playlist/album/
// artist the current track is playing from, or (nil, false) when there is no
// resolvable context.
func (m Model) navigateToPlaybackContext() (tea.Cmd, bool) {
	uri := m.playbackContextURI
	if uri == "" {
		return nil, false
	}
	// Playlists and Liked Songs live in the user's library — use that entry so
	// the loaded view has the right name and artwork.
	for _, pl := range m.playlists {
		if pl.URI == uri {
			return m.fetchPlaylistTracks(pl), true
		}
	}
	// Otherwise resolve by URI type: spotify:<type>:<id>.
	parts := strings.Split(uri, ":")
	if len(parts) < 3 {
		return nil, false
	}
	switch parts[1] {
	case "album":
		return m.fetchAlbumPage(parts[2]), true
	case "artist":
		return m.fetchArtistPage(parts[2]), true
	case "playlist":
		return m.fetchPlaylistTracks(source.Playlist{ID: parts[2], URI: uri}), true
	}
	return nil, false
}

// trackIDFromURI extracts the bare ID from a Spotify track URI
// ("spotify:track:abc" → "abc"). Returns "" for any non-track or malformed URI
// so a wrong-type ID can never flow into queue/like calls.
func trackIDFromURI(uri string) string {
	const prefix = "spotify:track:"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	return uri[len(prefix):]
}

// openInSpotify opens a Spotify URI in the Spotify desktop app.
func openInSpotify(uri string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{uri}
	case "linux":
		name = "xdg-open"
		args = []string{uri}
	case "windows":
		name = "cmd"
		args = []string{"/c", "start", uri}
	default:
		return fmt.Errorf("open not supported on %s", runtime.GOOS)
	}
	return exec.Command(name, args...).Start()
}

// copyToClipboard writes text to the system clipboard.
func copyToClipboard(text string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "pbcopy"
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			name = "xclip"
			args = []string{"-selection", "clipboard"}
		} else {
			name = "xsel"
			args = []string{"--clipboard", "--input"}
		}
	case "windows":
		name = "clip"
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
