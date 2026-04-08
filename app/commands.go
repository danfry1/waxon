package app

import (
	"context"
	"fmt"
	"image"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

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
			return trackErrorMsg{err}
		}
		if ps == nil {
			return trackUpdateMsg{}
		}
		return trackUpdateMsg{
			track:      ps.Track,
			volume:     ps.Volume,
			shuffleOn:  ps.ShuffleOn,
			repeatMode: ps.RepeatMode,
		}
	}
}

func (m Model) fetchPlaylists() tea.Cmd {
	src := m.source
	ctx := m.ctx
	return func() tea.Msg {
		playlists, err := src.Playlists(ctx)
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistsLoadedMsg{playlists}
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

func (m Model) fetchSidebarIcons(playlists []source.Playlist) tea.Cmd {
	ctx := m.ctx
	ap := m.artworkProvider
	return func() tea.Msg {
		type iconResult struct {
			id   string
			icon string
		}

		// Filter to playlists with images
		var toFetch []source.Playlist
		for _, pl := range playlists {
			if pl.ImageURL != "" {
				toFetch = append(toFetch, pl)
			}
		}

		results := make(chan iconResult, len(toFetch))
		sem := make(chan struct{}, 5) // bounded concurrency

		var wg sync.WaitGroup
		for _, pl := range toFetch {
			wg.Add(1)
			go func(p source.Playlist) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				var img image.Image
				if ap != nil {
					if i, ok := ap.ArtworkImage(p.ImageURL); ok {
						img = i
					}
				}
				if img == nil {
					var err error
					img, err = FetchImage(ctx, p.ImageURL)
					if err != nil {
						return
					}
				}
				results <- iconResult{id: p.ID, icon: renderHalfBlocks(img, 3, 1)}
			}(pl)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		icons := make(map[string]string)
		for r := range results {
			icons[r.id] = r.icon
		}
		return sidebarIconsLoadedMsg{icons: icons}
	}
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
				return controlDoneMsg{} // art fetch errors are non-fatal
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
				return controlDoneMsg{} // art fetch errors are non-fatal
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
	return func() tea.Msg {
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
	}
}

func (m Model) controlCmd(fn func(context.Context) error) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		if err := fn(ctx); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
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
	return func() tea.Msg {
		if err := src.Seek(ctx, pos); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
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
	return func() tea.Msg {
		if err := src.PlayTrack(ctx, contextURI, ""); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
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
		return func() tea.Msg {
			if err := src.SetVolume(ctx, vol); err != nil {
				return trackErrorMsg{err}
			}
			return cmdFlashMsg{fmt.Sprintf("Volume: %d%%", vol)}
		}
	case CmdShuffle:
		m.shuffleOn = !m.shuffleOn
		state := m.shuffleOn
		label := "Shuffle on"
		if !state {
			label = "Shuffle off"
		}
		return func() tea.Msg {
			if err := src.SetShuffle(ctx, state); err != nil {
				return trackErrorMsg{err}
			}
			return cmdFlashMsg{label}
		}
	case CmdRepeat:
		m.repeatMode = source.RepeatMode(cmd.StrArg)
		mode := source.RepeatMode(cmd.StrArg)
		return func() tea.Msg {
			if err := src.SetRepeat(ctx, mode); err != nil {
				return trackErrorMsg{err}
			}
			return cmdFlashMsg{"Repeat: " + cmd.StrArg}
		}
	case CmdDevice:
		return m.fetchDevices()
	case CmdRecent:
		m.pushNav()
		return m.fetchRecentlyPlayed()
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
		liked := m.liked && m.track != nil && m.track.ID == track.ID
		popup := NewTrackActions(track.Name, track.Artist, track.URI, track.ArtistID, track.AlbumID, liked, m.width, m.height)
		m.actions = &popup
		m.mode = ModeActions
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
	m.mode = ModeActions
	return m, nil
}

func (m Model) executeAction(action ActionItem, uri, artistID, albumID string) (Model, tea.Cmd) {
	switch action.Type {
	case ActionPlay:
		track := m.tracklist.SelectedTrack()
		if track != nil {
			return m, m.playTrack(track.URI, m.tracklist.ContextURI())
		}
	case ActionQueue:
		return m.handleAddQueue()
	case ActionLike:
		track := m.tracklist.SelectedTrack()
		if track == nil || track.IsSeparator || track.IsAlbumRow {
			m.toast.Show("No track selected", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		liked := m.liked && m.track != nil && m.track.ID == track.ID
		return m, m.toggleLike(track.ID, liked)
	case ActionGoArtist:
		if artistID == "" {
			m.toast.Show("No artist info available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		m.pushNav()
		m.tracklist.SetLoading("Loading artist...")
		return m, m.fetchArtistPage(artistID)
	case ActionGoAlbum:
		if albumID == "" {
			m.toast.Show("No album info available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		m.pushNav()
		m.tracklist.SetLoading("Loading album...")
		return m, m.fetchAlbumPage(albumID)
	case ActionOpenSpotify:
		if uri == "" {
			m.toast.Show("No URI available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		if err := openInSpotify(uri); err != nil {
			m.toast.Show("Failed to open Spotify", err.Error(), ToastError)
			return m, scheduleAutoDismiss()
		}
		m.toast.Show("Opened in Spotify", "", ToastSuccess)
		return m, scheduleAutoDismiss()
	case ActionCopyURI:
		if err := copyToClipboard(uri); err != nil {
			m.toast.Show("Copy failed", err.Error(), ToastError)
			return m, scheduleAutoDismiss()
		}
		m.toast.Show("Copied to clipboard", uri, ToastSuccess)
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
	id := track.ID
	name := track.Name
	src := m.source
	ctx := m.ctx
	return m, func() tea.Msg {
		if err := src.AddToQueue(ctx, id); err != nil {
			return trackErrorMsg{err}
		}
		return queueDoneMsg{trackName: name}
	}
}

func (m Model) jumpToCurrentTrack() (Model, tea.Cmd) {
	if m.track == nil {
		m.toast.Show("No track playing", "", ToastError)
		return m, scheduleAutoDismiss()
	}
	m.focusPane = PaneTrackList
	if m.tracklist.JumpToTrack(m.track.ID) {
		m.toast.Show("Jumped to current track", m.track.Name, ToastSuccess)
	} else {
		m.toast.Show("Track not in current view", m.track.Name, ToastInfo)
	}
	return m, scheduleAutoDismiss()
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
