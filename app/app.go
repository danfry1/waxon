package app

import (
	"context"
	"errors"
	"image"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

const (
	pollInterval         = 1500 * time.Millisecond
	progressTickInterval = 500 * time.Millisecond
)

// Messages — all tea.Msg types used by the Update loop are defined here.
type (
	pollTickMsg     time.Time
	progressTickMsg time.Time
	queueDoneMsg    struct{ trackName string }
	cmdFlashMsg     struct{ text string }
	trackUpdateMsg  struct {
		track      *source.Track
		volume     int
		shuffleOn  bool
		repeatMode source.RepeatMode
	}
)

type (
	trackErrorMsg       struct{ err error }
	moreTracksLoadedMsg struct {
		tracks     []source.Track
		playlistID string
	}
)

type (
	playlistsLoadedMsg struct{ playlists []source.Playlist }
	tracksLoadedMsg    struct {
		tracks     []source.Track
		title      string
		contextURI string
		imageURL   string
		playlistID string // non-empty when fetched from API (for caching)
		total      int    // total tracks in playlist (0 if unknown/fully loaded)
	}
)

type recentTracksLoadedMsg struct {
	tracks []source.Track
}
type (
	queueLoadedMsg   struct{ tracks []source.Track }
	devicesLoadedMsg struct{ devices []source.Device }
	controlDoneMsg   struct{}
	artworkLoadedMsg struct {
		url string
		img image.Image
	}
)

type trackLikedMsg struct {
	trackID string
	liked   bool
}

type trackLikeStatusMsg struct {
	trackID string
	liked   bool
}

type playlistArtLoadedMsg struct {
	url string
	img image.Image
}
type sidebarIconsLoadedMsg struct {
	icons map[string]string // playlist ID → rendered icon
}
type artistPageLoadedMsg struct {
	page *source.ArtistPage
}
type albumPageLoadedMsg struct {
	page *source.AlbumPage
}

type npArtLoadedMsg struct {
	url string
	img image.Image
}

// Model is the root Bubbletea model for waxon.
type Model struct {
	ctx             context.Context
	cancel          context.CancelFunc
	source          source.RichSource
	artworkProvider ArtworkProvider // non-nil when source provides embedded art
	mode            Mode
	focusPane       Pane
	sidebar         Sidebar
	tracklist       TrackList
	statusbar       StatusBar
	albumart        AlbumArt
	search          *Search
	actions         *ActionsPopup
	devices         *DevicePicker
	keys            KeyMap
	gtracker        GTracker
	cmdInput        string
	filterInput     string

	track      *source.Track
	playlists  []source.Playlist
	volume     int
	shuffleOn  bool
	repeatMode source.RepeatMode
	liked      bool // whether the currently playing track is liked
	deviceName string
	toast      Toast
	navStack   []NavState                // browser-like back navigation history
	trackCache map[string]cachedPlaylist // playlist ID → cached tracks
	pagination *paginationState          // non-nil while a playlist is being lazily loaded

	npArt       string      // large rendered art for now playing view
	npArtURL    string      // URL of the art currently rendered for NP
	npSourceImg image.Image // source image for vinyl rendering
	vinylAngle  float64     // current rotation angle in radians
	vinylMode   bool        // easter egg: vinyl spinning mode

	// Progress interpolation state
	lastPollTime time.Time     // when we last received a track update
	lastPollPos  time.Duration // track position at last poll

	// Mouse double-click tracking
	lastClickTime time.Time
	lastClickPane Pane

	// Error backoff: increases poll interval on consecutive API failures
	consecutiveErrors int

	width    int
	height   int
	quitting bool
}

func NewModel(src source.RichSource) Model {
	ctx, cancel := context.WithCancel(context.Background())
	var ap ArtworkProvider
	if provider, ok := src.(ArtworkProvider); ok {
		ap = provider
	}
	return Model{
		ctx:             ctx,
		cancel:          cancel,
		source:          src,
		artworkProvider: ap,
		mode:            ModeNormal,
		focusPane:       PaneSidebar,
		keys:            DefaultKeyMap(),
		albumart:        NewAlbumArt(),
		volume:          50,
		repeatMode:      source.RepeatOff,
		trackCache:      make(map[string]cachedPlaylist),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(pollInterval, func(t time.Time) tea.Msg { return pollTickMsg(t) }),
		tea.Tick(progressTickInterval, func(t time.Time) tea.Msg { return progressTickMsg(t) }),
		m.fetchPlaylists(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutResize()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case pollTickMsg:
		interval := m.backoffInterval()
		return m, tea.Batch(
			m.fetchCurrentTrack(),
			tea.Tick(interval, func(t time.Time) tea.Msg { return pollTickMsg(t) }),
		)

	case trackUpdateMsg:
		m.consecutiveErrors = 0
		prevTrackID := ""
		if m.track != nil {
			prevTrackID = m.track.ID
		}
		m.track = msg.track
		m.volume = msg.volume
		m.shuffleOn = msg.shuffleOn
		m.repeatMode = msg.repeatMode
		var cmds []tea.Cmd
		if msg.track != nil {
			m.tracklist.SetNowPlaying(msg.track.ID)
			m.deviceName = msg.track.DeviceName
			m.lastPollTime = time.Now()
			m.lastPollPos = msg.track.Position
			if msg.track.ArtworkURL != "" && msg.track.ArtworkURL != m.albumart.CurrentURL() {
				m.albumart.SetURL(msg.track.ArtworkURL)
				cmds = append(cmds, m.fetchArtwork(msg.track.ArtworkURL))
			}
			// Fetch analysis and large art for now playing view
			if m.mode == ModeNowPlaying {
				if msg.track.ArtworkURL != "" && m.npArtURL != msg.track.ArtworkURL {
					cmds = append(cmds, m.fetchNPArt(msg.track.ArtworkURL))
				}
			}
			// Refresh queue when track changes (queue shifts forward)
			if msg.track.ID != prevTrackID && m.sidebar.Section() == SectionQueue {
				cmds = append(cmds, m.fetchQueue())
			}
			// Check liked status when track changes
			if msg.track.ID != prevTrackID {
				cmds = append(cmds, m.checkLikeStatus(msg.track.ID))
			}
		} else {
			m.albumart.Clear()
			m.deviceName = ""
			m.liked = false
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case artworkLoadedMsg:
		if m.track != nil && msg.url == m.track.ArtworkURL {
			m.albumart.SetImage(msg.img)
		}
		return m, nil

	case playlistsLoadedMsg:
		m.playlists = msg.playlists
		// sidebar may not exist yet if WindowSizeMsg hasn't arrived;
		// layoutResize will apply playlists once the sidebar is created.
		if m.sidebar.width > 0 {
			m.sidebar.SetPlaylists(msg.playlists)
		}
		cmds := []tea.Cmd{m.fetchSidebarIcons(msg.playlists)}
		if len(msg.playlists) > 0 {
			cmds = append(cmds, m.fetchPlaylistTracks(msg.playlists[0]))
		}
		return m, tea.Batch(cmds...)

	case sidebarIconsLoadedMsg:
		m.sidebar.SetPlaylistIcons(msg.icons)
		return m, nil

	case tracksLoadedMsg:
		if msg.playlistID != "" {
			m.trackCache[msg.playlistID] = cachedPlaylist{
				tracks:    msg.tracks,
				total:     msg.total,
				fetchedAt: time.Now(),
			}
			m.evictTrackCache()
		}
		m.tracklist.SetTracks(msg.tracks, msg.title, msg.contextURI)
		// Set pagination state for lazy loading
		if msg.total > 0 && len(msg.tracks) < msg.total {
			m.pagination = &paginationState{
				playlistID: msg.playlistID,
				contextURI: msg.contextURI,
				imageURL:   msg.imageURL,
				title:      msg.title,
				total:      msg.total,
				loaded:     len(msg.tracks),
			}
			m.tracklist.SetHeaderInfo(FormatPartialTrackListInfo(len(msg.tracks), msg.total))
		} else {
			m.pagination = nil
			m.tracklist.SetHeaderInfo(FormatTrackListInfo(msg.tracks))
		}
		m.tracklist.SetArt("")
		if m.track != nil {
			m.tracklist.SetNowPlaying(m.track.ID)
		}
		if msg.imageURL != "" {
			return m, m.fetchPlaylistArt(msg.imageURL)
		}
		return m, nil

	case moreTracksLoadedMsg:
		if m.pagination != nil && m.pagination.playlistID == msg.playlistID {
			m.tracklist.AppendTracks(msg.tracks)
			m.pagination.loaded += len(msg.tracks)
			m.pagination.loadingMore = false
			// Update cache
			m.trackCache[msg.playlistID] = cachedPlaylist{
				tracks:    m.tracklist.tracks,
				total:     m.pagination.total,
				fetchedAt: time.Now(),
			}
			m.evictTrackCache()
			// Update header
			if m.pagination.loaded >= m.pagination.total {
				m.tracklist.SetHeaderInfo(FormatTrackListInfo(m.tracklist.tracks))
				m.pagination = nil
			} else {
				m.tracklist.SetHeaderInfo(FormatPartialTrackListInfo(m.pagination.loaded, m.pagination.total))
			}
		}
		return m, nil

	case playlistArtLoadedMsg:
		rendered := renderHalfBlocks(msg.img, HeaderArtW, HeaderArtH)
		m.tracklist.SetArt(rendered)
		return m, nil

	case artistPageLoadedMsg:
		if msg.page != nil {
			m.pagination = nil
			tracks := buildArtistTrackList(msg.page)
			m.tracklist.SetTracks(tracks, msg.page.Name, "")
			m.tracklist.SetHeaderInfo(FormatTrackListInfo(msg.page.Tracks))
			if len(msg.page.Genres) > 0 {
				m.tracklist.SetSubtitle(strings.Join(msg.page.Genres, ", "))
			}
			m.tracklist.SetArt("")
			m.focusPane = PaneTrackList
			if m.track != nil {
				m.tracklist.SetNowPlaying(m.track.ID)
			}
			if msg.page.ImageURL != "" {
				return m, m.fetchPlaylistArt(msg.page.ImageURL)
			}
		}
		return m, nil

	case albumPageLoadedMsg:
		if msg.page != nil {
			m.pagination = nil
			title := msg.page.Name
			contextURI := ""
			if msg.page.ID != "" {
				contextURI = "spotify:album:" + msg.page.ID
			}
			m.tracklist.SetTracks(msg.page.Tracks, title, contextURI)
			m.tracklist.SetHeaderInfo(FormatAlbumInfo(msg.page.Artist, msg.page.Year, msg.page.Tracks))
			m.tracklist.SetArt("")
			m.focusPane = PaneTrackList
			if m.track != nil {
				m.tracklist.SetNowPlaying(m.track.ID)
			}
			if msg.page.ImageURL != "" {
				return m, m.fetchPlaylistArt(msg.page.ImageURL)
			}
		}
		return m, nil

	case npArtLoadedMsg:
		m.npArt = renderHalfBlocks(msg.img, npArtW, npArtH)
		m.npArtURL = msg.url
		m.npSourceImg = msg.img
		m.vinylAngle = 0
		return m, nil

	case queueLoadedMsg:
		// Only update the live list if we're still on the queue section;
		// otherwise the async response would clobber the library view.
		if m.sidebar.Section() == SectionQueue {
			m.sidebar.SetQueueTracks(msg.tracks)
		}
		return m, nil

	case searchResultsMsg:
		if m.search != nil {
			var cmd tea.Cmd
			*m.search, cmd = m.search.Update(msg)
			return m, cmd
		}
		return m, nil

	case searchDebounceMsg:
		if m.search != nil && m.search.input.Value() == msg.query {
			return m, m.doSearch(msg.query)
		}
		return m, nil

	case recentTracksLoadedMsg:
		m.tracklist.SetTracks(msg.tracks, "Recently Played", "")
		m.tracklist.SetHeaderInfo(FormatTrackListInfo(msg.tracks))
		m.tracklist.SetArt("")
		if m.track != nil {
			m.tracklist.SetNowPlaying(m.track.ID)
		}
		return m, nil

	case devicesLoadedMsg:
		if len(msg.devices) == 0 {
			m.toast.Show("No devices available", "", ToastError)
			return m, scheduleAutoDismiss()
		}
		picker := NewDevicePicker(msg.devices, m.width, m.height)
		m.devices = &picker
		m.mode = ModeDevices
		return m, nil

	case controlDoneMsg:
		return m, nil

	case queueDoneMsg:
		m.toast.Show("Added to queue", msg.trackName, ToastSuccess)
		// Refresh queue view if it's currently visible
		if m.sidebar.Section() == SectionQueue {
			return m, tea.Batch(scheduleAutoDismiss(), m.fetchQueue())
		}
		return m, scheduleAutoDismiss()

	case cmdFlashMsg:
		m.toast.Show(msg.text, "", ToastInfo)
		return m, scheduleAutoDismiss()

	case trackLikedMsg:
		if m.track != nil && m.track.ID == msg.trackID {
			m.liked = msg.liked
		}
		if msg.liked {
			m.toast.Show("Saved to Liked Songs", "", ToastSuccess)
		} else {
			m.toast.Show("Removed from Liked Songs", "", ToastInfo)
		}
		return m, scheduleAutoDismiss()

	case trackLikeStatusMsg:
		if m.track != nil && m.track.ID == msg.trackID {
			m.liked = msg.liked
		}
		return m, nil

	case trackErrorMsg:
		m.consecutiveErrors++
		if m.pagination != nil {
			m.pagination.loadingMore = false
		}
		if isAuthError(msg.err) {
			m.toast.Show("Session expired", "Run 'waxon auth' to reconnect", ToastError)
		} else {
			m.toast.Show(msg.err.Error(), "", ToastError)
		}
		return m, scheduleAutoDismiss()

	case clearToastMsg:
		m.toast.Hide()
		return m, nil

	case progressTickMsg:
		// Interpolate track position between polls for smooth progress bar
		if m.track != nil && m.track.Playing && !m.lastPollTime.IsZero() {
			elapsed := time.Since(m.lastPollTime)
			m.track.Position = m.lastPollPos + elapsed
			if m.track.Position > m.track.Duration {
				m.track.Position = m.track.Duration
			}
		}
		// Spin vinyl in now playing mode
		if m.mode == ModeNowPlaying && m.vinylMode && m.track != nil && m.track.Playing {
			m.vinylAngle = math.Mod(m.vinylAngle+0.057, 2*math.Pi)
		}
		// Animate loading spinner
		m.tracklist.TickSpinner()
		return m, tea.Tick(progressTickInterval, func(t time.Time) tea.Msg { return progressTickMsg(t) })
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeHelp:
		return m.handleKeyHelp(msg)
	case ModeNowPlaying:
		return m.handleKeyNowPlaying(msg)
	case ModeSearch:
		return m.handleKeySearch(msg)
	case ModeActions:
		return m.handleKeyActions(msg)
	case ModeDevices:
		return m.handleKeyDevices(msg)
	case ModeCommand:
		return m.handleKeyCommand(msg)
	case ModeFilter:
		return m.handleKeyFilter(msg)
	default:
		return m.handleKeyNormal(msg)
	}
}

func (m Model) handleKeyHelp(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyEscape || msg.String() == "?" {
		m.mode = ModeNormal
	}
	return m, nil
}

func (m Model) handleKeyNowPlaying(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "N", msg.Type == tea.KeyEscape, msg.String() == "q":
		m.mode = ModeNormal
		m.vinylMode = false
		return m, nil
	case msg.String() == "V":
		m.vinylMode = !m.vinylMode
		if m.vinylMode {
			m.vinylAngle = 0
		}
		return m, nil
	case key.Matches(msg, m.keys.PlayPause):
		if m.track != nil && m.track.Playing {
			return m, m.controlCmd(m.source.Pause)
		}
		return m, m.controlCmd(m.source.Play)
	case key.Matches(msg, m.keys.Next):
		return m, m.controlCmd(m.source.Next)
	case key.Matches(msg, m.keys.Prev):
		return m, m.controlCmd(m.source.Previous)
	case key.Matches(msg, m.keys.SeekFwd):
		if m.track != nil {
			return m, m.seekRelative(5 * time.Second)
		}
	case key.Matches(msg, m.keys.SeekBack):
		if m.track != nil {
			return m, m.seekRelative(-5 * time.Second)
		}
	case key.Matches(msg, m.keys.Like):
		if m.track != nil {
			return m, m.toggleLike(m.track.ID, m.liked)
		}
	}
	return m, nil
}

func (m Model) handleKeySearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.search == nil {
		return m, nil
	}
	if msg.Type == tea.KeyEscape {
		m.search = nil
		m.mode = ModeNormal
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		if track := m.search.SelectedTrack(); track != nil {
			m.search = nil
			m.mode = ModeNormal
			return m, m.playTrack(track.URI, "")
		}
		if artist := m.search.SelectedArtist(); artist != nil {
			m.search = nil
			m.mode = ModeNormal
			m.pushNav()
			m.tracklist.SetLoading(artist.Name)
			return m, m.fetchArtistPage(artist.ID)
		}
		if album := m.search.SelectedAlbum(); album != nil {
			m.search = nil
			m.mode = ModeNormal
			m.pushNav()
			m.tracklist.SetLoading(album.Name)
			return m, m.fetchAlbumPage(album.ID)
		}
		return m, nil
	}
	var cmd tea.Cmd
	*m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m Model) handleKeyActions(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.actions == nil {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEscape:
		m.actions = nil
		m.mode = ModeNormal
		return m, nil
	case tea.KeyEnter:
		action := m.actions.Selected()
		uri := m.actions.URI()
		artistID := m.actions.ArtistID()
		albumID := m.actions.AlbumID()
		m.actions = nil
		m.mode = ModeNormal
		return m.executeAction(action, uri, artistID, albumID)
	}
	switch msg.String() {
	case "j", "down":
		m.actions.MoveDown()
	case "k", "up":
		m.actions.MoveUp()
	case "q":
		m.actions = nil
		m.mode = ModeNormal
	}
	return m, nil
}

func (m Model) handleKeyDevices(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.devices == nil {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEscape:
		m.devices = nil
		m.mode = ModeNormal
		return m, nil
	case tea.KeyEnter:
		dev := m.devices.Selected()
		if dev != nil {
			deviceID := dev.ID
			deviceName := dev.Name
			m.devices = nil
			m.mode = ModeNormal
			return m, m.transferPlayback(deviceID, deviceName)
		}
		m.devices = nil
		m.mode = ModeNormal
		return m, nil
	}
	switch msg.String() {
	case "j", "down":
		m.devices.MoveDown()
	case "k", "up":
		m.devices.MoveUp()
	case "q":
		m.devices = nil
		m.mode = ModeNormal
	}
	return m, nil
}

func (m Model) handleKeyCommand(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.cmdInput = ""
		m.mode = ModeNormal
		return m, nil
	case tea.KeyEnter:
		input := m.cmdInput
		m.cmdInput = ""
		m.mode = ModeNormal
		return m, m.executeCommand(input)
	case tea.KeyBackspace:
		if len(m.cmdInput) > 0 {
			runes := []rune(m.cmdInput)
			m.cmdInput = string(runes[:len(runes)-1])
		}
		return m, nil
	case tea.KeySpace:
		m.cmdInput += " "
		return m, nil
	case tea.KeyRunes:
		m.cmdInput += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

func (m Model) handleKeyFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.filterInput = ""
		if m.focusPane == PaneSidebar {
			m.sidebar.ClearFilter()
		} else {
			m.tracklist.ClearFilter()
		}
		m.mode = ModeNormal
		return m, nil
	case tea.KeyEnter:
		if m.filterInput == "" {
			if m.focusPane == PaneSidebar {
				m.sidebar.ClearFilter()
			} else {
				m.tracklist.ClearFilter()
			}
		}
		m.filterInput = ""
		m.mode = ModeNormal
		return m, nil
	case tea.KeyBackspace:
		if len(m.filterInput) > 0 {
			runes := []rune(m.filterInput)
			m.filterInput = string(runes[:len(runes)-1])
		}
	case tea.KeySpace:
		m.filterInput += " "
	case tea.KeyRunes:
		m.filterInput += string(msg.Runes)
	default:
		return m, nil
	}
	// Update the live filter for text-modifying keys
	if m.focusPane == PaneSidebar {
		m.sidebar.SetFilter(m.filterInput)
	} else {
		m.tracklist.SetFilter(m.filterInput)
	}
	return m, nil
}

func (m Model) handleKeyNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Handle g-prefix motions
	if m.gtracker.Pending() {
		action := m.gtracker.Feed(msg.String())
		switch action {
		case GActionTop:
			if m.focusPane == PaneSidebar {
				m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyHome})
			} else {
				m.tracklist, _ = m.tracklist.Update(tea.KeyMsg{Type: tea.KeyHome})
			}
			return m, nil
		case GActionLibrary:
			m.focusPane = PaneSidebar
			m.sidebar.SetSection(SectionLibrary)
			return m, nil
		case GActionQueue:
			m.focusPane = PaneSidebar
			m.sidebar.SetSection(SectionQueue)
			return m, m.fetchQueue()
		case GActionCurrent:
			return m.jumpToCurrentTrack()
		case GActionRecent:
			m.pushNav()
			m.tracklist.SetLoading("Recently Played")
			return m, m.fetchRecentlyPlayed()
		default:
			return m, nil
		}
	}

	// "g" starts the g-prefix, "G" goes to bottom immediately
	if msg.String() == "g" {
		m.gtracker.Feed("g")
		return m, nil
	}
	if msg.String() == "G" {
		if m.focusPane == PaneSidebar {
			m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyEnd})
		} else {
			m.tracklist, _ = m.tracklist.Update(tea.KeyMsg{Type: tea.KeyEnd})
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.NowPlaying):
		m.gtracker.Reset()
		m.mode = ModeNowPlaying
		if m.track != nil && m.track.ArtworkURL != "" && m.npArtURL != m.track.ArtworkURL {
			return m, m.fetchNPArt(m.track.ArtworkURL)
		}
		return m, nil

	case key.Matches(msg, m.keys.Quit):
		m.cancel()
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.gtracker.Reset()
		m.mode = ModeHelp
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		m.gtracker.Reset()
		m.mode = ModeFilter
		m.filterInput = ""
		if m.focusPane == PaneSidebar {
			m.sidebar.ClearFilter()
		} else {
			m.tracklist.ClearFilter()
		}
		return m, nil

	case key.Matches(msg, m.keys.Search):
		m.gtracker.Reset()
		s := NewSearch(m.width, m.height)
		m.search = &s
		m.mode = ModeSearch
		return m, nil

	case key.Matches(msg, m.keys.Command):
		m.gtracker.Reset()
		m.mode = ModeCommand
		m.cmdInput = ""
		return m, nil

	case key.Matches(msg, m.keys.PlayPause):
		if m.track != nil && m.track.Playing {
			return m, m.controlCmd(m.source.Pause)
		}
		return m, m.controlCmd(m.source.Play)

	case key.Matches(msg, m.keys.Next):
		return m, m.controlCmd(m.source.Next)

	case key.Matches(msg, m.keys.Prev):
		return m, m.controlCmd(m.source.Previous)

	case key.Matches(msg, m.keys.SeekFwd):
		if m.track != nil {
			return m, m.seekRelative(5 * time.Second)
		}
	case key.Matches(msg, m.keys.SeekBack):
		if m.track != nil {
			return m, m.seekRelative(-5 * time.Second)
		}

	case key.Matches(msg, m.keys.Back):
		if !m.popNav() {
			m.toast.Show("No previous view", "", ToastInfo)
			return m, scheduleAutoDismiss()
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, m.keys.AddQueue):
		return m.handleAddQueue()

	case key.Matches(msg, m.keys.Like):
		if m.focusPane == PaneTrackList {
			track := m.tracklist.SelectedTrack()
			if track == nil || track.IsSeparator || track.IsAlbumRow {
				m.toast.Show("No track selected", "", ToastError)
				return m, scheduleAutoDismiss()
			}
			liked := m.liked && m.track != nil && m.track.ID == track.ID
			return m, m.toggleLike(track.ID, liked)
		}
		// From sidebar, like the currently playing track
		if m.track != nil {
			return m, m.toggleLike(m.track.ID, m.liked)
		}
		m.toast.Show("No track playing", "", ToastError)
		return m, scheduleAutoDismiss()

	case key.Matches(msg, m.keys.Actions):
		m.gtracker.Reset()
		return m.openActions()

	case key.Matches(msg, m.keys.Devices):
		m.gtracker.Reset()
		return m, m.fetchDevices()

	case key.Matches(msg, m.keys.FocusLeft):
		m.focusPane = PaneSidebar
		return m, nil

	case key.Matches(msg, m.keys.FocusRight):
		m.focusPane = PaneTrackList
		return m, nil

	case key.Matches(msg, m.keys.CyclePane):
		if m.focusPane == PaneSidebar {
			m.focusPane = PaneTrackList
		} else {
			m.focusPane = PaneSidebar
		}
		return m, nil

	case key.Matches(msg, m.keys.Section1):
		m.focusPane = PaneSidebar
		m.sidebar.SetSection(SectionLibrary)
		return m, nil

	case key.Matches(msg, m.keys.Section2):
		m.focusPane = PaneSidebar
		m.sidebar.SetSection(SectionQueue)
		return m, m.fetchQueue()

	case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
		key.Matches(msg, m.keys.HalfUp), key.Matches(msg, m.keys.HalfDown):
		if m.focusPane == PaneSidebar {
			var cmd tea.Cmd
			m.sidebar, cmd = m.sidebar.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.tracklist, cmd = m.tracklist.Update(msg)
		// Lazy load: fetch more tracks when cursor nears the end
		if m.pagination != nil && !m.pagination.loadingMore &&
			m.pagination.loaded < m.pagination.total &&
			m.tracklist.table.Cursor()+20 >= m.pagination.loaded {
			m.pagination.loadingMore = true
			return m, tea.Batch(cmd, m.fetchMoreTracks())
		}
		return m, cmd
	}

	return m, nil
}

const doubleClickThreshold = 400 * time.Millisecond

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	// Only handle mouse in normal mode (not overlays)
	if m.mode != ModeNormal {
		return m, nil
	}

	sidebarW := max(20, m.width/4)
	statusRows := 2
	if m.height >= MinTermRows {
		statusRows = ArtHeight
	}
	contentH := m.height - statusRows

	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}
		// Determine which pane was clicked
		if msg.Y >= contentH {
			// Click in status bar area — ignore
			return m, nil
		}
		if msg.X < sidebarW {
			// Click in sidebar — manually set cursor from click Y
			m.focusPane = PaneSidebar
			m.sidebar.SetCursorFromClick(msg.Y)

			// Double-click to open playlist
			now := time.Now()
			if m.lastClickPane == PaneSidebar && now.Sub(m.lastClickTime) < doubleClickThreshold {
				m.lastClickTime = time.Time{}
				pl := m.sidebar.SelectedPlaylist()
				if pl != nil {
					if m.sidebar.Section() == SectionQueue {
						return m, m.playTrack("spotify:track:"+pl.ID, "")
					}
					m.pushNav()
					m.tracklist.SetLoading(pl.Name)
					return m, m.fetchPlaylistTracks(*pl)
				}
			}
			m.lastClickTime = now
			m.lastClickPane = PaneSidebar
			return m, nil
		}
		// Click in tracklist — manually set cursor from click Y
		m.focusPane = PaneTrackList
		m.tracklist.SetCursorFromClick(msg.Y)

		// Double-click to play track
		now := time.Now()
		if m.lastClickPane == PaneTrackList && now.Sub(m.lastClickTime) < doubleClickThreshold {
			m.lastClickTime = time.Time{}
			track := m.tracklist.SelectedTrack()
			if track != nil && !track.IsSeparator {
				if track.IsAlbumRow && track.AlbumID != "" {
					m.pushNav()
					m.tracklist.SetLoading(track.Name)
					return m, m.fetchAlbumPage(track.AlbumID)
				}
				return m, m.playTrack(track.URI, m.tracklist.ContextURI())
			}
		}
		m.lastClickTime = now
		m.lastClickPane = PaneTrackList
		return m, nil

	case tea.MouseButtonWheelUp:
		if m.focusPane == PaneSidebar {
			m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyUp})
		} else {
			m.tracklist, _ = m.tracklist.Update(tea.KeyMsg{Type: tea.KeyUp})
		}
		return m, nil

	case tea.MouseButtonWheelDown:
		if m.focusPane == PaneSidebar {
			m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyDown})
		} else {
			var cmd tea.Cmd
			m.tracklist, cmd = m.tracklist.Update(tea.KeyMsg{Type: tea.KeyDown})
			// Lazy load on scroll too
			if m.pagination != nil && !m.pagination.loadingMore &&
				m.pagination.loaded < m.pagination.total &&
				m.tracklist.table.Cursor()+20 >= m.pagination.loaded {
				m.pagination.loadingMore = true
				return m, tea.Batch(cmd, m.fetchMoreTracks())
			}
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleEnter() (Model, tea.Cmd) {
	if m.focusPane == PaneSidebar {
		pl := m.sidebar.SelectedPlaylist()
		if pl != nil {
			if m.sidebar.Section() == SectionQueue {
				return m, m.playTrack("spotify:track:"+pl.ID, "")
			}
			m.pushNav()
			m.tracklist.SetLoading(pl.Name)
			return m, m.fetchPlaylistTracks(*pl)
		}
	} else {
		track := m.tracklist.SelectedTrack()
		if track != nil {
			// Separator rows are not actionable
			if track.IsSeparator {
				return m, nil
			}
			// Album rows navigate to the album page
			if track.IsAlbumRow && track.AlbumID != "" {
				m.pushNav()
				m.tracklist.SetLoading(track.Name)
				return m, m.fetchAlbumPage(track.AlbumID)
			}
			return m, m.playTrack(track.URI, m.tracklist.ContextURI())
		}
	}
	return m, nil
}

func (m *Model) layoutResize() {
	if m.width == 0 || m.height == 0 {
		return
	}
	sidebarW := max(20, m.width/4)
	tracklistW := m.width - sidebarW

	// Reserve rows for the bottom area
	statusRows := 2
	if m.height >= MinTermRows {
		statusRows = ArtHeight
	}
	contentH := m.height - statusRows

	if m.sidebar.width == 0 {
		m.sidebar = NewSidebar(sidebarW, contentH)
		m.tracklist = NewTrackList(tracklistW, contentH)
		m.statusbar = NewStatusBar(m.width)
		// If playlists arrived before the first WindowSizeMsg (e.g. demo mode
		// with instant responses), apply them now that the sidebar exists.
		if len(m.playlists) > 0 {
			m.sidebar.SetPlaylists(m.playlists)
		}
	} else {
		m.sidebar.Resize(sidebarW, contentH)
		m.tracklist.Resize(tracklistW, contentH)
		m.statusbar.Resize(m.width)
	}
}

// httpStatusError is satisfied by any error type that carries an HTTP status code.
type httpStatusError interface {
	error
	HTTPStatus() int
}

// isAuthError checks whether an error is an HTTP 401 Unauthorized from the
// Spotify API, indicating an expired or revoked token.
func isAuthError(err error) bool {
	var sErr spotifyapi.Error
	if errors.As(err, &sErr) {
		return sErr.Status == http.StatusUnauthorized
	}
	// Check our own APIError type via the httpStatusError interface
	var hse httpStatusError
	if errors.As(err, &hse) {
		return hse.HTTPStatus() == http.StatusUnauthorized
	}
	return false
}

// backoffInterval returns the poll interval, increasing on consecutive errors
// up to a cap of ~48 seconds.
func (m Model) backoffInterval() time.Duration {
	if m.consecutiveErrors <= 0 {
		return pollInterval
	}
	shift := min(m.consecutiveErrors, 5) // cap at 2^5 = 32x
	return pollInterval * time.Duration(1<<shift)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Search overlay replaces everything
	if m.mode == ModeSearch && m.search != nil {
		return m.search.View()
	}

	// Actions overlay replaces everything
	if m.mode == ModeActions && m.actions != nil {
		return m.actions.View()
	}

	// Devices overlay replaces everything
	if m.mode == ModeDevices && m.devices != nil {
		return m.devices.View()
	}

	// Help overlay replaces everything
	if m.mode == ModeHelp {
		return ViewHelp(m.width, m.height)
	}

	// Now Playing overlay replaces everything
	if m.mode == ModeNowPlaying {
		artBlock := m.npArt
		if artBlock == "" {
			artBlock = m.albumart.View()
		}
		return RenderNowPlaying(m.track, artBlock, m.npSourceImg, m.vinylMode, m.vinylAngle, m.width, m.height)
	}

	// Two-pane layout
	sidebarView := m.sidebar.View(m.focusPane == PaneSidebar)
	tracklistView := m.tracklist.View(m.focusPane == PaneTrackList)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, tracklistView)

	// Now-playing area
	var view string
	if m.height >= MinTermRows {
		artView := m.albumart.View()
		if artView == "" {
			artView = PlaceholderArt(ArtWidth, ArtHeight)
		}
		nowPlayingArea := m.statusbar.ViewNowPlayingWithArt(m.track, m.shuffleOn, m.repeatMode, artView, m.volume, m.deviceName, m.mode, m.cmdInput, m.filterInput, m.tracklist.FilterText())
		view = lipgloss.JoinVertical(lipgloss.Left, panes, nowPlayingArea)
	} else {
		nowPlaying := m.statusbar.ViewNowPlaying(m.track, m.shuffleOn, m.repeatMode)
		modeLine := m.statusbar.ViewModeLine(m.mode, m.cmdInput, m.filterInput, m.tracklist.FilterText(), m.volume, m.deviceName)
		view = lipgloss.JoinVertical(lipgloss.Left, panes, nowPlaying, modeLine)
	}

	// Overlay floating toast
	if m.toast.Visible() {
		view = m.toast.Overlay(view, m.width)
	}

	return view
}
