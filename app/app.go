package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/media"
	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

// Poll cadence. The progress bar is interpolated locally on a 500ms tick, so
// playback polling only needs to catch track changes and external control;
// polling faster than this just burns Spotify's per-user rate limit (which
// surfaced as "API rate limit exceeded" / "connection issue" toasts).
const (
	pollInterval         = 2500 * time.Millisecond // playing, terminal focused
	pollIntervalPaused   = 8 * time.Second         // paused: nothing moves on its own
	pollIntervalIdle     = 10 * time.Second        // nothing playing anywhere
	pollIntervalBlurred  = 20 * time.Second        // terminal not focused
	progressTickInterval = 500 * time.Millisecond
	// progressTickBlurred keeps a visible-but-unfocused progress bar moving
	// at a cost suited to a window nobody is looking at closely.
	progressTickBlurred = time.Second
	// prefetchLead is how long before a track ends the next track's art is
	// fetched, so the switch shows it without a round trip.
	prefetchLead = 20 * time.Second
	// rateLimitCooldown is how long to stop polling after a 429 that carries no
	// Retry-After header (zmb3 errors drop it). Spotify's limit is a rolling
	// 30-second window, so waiting it out is the fastest way back.
	rateLimitCooldown = 30 * time.Second
	// volumeStep is how far one +/- press moves the volume. Presses within
	// volumeDebounce of each other are sent as one request, and the new
	// level is held against stale polls for volumeHold.
	volumeStep     = 5
	volumeDebounce = 250 * time.Millisecond
	volumeHold     = 2 * time.Second
	// volumeSentHold caps how long a sent level is kept while Spotify still
	// reports the old one: librespot reports volume changes seconds late.
	volumeSentHold = 8 * time.Second
	// deviceHold is how long a move of playback is held against polls that
	// still report the old device.
	deviceHold = 8 * time.Second
	// maxPollInterval caps error backoff so recovery is never more than a
	// minute away once Spotify is reachable again.
	maxPollInterval = 60 * time.Second
)

// playbackRefreshDelays are the delays after a control action (play/next/prev/
// seek) at which we re-poll playback, so the UI — and the lyrics view — pick up
// the change without waiting for the next regular poll tick. Spotify's API can
// lag a skip by several hundred ms, so a few staggered polls reliably catch the
// new track quickly across that variability.
var playbackRefreshDelays = []time.Duration{
	250 * time.Millisecond,
	700 * time.Millisecond,
	1400 * time.Millisecond,
}

// Messages — all tea.Msg types used by the Update loop are defined here.
type (
	pollTickMsg     time.Time
	progressTickMsg time.Time
	queueDoneMsg    struct {
		trackName string
		count     int // batch queueing: tracks queued and not
		failed    int
	}
	cmdFlashMsg     struct{ text string }
	volumeCommitMsg struct{ gen int }
	// volumeSetMsg: Spotify accepted the level sent at press gen.
	volumeSetMsg struct{ vol, gen int }
	// volumeFailedMsg: the level sent at press gen was refused.
	volumeFailedMsg struct {
		gen int
		err error
	}
	// volumeGlowMsg steps the fade of glow id.
	volumeGlowMsg  struct{ id int }
	trackUpdateMsg struct {
		track      *source.Track
		volume     int
		shuffleOn  bool
		repeatMode source.RepeatMode
		contextURI string
	}
)

type (
	trackErrorMsg struct{ err error }
	// pollErrorMsg is a failure of the background playback poll, which retries
	// automatically with backoff. Kept separate from trackErrorMsg so transient
	// blips (timeouts, brief Spotify hiccups around track changes) don't pop a
	// toast on every tick — only auth failures and persistent outages surface.
	pollErrorMsg        struct{ err error }
	moreTracksLoadedMsg struct {
		tracks     []source.Track
		playlistID string
		offset     int // offset the page was requested at
		total      int // playlist size as reported with this page
		// pagination is the list the page was requested for; a refreshed
		// first page starts a new one, and pages of the old are dropped.
		pagination *paginationState
	}
)

// pollErrorToastThreshold is the number of consecutive poll failures before we
// surface a connection-issue toast, so a real outage still informs the user.
const pollErrorToastThreshold = 3

type (
	playlistsLoadedMsg struct {
		playlists []source.Playlist
		initial   bool // first load: also open the first playlist
	}
	tracksLoadedMsg struct {
		tracks     []source.Track
		title      string
		contextURI string
		imageURL   string
		playlistID string // non-empty when fetched from API (for caching)
		total      int    // total tracks in playlist (0 if unknown/fully loaded)

		// Page cache (pagecache.go). playlist is the library playlist this
		// page belongs to and version its contents' version. fromDisk pages
		// came from the cache, current ones matching the live version;
		// revalidate marks the live page fetched to replace a cached one.
		playlist   source.Playlist
		version    string
		fromDisk   bool
		current    bool
		revalidate bool
	}
	// playlistRestrictedMsg means Spotify won't list this playlist's tracks
	// for this app (source.ErrPlaylistRestricted). It can still be played.
	playlistRestrictedMsg struct {
		playlist   source.Playlist
		revalidate bool // found while refreshing a cached copy
	}
	// revalidateFailedMsg is a failed refresh of a cached playlist page.
	revalidateFailedMsg struct {
		playlist source.Playlist
		err      error
	}
	// playlistUnplayableMsg means Spotify refused to play a playlist context
	// even though a device is active (it doesn't expose that playlist to
	// this app at all).
	playlistUnplayableMsg struct{}
)

type recentTracksLoadedMsg struct {
	tracks []source.Track
}
type (
	queueLoadedMsg     struct{ tracks []source.Track }
	devicesLoadedMsg   struct{ devices []source.Device }
	controlDoneMsg     struct{}
	playbackRefreshMsg struct{} // fired shortly after a control action to re-poll
	artworkLoadedMsg   struct {
		url string
		img image.Image
	}
	// artErrorMsg reports a failed artwork download. Art is cosmetic, so it is
	// only logged — but it must not masquerade as a successful control action.
	artErrorMsg struct {
		url string
		err error
	}
	// noPlaybackMsg means the poll succeeded but nothing is playing anywhere.
	// Distinct from trackUpdateMsg so device-level state (volume/shuffle/
	// repeat) isn't zeroed by an empty update.
	noPlaybackMsg struct{}
)

type trackLikedMsg struct {
	trackID string
	liked   bool
}

type trackLikeStatusMsg struct {
	trackID string
	liked   bool
}

// likeStatusErrorMsg is a failed background like-status lookup. Kept apart
// from trackErrorMsg because it fires on every track change: a permission
// refusal should disable the feature once, not toast every few minutes.
type likeStatusErrorMsg struct {
	err     error
	episode bool // the lookup was for a podcast episode
}

type playlistArtLoadedMsg struct {
	url string
	img image.Image
}
type sidebarIconLoadedMsg struct {
	id   string // playlist ID
	icon string // rendered icon
}

// pageLoadFailedMsg reports that an artist or album page (what) didn't
// load.
type pageLoadFailedMsg struct {
	what string
	err  error
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

type lyricsLoadedMsg struct {
	trackID string
	lyrics  *source.Lyrics
}

type lyricsErrorMsg struct {
	trackID string
	err     error
}

// playlistChangedMsg reports a track added to / removed from a playlist.
type playlistChangedMsg struct {
	playlistID   string
	playlistName string
	trackName    string
	added        bool
	position     int // row removed (remove only; -1 if unknown)
	// Batch edits (visual mode): count tracks changed, positions removed in
	// descending order, failed that didn't go through.
	count     int
	positions []int
	failed    int
}

// refreshPlaylistsMsg asks for a (delayed) reload of the library list.
type refreshPlaylistsMsg struct{}

// playlistRefreshDelay is how long to wait after a playlist edit before
// re-reading the library: Spotify's listing endpoint lags writes by a few
// seconds, so an immediate refresh just reads the old counts back.
const playlistRefreshDelay = 4 * time.Second

// playlistCreatedMsg reports a newly created playlist.
type playlistCreatedMsg struct {
	playlist source.Playlist
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
	playlistPick    *PlaylistPicker
	pickReturn      Mode // mode to return to when the playlist picker closes
	// pendingViewRefresh is a playlist whose open view should be refetched on
	// the next delayed library refresh (after an add/remove).
	pendingViewRefresh string
	pendingRetry       tea.Cmd // playback command to replay once a device is picked
	keys               KeyMap
	opts               Options
	gtracker           GTracker
	cmdInput           string
	filterInput        string

	track              *source.Track
	playbackContextURI string // URI of the playlist/album the current track plays from
	pendingJumpTrackID string // track to jump to once its context finishes loading
	playlists          []source.Playlist
	volume             int
	shuffleOn          bool
	repeatMode         source.RepeatMode
	liked              bool // whether the currently playing track is liked
	// likeUnavailable is set once Spotify refuses the library endpoints for
	// this app (development-mode restriction); like checks stop and like
	// actions explain instead of erroring on every track change.
	likeUnavailable bool
	likedCache      map[string]bool // saved state of tracks we've looked up or changed this session
	deviceName      string
	prevDevice      string            // the device played on before deviceName
	deviceTypes     map[string]string // device name → type, as last listed
	// A move to deviceHeld is held against polls that still report the old
	// device until deviceHoldUntil (or one reports the new one).
	deviceHeld      string
	deviceHoldUntil time.Time
	movedAt         time.Time // last move of playback waxon made
	// switchGen numbers device switches, so a Spotify app opened for an
	// earlier one doesn't take playback when it comes online.
	switchGen         int
	spotifyAppOpening bool
	toast             Toast
	actionsReturn     Mode                      // mode to return to when the actions popup closes
	navStack          []navEntry                // browser-like back navigation history
	trackCache        map[string]cachedPlaylist // playlist ID → cached tracks
	// restrictedPlaylists holds playlists Spotify won't list for this app
	// (source.ErrPlaylistRestricted), so reopening one doesn't refetch.
	restrictedPlaylists map[string]bool

	// Startup snapshot (snapshot.go) and page cache (pagecache.go).
	// libraryLive is set once the live library has loaded; snapshotShown
	// means the startup view came from the snapshot. lastPlaylistID is the
	// playlist last opened (saved for the next launch), resumeCursor the
	// cursor to restore when it opens, and tracklistVersion the version of
	// the playlist page on screen.
	libraryLive      bool
	snapshotShown    bool
	lastPlaylistID   string
	resumeCursor     int
	resumePlaylistID string
	tracklistVersion string

	playerErr error // why waxon's own player stopped, if it did
	// System media controls (media.go): what was last published, and the
	// current track's cover for them.
	mediaLast   mediaPublished
	mediaArt    image.Image
	mediaArtFor string           // the artwork URL mediaArt is for
	setup       *Setup           // open in ModeSetup (setting up the player in place)
	helpScroll  int              // first row of the help overlay shown
	// overlayFromNP: the open overlay was opened from Now Playing, which it
	// returns to.
	overlayFromNP bool
	muteRestore   int // the volume before m muted it
	pagination  *paginationState // non-nil while a playlist is being lazily loaded

	npArt        string      // large rendered art for now playing view
	npArtURL     string      // URL of the art currently rendered for NP
	npSourceImg  image.Image // source image for vinyl rendering
	vinylAngle   float64     // current rotation angle in radians
	vinylMode    bool        // easter egg: vinyl spinning mode
	vinylTicking bool        // a vinylTickMsg is scheduled
	vinylLast    time.Time   // when the record was last turned

	// Lyrics (Now Playing overlay, toggled with "l")
	lyricsView    bool           // true when the NP art region shows lyrics
	lyrics        *source.Lyrics // loaded lyrics for lyricsTrackID (nil = none)
	lyricsTrackID string         // track the current lyrics state belongs to
	lyricsLoading bool           // true while a lyrics fetch is in flight
	lyricsErr     error          // non-nil when the last lyrics fetch failed
	lyricsLine    int            // active synced-lyric line index

	// Progress interpolation state
	lastPollTime time.Time     // when we last received a track update
	lastPollPos  time.Duration // track position at last poll

	// Mouse double-click tracking
	lastClickTime time.Time
	lastClickPane Pane

	// Error backoff: increases poll interval on consecutive API failures
	consecutiveErrors int
	// rateLimitedUntil is set on a 429; polling pauses until then.
	rateLimitedUntil time.Time
	// rateLimitHits counts 429s this session (drives the client-ID hint).
	rateLimitHits int
	// blurred is true while the terminal window doesn't have focus.
	blurred bool
	// progressTicking is true while a progressTickMsg is scheduled. The tick
	// stops itself when nothing animates; Update restarts it on demand.
	progressTicking bool
	// count is a pending vim count prefix (0 = none).
	count int
	// prefetchedFor is the track whose successor's art has been prefetched.
	prefetchedFor string
	// volumeGen numbers +/- presses so only the last of a burst is sent;
	// volumeHoldUntil keeps polls from overwriting the level meanwhile.
	volumeGen       int
	volumeHoldUntil time.Time
	volumeSent      bool // the held level was sent; release once Spotify echoes it
	volumeLast      int  // the level waxon last set, shown while the device reports it within rounding
	volumeLastSet   bool
	volumeSentAt    time.Time // last volume sent, to rate-limit a held + key
	// The volume display (volumeShow): a change is in flight from a press
	// until Spotify accepts the newest level; volumeAccepted is the last
	// level it accepted; volumeGlow counts the glow's fade down.
	volumeInFlight bool
	volumeAccepted int
	volumeGlow     int
	volumeGlowID   int
	// modeHoldUntil does the same for shuffle and repeat after S / r.
	modeHoldUntil time.Time
	// refreshPending counts post-control re-polls still scheduled, so a burst
	// of keypresses doesn't stack up dozens of extra requests.
	refreshPending int

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
	m := Model{
		ctx:                 ctx,
		cancel:              cancel,
		source:              src,
		artworkProvider:     ap,
		mode:                ModeNormal,
		focusPane:           PaneSidebar,
		progressTicking:     true, // Init schedules the first tick
		keys:                DefaultKeyMap(),
		albumart:            NewAlbumArt(),
		volume:              50,
		repeatMode:          source.RepeatOff,
		trackCache:          make(map[string]cachedPlaylist),
		restrictedPlaylists: make(map[string]bool),
		likedCache:          make(map[string]bool),
	}
	// Pre-create the panes at a default size so data arriving before the first
	// WindowSizeMsg (instant demo data, or a fast initial load) has a valid
	// layout to render into — otherwise the tracklist table has no columns and
	// panics on render. width/height stay 0 so View defers painting until the
	// real terminal size is known; the first WindowSizeMsg then resizes in place.
	m.width, m.height = 80, 24
	m.layoutResize()
	m.width, m.height = 0, 0
	return m
}

// Options customises a Model beyond its data source.
type Options struct {
	// Keys replaces the default keybindings (see KeyMapFromOverrides).
	Keys *KeyMap
	// ColorOverrides are applied on top of whichever theme is active, so
	// switching themes with :theme keeps the user's tweaks.
	ColorOverrides Palette
	// SaveTheme persists a theme name chosen with :theme. nil = don't persist.
	SaveTheme func(name string) error
	// SharedClientID is true when waxon is using the bundled (shared) Spotify
	// client ID. Rate-limit messages then suggest switching to a personal one.
	SharedClientID bool
	// SnapshotPath is where the startup snapshot of the library is kept
	// (see snapshot.go). "" disables it.
	SnapshotPath string
	// ThisComputer are this machine's names (host name, and on macOS the
	// computer name), which the Spotify app here names its device after.
	ThisComputer []string
	// OpenSpotifyApp starts this computer's Spotify desktop app in the
	// background; nil when it isn't installed (or can't be started here).
	OpenSpotifyApp func() error
	// PlayerName is the Connect device waxon's own player (librespot) runs
	// as; "" when it's off. Playback with no active device prefers it.
	PlayerName string
	// KeysProblem says which of the config's custom keys weren't applied or
	// clash; shown once at startup.
	KeysProblem string
	// PlayerProblem says why the player is on but couldn't start, with
	// the fix (e.g. "Not signed in: run 'waxon player login'"). Shown as a
	// toast once the UI is up.
	PlayerProblem string
	// PlayerSetup, when set, lets the device picker set up waxon's own
	// player in place (see setup_app.go); StartPlayer then starts it and
	// returns its device name.
	PlayerSetup *SetupDeps
	StartPlayer func() string
	// Media publishes what's playing to the system's media controls (media
	// keys, Now Playing); presses come back as MediaMsg. nil: none.
	Media media.Controller
}

// WithOptions returns a copy of m with opts applied.
func (m Model) WithOptions(opts Options) Model {
	m.opts = opts
	if opts.Keys != nil {
		m.keys = *opts.Keys
	}
	return m
}

// setTheme applies a named theme (plus the configured colour overrides) to the
// live UI and reports the outcome as a toast.
func (m *Model) setTheme(name string) tea.Cmd {
	p, err := ResolveTheme(name, m.opts.ColorOverrides)
	if err != nil {
		m.toast.Show(err.Error(), "", ToastError)
		return scheduleAutoDismiss()
	}
	ApplyTheme(p)
	m.tracklist.Restyle()
	m.sidebar.Restyle()
	detail := ""
	if m.opts.SaveTheme != nil {
		if err := m.opts.SaveTheme(name); err != nil {
			detail = "not saved: " + err.Error()
		} else {
			detail = "saved to config"
		}
	}
	m.toast.Show("Theme: "+strings.ToLower(name), detail, ToastSuccess)
	return scheduleAutoDismiss()
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(pollInterval, func(t time.Time) tea.Msg { return pollTickMsg(t) }),
		tea.Tick(progressTickInterval, func(t time.Time) tea.Msg { return progressTickMsg(t) }),
		m.fetchPlaylists(),
		m.loadSnapshot(),
		m.reportPlayerProblem(),
		m.reportKeysProblem(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	res, cmd := m.update(msg)
	nm, ok := res.(Model)
	if !ok {
		return res, cmd
	}
	nm.publishMedia(time.Now())
	if artMode == ArtKitty {
		if nm.quitting {
			kittyClear() // still on the alternate screen, where the images are
		} else {
			kittyReap(time.Now())
		}
	}
	// Visual mode and its selection live and die together, whichever handler
	// changed one of them (a list reload, a device picker opening, …), and a
	// count prefix doesn't survive into another mode.
	switch {
	case nm.mode == ModeVisual && !nm.tracklist.Visual():
		nm.mode = ModeNormal
	case nm.mode != ModeVisual && nm.tracklist.Visual():
		nm.tracklist.EndVisual()
	}
	if nm.mode != ModeNormal && nm.mode != ModeVisual {
		nm.count = 0
	}
	// The lyric highlight follows the position whatever moved it: the tick,
	// a poll, a seek, or lyrics arriving while paused (when no tick runs).
	nm.syncLyricsLine()
	if nm.progressTicking || !nm.needsProgressTick() {
		return nm, cmd
	}
	nm.progressTicking = true
	return nm, tea.Batch(cmd, nm.progressTick())
}

// needsProgressTick reports whether anything on screen animates on the
// progress tick: the playing track's position, vinyl, synced lyrics (all
// driven by playback) or the track list's loading spinner.
func (m Model) needsProgressTick() bool {
	return (m.track != nil && m.track.Playing) || m.tracklist.loading || m.loadingMore()
}

// loadingMore reports whether a further page of the open playlist is being
// fetched (shown as a spinner in the track list header).
func (m Model) loadingMore() bool { return m.pagination != nil && m.pagination.loadingMore }

// syncLyricsLine points the focus lyric line at the (interpolated) position:
// by timestamp for synced lyrics, by progress ratio for plain ones.
func (m *Model) syncLyricsLine() {
	if m.mode != ModeNowPlaying || !m.lyricsView || m.track == nil ||
		m.lyrics == nil || len(m.lyrics.Lines) == 0 {
		return
	}
	if m.lyrics.Synced {
		m.lyricsLine = activeLyricLine(m.lyrics.Lines, m.track.Position)
	} else if m.track.Duration > 0 {
		ratio := float64(m.track.Position) / float64(m.track.Duration)
		m.lyricsLine = clampInt(int(ratio*float64(len(m.lyrics.Lines)-1)), 0, len(m.lyrics.Lines)-1)
	}
}

func (m Model) progressTick() tea.Cmd {
	interval := progressTickInterval
	if m.blurred {
		interval = progressTickBlurred
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg { return progressTickMsg(t) })
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.setupOwnsMsg(msg) {
		return m.updateSetup(msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.setup != nil {
			m.setup.update(msg)
		}
		m.layoutResize()
		m.renderNPArt()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case pollTickMsg:
		interval := m.backoffInterval()
		next := tea.Tick(interval, func(t time.Time) tea.Msg { return pollTickMsg(t) })
		if m.coolingDown() {
			// A tick scheduled before the 429 landed: don't fetch, just wait.
			return m, next
		}
		return m, tea.Batch(m.fetchCurrentTrack(), next)

	case noPlaybackMsg:
		m.consecutiveErrors = 0
		m.rateLimitedUntil = time.Time{}
		m.track = nil
		m.playbackContextURI = ""
		m.albumart.Clear()
		m.deviceName = ""
		m.liked = false
		return m, nil

	case trackUpdateMsg:
		m.consecutiveErrors = 0
		m.rateLimitedUntil = time.Time{}
		prevTrackID := ""
		if m.track != nil {
			prevTrackID = m.track.ID
		}
		m.track = msg.track
		m.playbackContextURI = msg.contextURI
		// Hold a volume the user just set until Spotify reports it back, so
		// a stale poll doesn't snap the level back. Within a couple of points
		// counts as back: devices round the level to their own steps.
		// Afterwards, a level within a couple of points of the one waxon
		// set is still that one (librespot reports 60% as 59%); anything
		// further off was changed elsewhere.
		switch {
		case time.Now().After(m.volumeHoldUntil):
			if m.volumeLastSet && absInt(msg.volume-m.volumeLast) <= 2 {
				m.volume = m.volumeLast
			} else {
				m.volume, m.volumeLastSet = msg.volume, false
			}
		case m.volumeSent && absInt(msg.volume-m.volume) <= 2:
			m.volumeHoldUntil, m.volumeSent = time.Time{}, false
		}
		if time.Now().After(m.modeHoldUntil) {
			m.shuffleOn = msg.shuffleOn
			m.repeatMode = msg.repeatMode
		}
		var cmds []tea.Cmd
		if msg.track != nil {
			m.tracklist.SetNowPlaying(msg.track.ID)
			switch d := msg.track.DeviceName; {
			case m.deviceHeld != "" && d == m.deviceHeld:
				m.deviceHeld = "" // Spotify has caught up with the move
			case m.deviceHeld != "" && time.Now().Before(m.deviceHoldUntil):
				// A poll from before the move: keep the device moved to.
			default:
				m.deviceHeld = ""
				if d != m.deviceName && m.deviceName != "" && d != "" {
					m.prevDevice = m.deviceName
				}
				m.deviceName = d
			}
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
				// Follow track changes with fresh lyrics while the view is open
				// (fetched even with lyrics hidden, so the hint can say whether
				// there are any).
				if msg.track.ID != prevTrackID {
					if c := m.ensureLyrics(); c != nil {
						cmds = append(cmds, c)
					}
				}
			}
			// Refresh queue when track changes (queue shifts forward)
			if msg.track.ID != prevTrackID && m.sidebar.Section() == SectionQueue {
				cmds = append(cmds, m.fetchQueue())
			}
			// Check liked status when track changes
			if msg.track.ID != prevTrackID && !m.likeUnavailable {
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
			m.mediaArt, m.mediaArtFor = msg.img, msg.url
		}
		return m, nil

	case MediaMsg:
		return m.handleMedia(msg)

	case artErrorMsg:
		slog.Warn("artwork fetch failed", "url", msg.url, "error", msg.err)
		return m, nil

	case snapshotLoadedMsg:
		return m.applySnapshot(msg.snap)

	case PlayerExitedMsg:
		return m.handlePlayerExited(msg)
	case playerProblemMsg:
		return m.handlePlayerProblem(msg)

	case playlistsLoadedMsg:
		m.playlists = msg.playlists
		m.libraryLive = true
		// sidebar may not exist yet if WindowSizeMsg hasn't arrived;
		// layoutResize will apply playlists once the sidebar is created.
		if m.sidebar.width > 0 {
			m.sidebar.SetPlaylists(msg.playlists)
		}
		cmds := []tea.Cmd{m.fetchSidebarIcons(msg.playlists), m.saveSnapshot()}
		if msg.initial && len(msg.playlists) > 0 {
			switch shown := m.playlistByURI(m.tracklist.contextURI); {
			case !m.snapshotShown:
				cmds = append(cmds, m.fetchPlaylistTracks(msg.playlists[0]))
			case shown != nil && (pageVersion(*shown) == "" || pageVersion(*shown) != m.tracklistVersion):
				// The startup view, or wherever the user has gone since, came
				// from the cache and has changed (or can't tell): refresh it.
				cmds = append(cmds, m.livePlaylistPage(*shown, true))
			}
		}
		return m, tea.Batch(cmds...)

	case pageMissMsg:
		return m, m.livePlaylistPage(msg.playlist, false)

	case sidebarIconLoadedMsg:
		m.sidebar.SetPlaylistIcons(map[string]string{msg.id: msg.icon})
		return m, nil

	case collectionLoadedMsg:
		if msg.err != nil {
			m.noteRateLimit(msg.err) // a 429 here still pauses polling
		}
		m.pagination = nil
		m.tracklist.SetTracks(msg.rows, msg.title, "")
		m.tracklist.SetHeaderInfo(msg.info)
		m.tracklist.SetArt("")
		if msg.notice != "" {
			m.tracklist.SetNotice(msg.notice)
		}
		return m, nil

	case tracksLoadedMsg:
		// A cached page is judged against the playlist as the library knows
		// it now, which may have refreshed since the read was issued.
		if msg.fromDisk && msg.version != "" {
			if pl := m.playlistByID(msg.playlistID); pl != nil {
				msg.playlist, msg.current = *pl, msg.version == pageVersion(*pl)
			}
		}
		if !msg.fromDisk {
			m.noteLikedTracks(msg.playlistID, msg.tracks)
		}
		var saveCmd tea.Cmd
		if msg.playlistID != "" && (!msg.fromDisk || msg.current) {
			m.trackCache[msg.playlistID] = cachedPlaylist{
				tracks:    msg.tracks,
				total:     msg.total,
				fetchedAt: time.Now(),
				version:   msg.version,
			}
			m.evictTrackCache()
		}
		if !msg.fromDisk && msg.playlist.ID != "" {
			saveCmd = m.savePage(msg)
		}
		// A page for a playlist the user has since left (a slow answer
		// overtaken by a faster one) only updates the caches.
		if !msg.revalidate && msg.playlist.ID != "" && !m.tracklist.Wants(msg.contextURI) {
			return m, saveCmd
		}
		// A live page replacing a cached one the user has since left (or is
		// leaving: another view is loading) only updates the caches. One that
		// matches what's shown keeps the view as it is — scrolled-in pages,
		// cursor, selection — and just marks it current.
		if msg.revalidate {
			if !m.showingLoaded(msg.contextURI) {
				return m, saveCmd
			}
			if samePage(m.tracklist.tracks, msg.tracks) && msg.total == m.shownTotal() {
				m.tracklistVersion = msg.version
				return m, tea.Batch(saveCmd, m.applyPendingJump())
			}
		}
		if msg.playlist.ID != "" {
			if m.lastPlaylistID != msg.playlist.ID {
				m.lastPlaylistID = msg.playlist.ID
				saveCmd = tea.Batch(saveCmd, m.saveSnapshot())
			}
			m.tracklistVersion = msg.version
		}
		if msg.fromDisk && !msg.current {
			saveCmd = tea.Batch(saveCmd, m.livePlaylistPage(msg.playlist, true))
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
			m.tracklist.SetHeaderInfo(partialListInfo(msg.tracks, len(msg.tracks), msg.total))
		} else {
			m.pagination = nil
			m.tracklist.SetHeaderInfo(FormatTrackListInfo(msg.tracks))
		}
		m.tracklist.SetArt("")
		if m.track != nil {
			m.tracklist.SetNowPlaying(m.track.ID)
		}
		if m.resumeCursor > 0 && msg.playlist.ID == m.resumePlaylistID {
			// Where the last session left off (within the first page).
			m.tracklist.GotoRow(m.resumeCursor)
		}
		m.resumeCursor = 0
		// A stale copy can't settle a pending jump (the track may only be on
		// the live page); the refreshed page will.
		jumpCmd := saveCmd
		if !msg.fromDisk || msg.current {
			jumpCmd = tea.Batch(jumpCmd, m.applyPendingJump())
		}
		// A filter that survived a refresh resumes loading the rest.
		if m.tracklist.Filtering() {
			jumpCmd = tea.Batch(jumpCmd, m.maybeLoadMore())
		}
		if msg.imageURL != "" {
			return m, tea.Batch(jumpCmd, m.fetchPlaylistArt(msg.imageURL))
		}
		return m, jumpCmd

	case revalidateFailedMsg:
		m.noteRateLimit(msg.err) // a 429 still pauses polling
		slog.Warn("refreshing cached playlist", "playlist", msg.playlist.ID, "error", msg.err)
		// A jump waiting on this refresh settles on the copy shown instead,
		// rather than staying armed for some later, unrelated view.
		if m.showingLoaded(msg.playlist.URI) {
			return m, m.applyPendingJump()
		}
		return m, nil

	case playlistRestrictedMsg:
		m.restrictedPlaylists[msg.playlist.ID] = true
		m.resumeCursor = 0
		forget := m.forgetPage(msg.playlist.ID)
		if (msg.revalidate && !m.showingLoaded(msg.playlist.URI)) || (!msg.revalidate && !m.tracklist.Wants(msg.playlist.URI)) {
			return m, forget // the user has moved on; remember it for later
		}
		m.pagination = nil
		// There are no tracks to jump to; don't leave a jump armed.
		m.pendingJumpTrackID = ""
		m.tracklist.SetTracks(nil, msg.playlist.Name, msg.playlist.URI)
		m.tracklist.SetNotice(restrictedPlaylistNotice)
		// Focus the notice so its "Enter: play" applies (and so it is the
		// pane shown on narrow terminals) rather than leaving Enter on the
		// sidebar, where it would only reopen the playlist.
		m.focusPane = PaneTrackList
		if n := msg.playlist.TrackCount; n > 0 {
			m.tracklist.SetHeaderInfo(fmt.Sprintf("%d %s", n, pluralize(n, "track", "tracks")))
		}
		m.tracklist.SetArt("")
		if msg.playlist.ImageURL != "" {
			return m, tea.Batch(forget, m.fetchPlaylistArt(msg.playlist.ImageURL))
		}
		return m, forget

	case playlistUnplayableMsg:
		m.toast.Show("Spotify won't play this playlist from waxon", "Press o to open it in the Spotify app", ToastError)
		return m, scheduleAutoDismiss()

	case moreTracksLoadedMsg:
		m.noteLikedTracks(msg.playlistID, msg.tracks)
		// The offset check drops a page that was requested before the view
		// was reloaded, which would otherwise be appended twice.
		if m.pagination != nil && m.pagination.playlistID == msg.playlistID &&
			msg.offset == m.pagination.loaded &&
			(msg.pagination == nil || msg.pagination == m.pagination) {
			m.tracklist.AppendTracks(msg.tracks)
			m.pagination.loaded += len(msg.tracks)
			m.pagination.loadingMore = false
			// Follow the playlist's live size: if it shrank since the first
			// page, or a page comes back empty, stop rather than requesting the
			// same offset forever.
			if msg.total > 0 {
				m.pagination.total = msg.total
			}
			if len(msg.tracks) == 0 {
				m.pagination.total = m.pagination.loaded
			}
			// Update cache
			m.trackCache[msg.playlistID] = cachedPlaylist{
				tracks:    m.tracklist.tracks,
				total:     m.pagination.total,
				fetchedAt: time.Now(),
				version:   m.tracklistVersion,
			}
			m.evictTrackCache()
			// Update header
			if m.pagination.loaded >= m.pagination.total {
				m.tracklist.SetHeaderInfo(FormatTrackListInfo(m.tracklist.tracks))
				m.pagination = nil
			} else {
				m.tracklist.SetHeaderInfo(partialListInfo(m.tracklist.tracks, m.pagination.loaded, m.pagination.total))
			}
		}
		// An active filter keeps pulling pages until the whole playlist is
		// loaded, so matches beyond the first page aren't silently missed.
		return m, tea.Batch(m.applyPendingJump(), m.maybeLoadMore())

	case playlistArtLoadedMsg:
		rendered := renderHalfBlocks(msg.img, HeaderArtW, HeaderArtH)
		m.tracklist.SetArt(rendered)
		return m, nil

	case keysProblemMsg:
		m.toast.Show("Some custom keys in config.json need fixing", msg.problem, ToastError)
		return m, scheduleAutoDismiss()

	case pageLoadFailedMsg:
		return m.pageLoadFailed(msg)

	case artistPageLoadedMsg:
		if msg.page != nil {
			m.pagination = nil
			tracks := buildArtistTrackList(msg.page)
			m.tracklist.SetTracks(tracks, msg.page.Name, "")
			m.tracklist.SetHeaderInfo(formatArtistInfo(msg.page))
			if len(msg.page.Tracks) == 0 && len(tracks) > 1 {
				// No top tracks: start on the first release, not its heading.
				m.tracklist.GotoTop()
				m.tracklist.MoveDown(1)
			}
			if len(msg.page.Genres) > 0 {
				m.tracklist.SetSubtitle(strings.Join(msg.page.Genres, ", "))
			}
			m.tracklist.SetArt("")
			m.focusPane = PaneTrackList
			if m.track != nil {
				m.tracklist.SetNowPlaying(m.track.ID)
			}
			jumpCmd := m.applyPendingJump()
			if msg.page.ImageURL != "" {
				return m, tea.Batch(jumpCmd, m.fetchPlaylistArt(msg.page.ImageURL))
			}
			return m, jumpCmd
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
			jumpCmd := m.applyPendingJump()
			if msg.page.ImageURL != "" {
				return m, tea.Batch(jumpCmd, m.fetchPlaylistArt(msg.page.ImageURL))
			}
			return m, jumpCmd
		}
		return m, nil

	case npArtLoadedMsg:
		m.npArtURL = msg.url
		m.npSourceImg = msg.img
		m.vinylAngle = 0
		m.renderNPArt()
		return m, nil

	case lyricsLoadedMsg:
		// Ignore stale responses for a track we've since moved on from.
		if m.track != nil && msg.trackID == m.track.ID {
			m.lyricsLoading = false
			m.lyrics = msg.lyrics
			m.lyricsErr = nil
			m.lyricsLine = 0
		}
		return m, nil

	case lyricsErrorMsg:
		if m.track != nil && msg.trackID == m.track.ID {
			m.lyricsLoading = false
			m.lyrics = nil
			m.lyricsErr = msg.err
			// Clear the attempted-track marker so the failure is retryable:
			// toggling the lyrics view (or the next track change) refetches.
			// A transient lrclib error shouldn't pin "Couldn't load lyrics"
			// for the life of the track. (A successful "no lyrics found"
			// result keeps its marker and stays cached — only errors retry.)
			m.lyricsTrackID = ""
			slog.Warn("lyrics fetch failed", "track", msg.trackID, "error", msg.err)
		}
		return m, nil

	case playlistChangedMsg:
		// The cached track list for that playlist is stale now.
		cmds := make([]tea.Cmd, 0, 3)
		cmds = append(cmds, scheduleAutoDismiss(), m.forgetPage(msg.playlistID))
		n := max(1, msg.count)
		switch {
		case msg.failed > 0:
			verb := "Removed"
			if msg.added {
				verb = "Added"
			}
			m.toast.Show(fmt.Sprintf("%s %d of %d tracks", verb, n, n+msg.failed), "Spotify refused the rest — try again shortly", ToastError)
		case msg.added:
			m.toast.Show("Added to "+msg.playlistName, msg.trackName, ToastSuccess)
		default:
			m.toast.Show("Removed from "+msg.playlistName, msg.trackName, ToastInfo)
		}
		// Update the count immediately; Spotify's playlist listing lags edits by
		// a few seconds, so the reconciling refresh is delayed.
		delta := -n
		if msg.added {
			delta = n
		}
		for i := range m.playlists {
			if m.playlists[i].ID == msg.playlistID {
				m.playlists[i].TrackCount = max(0, m.playlists[i].TrackCount+delta)
			}
		}
		m.sidebar.AdjustPlaylistCount(msg.playlistID, delta)
		// If that playlist is on screen, reflect a removal immediately; the
		// authoritative refetch (which also picks up additions) runs with the
		// delayed refresh below.
		if m.tracklist.ContextURI() == "spotify:playlist:"+msg.playlistID {
			if !msg.added && msg.position >= 0 {
				m.tracklist.RemoveAt(msg.position)
				m.tracklist.SetHeaderInfo(FormatTrackListInfo(m.tracklist.tracks))
			}
			if !msg.added && len(msg.positions) > 0 {
				for _, p := range msg.positions { // descending, so indices stay valid
					m.tracklist.RemoveAt(p)
				}
				m.tracklist.SetHeaderInfo(FormatTrackListInfo(m.tracklist.tracks))
			}
			m.pendingViewRefresh = msg.playlistID
		}
		cmds = append(cmds, tea.Tick(playlistRefreshDelay, func(time.Time) tea.Msg { return refreshPlaylistsMsg{} }))
		return m, tea.Batch(cmds...)

	case refreshPlaylistsMsg:
		cmds := []tea.Cmd{m.refreshPlaylists()}
		if id := m.pendingViewRefresh; id != "" {
			m.pendingViewRefresh = ""
			// Only while that playlist is still shown: not if the user has
			// opened another meanwhile, even one still loading.
			if m.showingLoaded("spotify:playlist:" + id) {
				cmds = append(cmds, m.forgetPage(id))
				for _, pl := range m.playlists {
					if pl.ID == id {
						// Straight to Spotify: the cached page is the pre-edit
						// one, and it's being deleted concurrently.
						m.tracklist.Expect(pl.URI)
						cmds = append(cmds, m.livePlaylistPage(pl, false))
						break
					}
				}
			}
		}
		return m, tea.Batch(cmds...)

	case playlistCreatedMsg:
		m.toast.Show("Created playlist", msg.playlist.Name, ToastSuccess)
		return m, tea.Batch(scheduleAutoDismiss(), m.refreshPlaylists())

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
		m.pagination = nil
		m.tracklist.SetTracks(msg.tracks, "Recently Played", "")
		m.tracklist.SetHeaderInfo(FormatTrackListInfo(msg.tracks))
		m.tracklist.SetArt("")
		if m.track != nil {
			m.tracklist.SetNowPlaying(m.track.ID)
		}
		return m, m.applyPendingJump()

	case noActiveDeviceMsg:
		// A volume change waits on a device now, perhaps forever (the
		// picker can be cancelled); its retry still lands it if one is chosen.
		m.volumeInFlight = false
		if m.toast.meter {
			m.toast.Hide()
		}
		return m, m.fetchDevicesForRetry(msg.retry, msg.refused)

	case deviceChoicesMsg:
		return m.handleDeviceChoices(msg)

	case deviceTargetMsg:
		return m.deviceTarget(msg)

	case playbackMovedMsg:
		// Polls take a moment to catch up; until they do, this is where
		// playback is (so a second M goes back, not here again).
		if msg.device != m.deviceName {
			m.prevDevice, m.deviceName = m.deviceName, msg.device
		}
		m.deviceHeld, m.deviceHoldUntil = msg.device, time.Now().Add(deviceHold)
		m.movedAt = time.Now()
		title, detail := m.modeLabel(msg.device)
		m.toast.Show(title, detail, ToastSuccess)
		return m, scheduleAutoDismiss()

	case spotifyAppMsg:
		return m.spotifyAppReady(msg)

	case devicesLoadedMsg:
		for _, d := range msg.devices {
			m.noteDevice(d)
		}
		if len(msg.devices) == 0 {
			if m.canOfferDevices() {
				return m.openPlayHerePicker()
			}
			m.toast.Show("No Spotify device found", m.noDeviceHint(), ToastError)
			return m, scheduleAutoDismiss()
		}
		picker := NewDevicePicker(m.withOpenSpotify(m.withPlayHere(msg.devices)), m.width, m.height)
		m.devices = &picker
		m.mode = ModeDevices
		return m, nil

	case controlDoneMsg:
		// A successful control action proves connectivity, so drop any error
		// backoff and re-poll (a few staggered times) so the UI and lyrics
		// reflect the change quickly instead of waiting for the next regular
		// poll — and so a skip is caught even when Spotify's API lags it.
		m.consecutiveErrors = 0
		m.rateLimitedUntil = time.Time{}
		if m.refreshPending > 0 {
			return m, nil // a burst is already in flight; it will pick this up
		}
		m.refreshPending = len(playbackRefreshDelays)
		cmds := make([]tea.Cmd, len(playbackRefreshDelays))
		for i, d := range playbackRefreshDelays {
			cmds[i] = tea.Tick(d, func(time.Time) tea.Msg { return playbackRefreshMsg{} })
		}
		return m, tea.Batch(cmds...)

	case playbackRefreshMsg:
		if m.refreshPending > 0 {
			m.refreshPending--
		}
		if m.coolingDown() {
			return m, nil
		}
		return m, m.fetchCurrentTrack()

	case tea.FocusMsg:
		m.blurred = false
		return m, nil

	case tea.BlurMsg:
		m.blurred = true
		return m, nil

	case queueDoneMsg:
		if msg.failed > 0 {
			m.toast.Show(fmt.Sprintf("Queued %d of %d tracks", msg.count, msg.count+msg.failed), "Spotify refused the rest — try again shortly", ToastError)
		} else {
			m.toast.Show("Added to queue", msg.trackName, ToastSuccess)
		}
		// Refresh queue view if it's currently visible
		if m.sidebar.Section() == SectionQueue {
			return m, tea.Batch(scheduleAutoDismiss(), m.fetchQueue())
		}
		return m, scheduleAutoDismiss()

	case volumeCommitMsg:
		// Only the last of a burst of +/- presses is sent.
		if msg.gen != m.volumeGen {
			return m, nil
		}
		return m, m.runCommand(Command{Type: CmdVolume, IntArg: m.volume})

	case likedBatchMsg:
		forget := m.forgetPage(likedPlaylistID)
		for _, id := range msg.ids {
			m.likedCache[id] = true
			if m.track != nil && m.track.ID == id {
				m.liked = true
			}
		}
		if msg.failed > 0 {
			m.toast.Show(fmt.Sprintf("Liked %d of %d tracks", len(msg.ids), len(msg.ids)+msg.failed), "Spotify refused the rest — try again shortly", ToastError)
		} else {
			m.toast.Show("Added to Liked Songs", fmt.Sprintf("%d %s", len(msg.ids), pluralize(len(msg.ids), "track", "tracks")), ToastSuccess)
		}
		return m, tea.Batch(scheduleAutoDismiss(), forget)

	case cmdFlashMsg:
		m.toast.Show(msg.text, "", ToastInfo)
		return m, scheduleAutoDismiss()

	case volumeSetMsg:
		m.volumeAccepted = msg.vol
		if msg.gen != m.volumeGen {
			return m, nil // a newer level is on its way
		}
		m.volumeInFlight = false
		m.volumeGlow = volumeFadeSteps
		m.volumeGlowID++
		m.toast.ShowVolume()
		return m, tea.Batch(scheduleAutoDismiss(), volumeGlowTick(m.volumeGlowID))

	case volumeFailedMsg:
		if msg.gen == m.volumeGen {
			m.volumeInFlight = false
			if m.toast.meter {
				m.toast.Hide() // an error toast follows
			}
		}
		return m.Update(trackErrorMsg{msg.err})

	case volumeGlowMsg:
		if msg.id != m.volumeGlowID || m.volumeGlow == 0 {
			return m, nil
		}
		m.volumeGlow--
		if m.volumeGlow > 0 {
			return m, volumeGlowTick(msg.id)
		}
		return m, nil

	case trackLikedMsg:
		if m.track != nil && m.track.ID == msg.trackID {
			m.liked = msg.liked
		}
		m.likedCache[msg.trackID] = msg.liked
		// Invalidate cached Liked Songs so navigating there shows fresh data
		forget := m.forgetPage(likedPlaylistID)
		if msg.liked {
			m.toast.Show("Saved to Liked Songs", "", ToastSuccess)
		} else {
			m.toast.Show("Removed from Liked Songs", "", ToastInfo)
		}
		return m, tea.Batch(scheduleAutoDismiss(), forget)

	case likeStatusErrorMsg:
		// A refusal for an episode says nothing about tracks; don't let it
		// switch track like-checks off for the session.
		if errors.Is(msg.err, source.ErrForbidden) && msg.episode {
			return m, nil
		}
		if errors.Is(msg.err, source.ErrForbidden) {
			if !m.likeUnavailable {
				m.likeUnavailable = true
				slog.Warn("like status not permitted for this app; disabling like checks", "error", msg.err)
			}
			return m, nil
		}
		if isRateLimitError(msg.err) {
			m.noteRateLimit(msg.err)
			return m, nil
		}
		// Other failures are transient and cosmetic (the heart may lag); log only.
		slog.Debug("like status lookup failed", "error", msg.err)
		return m, nil

	case trackLikeStatusMsg:
		if m.track != nil && m.track.ID == msg.trackID {
			m.liked = msg.liked
		}
		m.likedCache[msg.trackID] = msg.liked
		if m.actions != nil && m.actions.TrackID() == msg.trackID {
			m.actions.SetLiked(msg.liked)
		}
		return m, nil

	case trackErrorMsg:
		slog.Warn("command failed", "error", msg.err)
		// A rejected change (e.g. a device without volume control) should
		// show Spotify's real state on the next poll, not the optimistic one.
		m.volumeHoldUntil, m.modeHoldUntil = time.Time{}, time.Time{}
		m.resumeCursor = 0
		m.consecutiveErrors++
		m.noteRateLimit(msg.err)
		if m.pagination != nil {
			m.pagination.loadingMore = false
		}
		// A failed context load must not leave the jump-to-current-track
		// workflow armed, or a later unrelated load would yank the cursor to a
		// stale track. Drop the pending jump and restore the view we came from
		// so the user isn't stranded on a permanent loading spinner.
		if m.pendingJumpTrackID != "" {
			m.pendingJumpTrackID = ""
			m.popNav()
		}
		switch {
		case isAuthError(msg.err):
			m.toast.Show("Session expired", "Run 'waxon auth' to reconnect", ToastError)
		case isRateLimitError(msg.err):
			m.toast.Show("Spotify is rate limiting", m.rateLimitHint(msg.err), ToastError)
		default:
			if title, detail, ok := friendlyPlaybackError(msg.err); ok {
				m.toast.Show(title, detail, ToastError)
			} else {
				m.toast.Show(msg.err.Error(), "", ToastError)
			}
		}
		return m, scheduleAutoDismiss()

	case pollErrorMsg:
		m.consecutiveErrors++
		// A dead/expired token is worth surfacing immediately.
		if isAuthError(msg.err) {
			m.toast.Show("Session expired", "Run 'waxon auth' to reconnect", ToastError)
			return m, scheduleAutoDismiss()
		}
		// A 429 isn't an outage: pause polling for the cooldown and say so
		// once, quietly. Don't let it count towards the connection-issue toast.
		if isRateLimitError(msg.err) {
			first := m.rateLimitedUntil.IsZero()
			m.noteRateLimit(msg.err)
			m.consecutiveErrors--
			if first {
				m.toast.Show("Spotify rate limit", m.rateLimitHint(msg.err), ToastInfo)
				return m, scheduleAutoDismiss()
			}
			return m, nil
		}
		// Otherwise the poll self-heals on the next tick (with backoff); only
		// flag it once failures persist, and only once, to avoid nagging.
		if m.consecutiveErrors == pollErrorToastThreshold {
			m.toast.Show("Spotify connection issue", "Retrying…", ToastInfo)
			return m, scheduleAutoDismiss()
		}
		return m, nil

	case clearToastMsg:
		if m.toast.Expired(msg.at) {
			m.toast.Hide()
		}
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
		// Vinyl spins on its own, faster tick; start it if playback resumed.
		vinylCmd := m.startVinylSpin()
		// Animate loading spinner
		m.tracklist.TickSpinner()
		prefetch := tea.Batch(m.maybePrefetchNext(), vinylCmd)
		if !m.needsProgressTick() {
			m.progressTicking = false
			return m, prefetch
		}
		return m, tea.Batch(prefetch, m.progressTick())

	case vinylTickMsg:
		if !m.vinylSpinning() {
			m.vinylTicking = false
			return m, nil
		}
		now := time.Time(msg)
		if !m.vinylLast.IsZero() {
			dt := now.Sub(m.vinylLast).Seconds()
			m.vinylAngle = math.Mod(m.vinylAngle+vinylOmega*math.Min(dt, 0.5), 2*math.Pi)
		}
		m.vinylLast = now
		return m, vinylTick()
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m.quit() // from anywhere, as in any terminal program
	}
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
	case ModePlaylistPick:
		return m.handleKeyPlaylistPick(msg)
	case ModeCommand:
		return m.handleKeyCommand(msg)
	case ModeFilter:
		return m.handleKeyFilter(msg)
	case ModeVisual:
		return m.handleKeyVisual(msg)
	default:
		return m.handleKeyNormal(msg)
	}
}

// keysProblemMsg carries Options.KeysProblem to the UI.
type keysProblemMsg struct{ problem string }

func (m Model) reportKeysProblem() tea.Cmd {
	if m.opts.KeysProblem == "" {
		return nil
	}
	p := m.opts.KeysProblem
	// After the player's notice, if there's one, so both are seen.
	return tea.Tick(toastDuration, func(time.Time) tea.Msg { return keysProblemMsg{p} })
}

// quit saves where the user was and ends the program.
func (m Model) quit() (Model, tea.Cmd) {
	m.saveSnapshotOnQuit()
	m.cancel()
	m.quitting = true
	return m, tea.Quit
}

func (m Model) handleKeyHelp(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyEscape || key.Matches(msg, m.keys.Help) || msg.String() == "q" {
		m.leaveOverlay()
		m.helpScroll = 0
		return m, nil
	}
	half := max(1, helpRows(m.height)/2)
	switch {
	case key.Matches(msg, m.keys.Down):
		m.helpScroll++
	case key.Matches(msg, m.keys.Up):
		m.helpScroll--
	case key.Matches(msg, m.keys.HalfDown):
		m.helpScroll += half
	case key.Matches(msg, m.keys.HalfUp):
		m.helpScroll -= half
	case key.Matches(msg, m.keys.Bottom):
		m.helpScroll = helpMaxScroll(m.keys, m.width, m.height)
	case msg.String() == "g":
		m.helpScroll = 0
	}
	m.helpScroll = max(0, min(m.helpScroll, helpMaxScroll(m.keys, m.width, m.height)))
	return m, nil
}

// ensureLyrics starts a lyrics fetch for the current track unless one is
// already loaded, attempted, or in flight for it. Returns nil when nothing to do.
func (m *Model) ensureLyrics() tea.Cmd {
	if m.track == nil {
		return nil
	}
	if m.lyricsTrackID == m.track.ID {
		return nil // already loaded/attempted/in-flight for this track
	}
	if m.track.IsEpisode {
		// Podcasts have no lyrics; settle on "none" without a lookup.
		m.lyricsTrackID, m.lyrics, m.lyricsErr, m.lyricsLoading = m.track.ID, nil, nil, false
		return nil
	}
	m.lyricsLoading = true
	m.lyrics = nil
	m.lyricsErr = nil
	m.lyricsLine = 0
	m.lyricsTrackID = m.track.ID
	return m.fetchLyrics(*m.track)
}

// npView is what Now Playing's art region shows.
type npView int

const (
	npViewLast   npView = iota // as Now Playing was last left
	npViewCover                // the album cover
	npViewLyrics               // synced lyrics
	npViewVinyl                // the spinning record
)

// openNowPlaying opens Now Playing on view, loading what it needs.
func (m *Model) openNowPlaying(view npView) tea.Cmd {
	m.mode = ModeNowPlaying
	if view != npViewLast {
		m.lyricsView, m.vinylMode = view == npViewLyrics, view == npViewVinyl
		if m.vinylMode {
			m.vinylAngle = 0
		}
	}
	cmds := []tea.Cmd{m.ensureLyrics(), m.startVinylSpin()} // ensureLyrics: so the hint can say if there are lyrics
	if m.track != nil && m.track.ArtworkURL != "" && m.npArtURL != m.track.ArtworkURL {
		cmds = append(cmds, m.fetchNPArt(m.track.ArtworkURL))
	} else {
		m.renderNPArt() // kitty art is only placed while the view is open
	}
	return tea.Batch(cmds...)
}

func (m Model) handleKeyNowPlaying(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.gtracker.Pending() {
		// ga / gA / gc work here too, on the playing track.
		switch m.gtracker.Feed(msg.String()) {
		case GActionGoAlbum:
			return m.goToAlbumOrArtist(ActionGoAlbum)
		case GActionGoArtist:
			return m.goToAlbumOrArtist(ActionGoArtist)
		case GActionCurrent:
			m.mode = ModeNormal
			return m.jumpToCurrentTrack()
		}
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.NowPlaying), msg.Type == tea.KeyEscape, msg.String() == "q":
		// Lyrics/vinyl stay chosen: N reopens Now Playing as it was left.
		m.mode = ModeNormal
		return m, nil
	case msg.String() == "g":
		m.gtracker.Feed("g")
		return m, nil
	case msg.String() == "v", msg.String() == "V":
		m.vinylMode = !m.vinylMode
		if m.vinylMode {
			m.vinylAngle = 0
			m.lyricsView = false // vinyl and lyrics share the art region
			return m, m.startVinylSpin()
		}
		m.renderNPArt()
		return m, nil
	case msg.String() == "l":
		m.lyricsView = !m.lyricsView
		if m.lyricsView {
			m.vinylMode = false
			return m, m.ensureLyrics()
		}
		return m, nil
	// Here the selection is the playing track, so f, a and o act on it.
	case key.Matches(msg, m.keys.Like):
		if m.track != nil {
			return m, m.toggleLike(m.track.ID, m.liked)
		}
		return m, nil
	case key.Matches(msg, m.keys.AddQueue):
		if m.track != nil {
			return m, m.queueTrack(m.track.ID, m.track.Name)
		}
		return m, nil
	case key.Matches(msg, m.keys.Actions):
		if m.track != nil {
			return m.openPlayingActions(), nil
		}
		return m, nil
	}
	if nm, cmd, ok := m.handleGlobalKey(msg, 0); ok {
		return nm, cmd
	}
	return m, nil
}

// searchPlayContext is the context a search result plays in: its album, when
// known. Without one Spotify plays the single track and then stops.
func searchPlayContext(t source.Track) string {
	if t.AlbumID == "" {
		return ""
	}
	return "spotify:album:" + t.AlbumID
}

func (m Model) handleKeySearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.search == nil {
		return m, nil
	}
	if msg.Type == tea.KeyEscape {
		m.search = nil
		m.leaveOverlay()
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		if track := m.search.SelectedTrack(); track != nil {
			m.search = nil
			m.leaveOverlay()
			// Play it inside its album so playback carries on afterwards,
			// as it does when the track is picked from the album page.
			return m, m.playTrack(track.URI, searchPlayContext(*track))
		}
		if artist := m.search.SelectedArtist(); artist != nil {
			m.search = nil
			m.leaveOverlay()
			m.pushNav()
			m.tracklist.SetLoading(artist.Name)
			return m, m.fetchArtistPage(artist.ID)
		}
		if album := m.search.SelectedAlbum(); album != nil {
			m.search = nil
			m.leaveOverlay()
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
		m.mode = m.actionsReturn
		return m, nil
	case tea.KeyEnter:
		subj := actionSubject{
			trackID:    m.actions.TrackID(),
			name:       m.actions.Name(),
			uri:        m.actions.URI(),
			contextURI: m.actions.ContextURI(),
			artistID:   m.actions.ArtistID(),
			albumID:    m.actions.AlbumID(),
			playlistID: m.actions.PlaylistID(),
			position:   m.actions.Position(),
		}
		action := m.actions.Selected()
		m.actions = nil
		// Default to returning to wherever the popup was opened from (e.g. Now
		// Playing). Actions that navigate to a new tracklist override this with
		// ModeNormal. This keeps error paths consistent with success paths.
		m.mode = m.actionsReturn
		return m.executeAction(action, subj)
	}
	switch msg.String() {
	case "j", "down":
		m.actions.MoveDown()
	case "k", "up":
		m.actions.MoveUp()
	case "q":
		m.actions = nil
		m.mode = m.actionsReturn
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
		m.pendingRetry = nil
		m.leaveOverlay()
		return m, nil
	case tea.KeyEnter:
		dev := m.devices.Selected()
		retry := m.pendingRetry
		m.devices = nil
		m.pendingRetry = nil
		m.leaveOverlay()
		if dev == nil {
			return m, nil
		}
		if dev.ID == playHereDeviceID {
			return m.openPlayerSetup()
		}
		if dev.ID == openSpotifyDeviceID {
			return m.startSpotifyApp(retry)
		}
		m.supersedeSwitch()
		if retry != nil {
			// Picker opened to recover a failed command: activate, then replay it.
			m.toast.Show("Playing on "+dev.Name, "", ToastInfo)
			return m, tea.Batch(scheduleAutoDismiss(), m.activateAndRetry(dev.ID, retry))
		}
		return m, m.transferPlayback(dev.ID, dev.Name)
	}
	switch msg.String() {
	case "j", "down":
		m.devices.MoveDown()
	case "k", "up":
		m.devices.MoveUp()
	case "q":
		m.devices = nil
		m.pendingRetry = nil
		m.leaveOverlay()
	}
	return m, nil
}

func (m Model) handleKeyPlaylistPick(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.playlistPick == nil {
		m.mode = ModeNormal
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEscape:
		m.playlistPick = nil
		m.mode = m.pickReturn
		return m, nil
	case tea.KeyEnter:
		pl := m.playlistPick.Selected()
		trackID, trackName := m.playlistPick.TrackID(), m.playlistPick.TrackName()
		batch := m.playlistPick.TrackIDs()
		m.playlistPick = nil
		m.mode = m.pickReturn
		if pl == nil {
			return m, nil
		}
		if batch != nil {
			return m, m.addTracksToPlaylist(*pl, batch)
		}
		return m, m.addToPlaylist(*pl, trackID, trackName)
	}
	var cmd tea.Cmd
	*m.playlistPick, cmd = m.playlistPick.Update(msg)
	return m, cmd
}

func (m Model) handleKeyCommand(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.cmdInput = ""
		m.leaveOverlay()
		return m, nil
	case tea.KeyEnter:
		input := m.cmdInput
		m.cmdInput = ""
		m.leaveOverlay()
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
		return m, m.maybeLoadMore()
	}
	return m, nil
}

func (m Model) handleKeyNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Handle g-prefix motions
	if m.gtracker.Pending() {
		count := m.count
		m.count = 0
		action := m.gtracker.Feed(msg.String())
		switch action {
		case GActionTop:
			switch {
			case m.focusPane == PaneSidebar:
				m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyHome})
			case count > 0:
				m.tracklist.GotoRow(count - 1) // 12gg: row 12
			default:
				m.tracklist.GotoTop()
			}
			return m, m.maybeLoadMore()
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
		case GActionGoAlbum:
			return m.goToAlbumOrArtist(ActionGoAlbum)
		case GActionGoArtist:
			return m.goToAlbumOrArtist(ActionGoArtist)
		default:
			return m, nil
		}
	}

	// A count (5j, 12G, 6]) is typed with digits, in either pane.
	if d, ok := m.countDigit(msg); ok {
		m.count = min(m.count*10+d, maxCount)
		return m, nil
	}
	count := m.count
	m.count = 0

	// "g" starts the g-prefix, "G" goes to bottom immediately
	if msg.String() == "g" {
		m.count = count // carried into 12gg
		m.gtracker.Feed("g")
		return m, nil
	}
	if key.Matches(msg, m.keys.Bottom) {
		switch {
		case m.focusPane == PaneSidebar:
			m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: tea.KeyEnd})
		case count > 0:
			m.tracklist.GotoRow(count - 1) // 12G: row 12
		default:
			m.tracklist.GotoBottom()
		}
		return m, m.maybeLoadMore()
	}

	if msg.Type == tea.KeyEscape {
		// Esc backs out of whatever is half done: a count (already dropped
		// above), a g prefix, a filter left applied.
		m.gtracker.Reset()
		switch {
		case m.focusPane == PaneSidebar && m.sidebar.filterText != "":
			m.sidebar.ClearFilter()
		case m.focusPane == PaneTrackList && m.tracklist.FilterText() != "":
			m.tracklist.ClearFilter()
		}
		return m, nil
	}
	if nm, cmd, ok := m.handleGlobalKey(msg, count); ok {
		nm.gtracker.Reset()
		return nm, cmd
	}

	switch {
	case key.Matches(msg, m.keys.NowPlaying):
		m.gtracker.Reset()
		return m, m.openNowPlaying(npViewLast)

	case key.Matches(msg, m.keys.Quit):
		return m.quit()


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

	case key.Matches(msg, m.keys.Visual):
		if m.focusPane != PaneTrackList || m.tracklist.SelectedTrack() == nil || m.tracklist.HasNotice() {
			return m, nil
		}
		m.gtracker.Reset()
		m.tracklist.StartVisual()
		m.mode = ModeVisual
		return m, nil

	case key.Matches(msg, m.keys.Like):
		if m.focusPane == PaneTrackList {
			track := m.tracklist.SelectedTrack()
			if track == nil || track.IsSeparator || track.IsAlbumRow {
				m.toast.Show("No track selected", "", ToastError)
				return m, scheduleAutoDismiss()
			}
			return m, m.likeToggleCmd(track.ID)
		}
		// f is the selection; the playing track is F.
		m.toast.Show("Pick a track to like", m.keys.LikePlaying.Help().Key+" likes the playing track", ToastInfo)
		return m, scheduleAutoDismiss()

	case key.Matches(msg, m.keys.Actions):
		m.gtracker.Reset()
		return m.openActions()

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

	case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
		key.Matches(msg, m.keys.HalfUp), key.Matches(msg, m.keys.HalfDown):
		if m.focusPane == PaneSidebar {
			// Driven here rather than by the list's own keys, so half pages
			// and rebound keys work in the sidebar too.
			switch {
			case key.Matches(msg, m.keys.Up):
				m.sidebar.Move(-max(1, count))
			case key.Matches(msg, m.keys.Down):
				m.sidebar.Move(max(1, count))
			case key.Matches(msg, m.keys.HalfUp):
				m.sidebar.Move(-m.sidebar.HalfPage())
			case key.Matches(msg, m.keys.HalfDown):
				m.sidebar.Move(m.sidebar.HalfPage())
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Up):
			m.tracklist.MoveUp(max(1, count))
		case key.Matches(msg, m.keys.Down):
			m.tracklist.MoveDown(max(1, count))
		case key.Matches(msg, m.keys.HalfUp):
			m.tracklist.HalfPageUp()
		case key.Matches(msg, m.keys.HalfDown):
			m.tracklist.HalfPageDown()
		}
		return m, m.maybeLoadMore()
	}

	return m, nil
}

const doubleClickThreshold = 400 * time.Millisecond

// wheelScrollLines is how many rows a single mouse-wheel notch scrolls.
const wheelScrollLines = 3

// maybeLoadMore triggers lazy-loading of additional tracks when the
// tracklist cursor approaches the end of the currently loaded set, or while a
// filter is active (a filter has to see every track to be correct). Returns
// nil if no fetch is needed (or one is already in flight).
func (m Model) maybeLoadMore() tea.Cmd {
	if m.pagination == nil || m.pagination.loadingMore ||
		m.pagination.loaded >= m.pagination.total {
		return nil
	}
	if !m.tracklist.Filtering() && m.tracklist.Cursor()+20 < m.pagination.loaded {
		return nil
	}
	m.pagination.loadingMore = true
	return m.fetchMoreTracks()
}

// paneAt returns which pane a given X column falls in. In narrow mode only
// the focused pane is on screen, so every click lands on it.
func (m Model) paneAt(x int) Pane {
	if m.narrow() {
		return m.focusPane
	}
	if x < m.sidebarWidth() {
		return PaneSidebar
	}
	return PaneTrackList
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.mode != ModeNormal {
		return m.handleOverlayMouse(msg)
	}

	contentH := m.height - m.statusRows()

	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}
		if msg.Y >= contentH {
			return m, m.handleStatusClick(msg.X, msg.Y-contentH)
		}
		if m.paneAt(msg.X) == PaneSidebar {
			return m.handleSidebarClick(msg.Y)
		}
		return m.handleTrackListClick(msg.Y)

	case tea.MouseButtonWheelUp:
		return m.handleWheel(msg.X, -wheelScrollLines)

	case tea.MouseButtonWheelDown:
		return m.handleWheel(msg.X, wheelScrollLines)
	}

	return m, nil
}

// handleWheel scrolls the pane under the cursor (not necessarily the focused
// pane) by delta rows. A negative delta scrolls up.
func (m Model) handleWheel(x, delta int) (Model, tea.Cmd) {
	dir := tea.KeyDown
	if delta < 0 {
		dir = tea.KeyUp
		delta = -delta
	}
	if m.paneAt(x) == PaneSidebar {
		for range delta {
			m.sidebar, _ = m.sidebar.Update(tea.KeyMsg{Type: dir})
		}
		return m, nil
	}
	if dir == tea.KeyUp {
		m.tracklist.MoveUp(delta)
	} else {
		m.tracklist.MoveDown(delta)
	}
	return m, m.maybeLoadMore()
}

func (m Model) handleSidebarClick(y int) (Model, tea.Cmd) {
	m.focusPane = PaneSidebar
	m.sidebar.SetCursorFromClick(y)

	now := time.Now()
	if m.lastClickPane == PaneSidebar && now.Sub(m.lastClickTime) < doubleClickThreshold {
		m.lastClickTime = time.Time{}
		if pl := m.sidebar.SelectedPlaylist(); pl != nil {
			return m.openSidebarRow(*pl)
		}
	}
	m.lastClickTime = now
	m.lastClickPane = PaneSidebar
	return m, nil
}

func (m Model) handleTrackListClick(y int) (Model, tea.Cmd) {
	m.focusPane = PaneTrackList
	// Ignore clicks on empty space below the last row.
	if !m.tracklist.SetCursorFromClick(y) {
		return m, nil
	}

	now := time.Now()
	if m.lastClickPane == PaneTrackList && now.Sub(m.lastClickTime) < doubleClickThreshold {
		m.lastClickTime = time.Time{}
		track := m.tracklist.SelectedTrack()
		if track != nil && !track.IsSeparator {
			if cmd, ok := m.openRow(*track); ok {
				return m, cmd
			}
			return m, m.playTrack(track.URI, m.tracklist.ContextURI())
		}
	}
	m.lastClickTime = now
	m.lastClickPane = PaneTrackList
	return m, nil
}

// handleOverlayMouse handles mouse input while an overlay is open: the wheel
// scrolls the overlay's list and a left click closes the overlay (matching its
// Escape behaviour). Per-row click selection is intentionally omitted — the
// popups are centered with variable internal layouts, so pixel-accurate
// hit-testing would couple to each overlay's rendering and be brittle.
func (m Model) handleOverlayMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.scrollOverlay(tea.KeyUp), nil
	case tea.MouseButtonWheelDown:
		return m.scrollOverlay(tea.KeyDown), nil
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionRelease {
			return m.closeOverlay()
		}
	}
	return m, nil
}

// scrollOverlay moves the active overlay's selection by wheelScrollLines in
// the given direction.
func (m Model) scrollOverlay(dir tea.KeyType) Model {
	for range wheelScrollLines {
		switch m.mode {
		case ModeSearch:
			if m.search != nil {
				*m.search, _ = m.search.Update(tea.KeyMsg{Type: dir})
			}
		case ModeActions:
			if m.actions != nil {
				if dir == tea.KeyUp {
					m.actions.MoveUp()
				} else {
					m.actions.MoveDown()
				}
			}
		case ModeDevices:
			if m.devices != nil {
				if dir == tea.KeyUp {
					m.devices.MoveUp()
				} else {
					m.devices.MoveDown()
				}
			}
		case ModePlaylistPick:
			if m.playlistPick != nil {
				if dir == tea.KeyUp {
					m.playlistPick.MoveUp()
				} else {
					m.playlistPick.MoveDown()
				}
			}
		}
	}
	return m
}

// closeOverlay dismisses the current overlay, mirroring its Escape handling.
// Input modes (command/filter) are left untouched.
func (m Model) closeOverlay() (Model, tea.Cmd) {
	switch m.mode {
	case ModeSearch:
		m.search = nil
		m.leaveOverlay()
	case ModeActions:
		m.actions = nil
		m.mode = m.actionsReturn
	case ModeDevices:
		m.devices = nil
		m.pendingRetry = nil
		m.leaveOverlay()
	case ModePlaylistPick:
		m.playlistPick = nil
		m.mode = m.pickReturn
	case ModeHelp:
		m.leaveOverlay()
	case ModeNowPlaying:
		m.mode = ModeNormal
	}
	return m, nil
}

// queuedURI is the URI to play a queue entry by: its own when known (an
// episode's isn't a track URI), else built from its track ID.
func queuedURI(pl source.Playlist) string {
	if pl.URI != "" {
		return pl.URI
	}
	return "spotify:track:" + pl.ID
}

// openSidebarRow opens the sidebar row pl: a playlist or collection opens
// in the track list (and takes focus, so the next Enter plays from it); a
// queue entry plays.
func (m Model) openSidebarRow(pl source.Playlist) (Model, tea.Cmd) {
	if m.sidebar.Section() == SectionQueue {
		return m, m.playTrack(queuedURI(pl), "")
	}
	if isCollection(pl.ID) {
		return m, m.openCollection(pl.ID)
	}
	m.pushNav()
	m.tracklist.SetLoadingFor(pl.Name, pl.URI)
	m.focusPane = PaneTrackList
	return m, m.fetchPlaylistTracks(pl)
}

// goToAlbumOrArtist (ga / gA) opens the album or artist of the selected
// track, or of the playing one when no track is selected (the sidebar has
// focus, or Now Playing is open).
func (m Model) goToAlbumOrArtist(action ActionType) (Model, tea.Cmd) {
	var t *source.Track
	if m.focusPane == PaneTrackList && m.mode != ModeNowPlaying {
		if sel := m.tracklist.SelectedTrack(); sel != nil && !sel.IsSeparator && !sel.IsAlbumRow {
			t = sel
		}
	}
	if t == nil {
		t = m.track
	}
	if t == nil {
		m.toast.Show("No track selected", "", ToastError)
		return m, scheduleAutoDismiss()
	}
	m.mode = ModeNormal
	return m.executeAction(ActionItem{Type: action}, actionSubject{albumID: t.AlbumID, artistID: t.ArtistID})
}

func (m Model) handleEnter() (Model, tea.Cmd) {
	if m.focusPane == PaneSidebar {
		if pl := m.sidebar.SelectedPlaylist(); pl != nil {
			return m.openSidebarRow(*pl)
		}
	} else {
		track := m.tracklist.SelectedTrack()
		if track != nil {
			// Separator rows are not actionable
			if track.IsSeparator {
				return m, nil
			}
			// Album rows navigate to the album page
			if cmd, ok := m.openRow(*track); ok {
				return m, cmd
			}
			return m, m.playTrack(track.URI, m.tracklist.ContextURI())
		}
		// A playlist whose tracks Spotify won't list can still be played whole.
		if m.tracklist.HasNotice() && m.tracklist.ContextURI() != "" {
			return m, m.playPlaylistFromStart(m.tracklist.ContextURI())
		}
	}
	return m, nil
}

// restrictedPlaylistNotice explains an empty playlist view for
// source.ErrPlaylistRestricted.
const restrictedPlaylistNotice = "Spotify doesn't let this app list the tracks of playlists you don't own, " +
	"but you can still play them.\n\n" +
	"Enter  play this playlist\n" +
	"o      more options (open in Spotify)"

// renderNPArt (re)renders the Now Playing art for the current terminal size.
func (m *Model) renderNPArt() {
	if m.npSourceImg == nil {
		return
	}
	// Kitty art costs a transmission, so only place it while Now Playing
	// shows it (vinyl mode draws its own frames); entering the view or
	// leaving vinyl renders it then.
	if artMode == ArtKitty && (m.mode != ModeNowPlaying || m.vinylMode) {
		return
	}
	w, h := npArtSize(m.width, m.height)
	if w == 0 {
		m.npArt = ""
		if artMode == ArtKitty {
			kittyRelease("np")
		}
		return
	}
	m.npArt = renderArt("np", m.npSourceImg, w, h)
}

// Layout breakpoints.
const (
	// narrowWidth is the terminal width below which the two panes stop
	// fitting side by side; the focused pane then takes the full width and
	// h/l/Tab switch between them.
	narrowWidth = 80
	// minTermWidth/minTermHeight are the smallest terminal waxon will try to
	// draw; below that View shows a resize prompt instead of a broken layout.
	minTermWidth  = 40
	minTermHeight = 10
)

// artStatusBar reports whether the bottom area uses the tall layout with
// album art (needs both height and width; narrow terminals use the compact
// two-row bar so nothing wraps).
func (m Model) artStatusBar() bool { return m.height >= MinTermRows && !m.narrow() }

// statusRows is how many rows the bottom (status) area occupies.
func (m Model) statusRows() int {
	if m.artStatusBar() {
		return ArtHeight
	}
	return 2
}

// narrow reports whether the terminal is too narrow for two panes.
func (m Model) narrow() bool { return m.width < narrowWidth }

// tooSmall reports whether the terminal is too small to draw anything useful.
func (m Model) tooSmall() bool { return m.width < minTermWidth || m.height < minTermHeight }

// sidebarWidth is the sidebar's width for the current terminal size.
func (m Model) sidebarWidth() int {
	if m.narrow() {
		return m.width
	}
	return max(20, m.width/4)
}

func (m *Model) layoutResize() {
	if m.width == 0 || m.height == 0 {
		return
	}
	sidebarW := m.sidebarWidth()
	tracklistW := m.width - sidebarW
	if m.narrow() {
		tracklistW = m.width
	}

	// Reserve rows for the bottom area
	contentH := m.height - m.statusRows()

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

// viewTooSmall asks for a bigger terminal instead of drawing a broken layout.
func viewTooSmall(width, height int) string {
	msg := fmt.Sprintf("waxon needs at least %d×%d\n(now %d×%d)", minTermWidth, minTermHeight, width, height)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, StyleDimText.Render(msg))
}

// httpStatusError is satisfied by any error type that carries an HTTP status code.
type httpStatusError interface {
	error
	HTTPStatus() int
}

// isAuthError reports whether an error indicates the session needs re-auth —
// either a 401 from the Spotify API (token revoked server-side) or a failed
// token refresh (refresh token expired/revoked), so the UI can prompt the user
// to run 'waxon auth' instead of showing a cryptic error.
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
	// A failed token refresh surfaces as *oauth2.RetrieveError. Spotify returns
	// 400 invalid_grant when the refresh token is no longer valid; treat that
	// (and a 401) as needing re-auth. 5xx responses are transient and left to
	// normal error backoff.
	var rErr *oauth2.RetrieveError
	if errors.As(err, &rErr) {
		if rErr.ErrorCode == "invalid_grant" {
			return true
		}
		if rErr.Response != nil {
			return rErr.Response.StatusCode == http.StatusBadRequest ||
				rErr.Response.StatusCode == http.StatusUnauthorized
		}
	}
	return false
}

// retryAfterError is satisfied by errors that carry a Retry-After hint.
type retryAfterError interface {
	error
	RetryAfterSeconds() int
}

// isRateLimitError reports whether err is an HTTP 429 from Spotify, from
// either the zmb3 client or waxon's own API path.
func isRateLimitError(err error) bool {
	var sErr spotifyapi.Error
	if errors.As(err, &sErr) && sErr.Status == http.StatusTooManyRequests {
		return true
	}
	var hse httpStatusError
	return errors.As(err, &hse) && hse.HTTPStatus() == http.StatusTooManyRequests
}

// coolingDown reports whether a rate-limit cooldown is still in effect.
func (m Model) coolingDown() bool {
	return time.Now().Before(m.rateLimitedUntil)
}

// noteRateLimit records a rate-limit cooldown if err is a 429, honouring the
// server's Retry-After when it's known.
func (m *Model) noteRateLimit(err error) {
	if !isRateLimitError(err) {
		return
	}
	m.rateLimitHits++
	wait := rateLimitCooldown
	var ra retryAfterError
	if errors.As(err, &ra) && ra.RetryAfterSeconds() > 0 {
		wait = time.Duration(ra.RetryAfterSeconds()) * time.Second
	}
	until := time.Now().Add(wait)
	if until.After(m.rateLimitedUntil) {
		m.rateLimitedUntil = until
	}
}

// rateLimitHint explains a 429 and, when the wait is long or we are on the
// shared client ID, points at the fix (a personal client ID — README).
func (m Model) rateLimitHint(err error) string {
	wait := 0
	var ra retryAfterError
	if errors.As(err, &ra) {
		wait = ra.RetryAfterSeconds()
	}
	var sb strings.Builder
	switch {
	case wait >= 3600:
		fmt.Fprintf(&sb, "Spotify asks to wait %dh", wait/3600)
	case wait >= 90:
		fmt.Fprintf(&sb, "Spotify asks to wait %dm", wait/60)
	case wait > 0:
		fmt.Fprintf(&sb, "Retrying in %ds", wait)
	default:
		sb.WriteString("Pausing updates for a moment")
	}
	if m.opts.SharedClientID && (wait >= 60 || m.rateLimitHits >= 2) {
		sb.WriteString(" · shared app is busy — run 'waxon auth --own' for your own")
	}
	return sb.String()
}

// basePollInterval picks the steady-state poll cadence from what the user can
// actually see change: fast while playing and focused, slow when paused,
// idle, or the terminal is in the background.
func (m Model) basePollInterval() time.Duration {
	switch {
	case m.blurred:
		return pollIntervalBlurred
	case m.track == nil:
		return pollIntervalIdle
	case !m.track.Playing:
		return pollIntervalPaused
	default:
		return pollInterval
	}
}

// backoffInterval returns the next poll delay: the base cadence, doubled per
// consecutive error (capped at 32×), and never earlier than an active
// rate-limit cooldown.
func (m Model) backoffInterval() time.Duration {
	d := m.basePollInterval()
	if m.consecutiveErrors > 0 {
		shift := min(m.consecutiveErrors, 5) // cap at 2^5 = 32x
		d = min(d*time.Duration(1<<shift), maxPollInterval)
	}
	if remaining := time.Until(m.rateLimitedUntil); remaining > d {
		d = remaining
	}
	return d
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.mode == ModeSetup && m.setup != nil {
		return m.setup.View()
	}
	if m.tooSmall() {
		return viewTooSmall(m.width, m.height)
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

	if m.mode == ModePlaylistPick && m.playlistPick != nil {
		return m.playlistPick.View()
	}

	// Help overlay replaces everything
	if m.mode == ModeHelp {
		return viewHelp(m.keys, m.width, m.height, m.helpScroll)
	}

	// Now Playing overlay replaces everything
	if m.mode == ModeNowPlaying {
		artBlock := m.npArt
		if artBlock == "" {
			artBlock = m.albumart.View()
		}
		lyr := npLyrics{
			active:  m.lyricsView,
			lyrics:  m.lyrics,
			line:    m.lyricsLine,
			loading: m.lyricsLoading,
			err:     m.lyricsErr != nil,
		}
		return RenderNowPlaying(m.track, artBlock, m.npSourceImg, m.vinylMode, m.vinylAngle, m.liked, lyr, m.width, m.height)
	}

	m.tracklist.loadingMore = m.loadingMore()

	// Two-pane layout — or just the focused pane when the terminal is narrow.
	var panes string
	if m.narrow() {
		if m.focusPane == PaneSidebar {
			panes = m.sidebar.View(true)
		} else {
			panes = m.tracklist.View(true)
		}
	} else {
		sidebarView := m.sidebar.View(m.focusPane == PaneSidebar)
		tracklistView := m.tracklist.View(m.focusPane == PaneTrackList)
		panes = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, tracklistView)
	}

	// Now-playing area
	if m.mode == ModeVisual {
		m.statusbar.visualHint = m.visualHint()
	}
	if m.count > 0 {
		m.statusbar.pending = strconv.Itoa(m.count)
	}
	show := volumeShow{level: m.volume, accepted: m.volumeAccepted, pending: m.volumeInFlight, glow: m.volumeGlow}
	m.statusbar.volume, m.toast.volume = show, show
	var view string
	if m.artStatusBar() {
		artView := m.albumart.View()
		if artView == "" {
			artView = PlaceholderArt(ArtWidth, ArtHeight)
		}
		nowPlayingArea := m.statusbar.ViewNowPlayingWithArt(m.track, m.shuffleOn, m.repeatMode, m.liked, artView, m.volume, m.deviceName, m.mode, m.cmdInput, m.filterInput, m.tracklist.FilterText())
		view = lipgloss.JoinVertical(lipgloss.Left, panes, nowPlayingArea)
	} else {
		nowPlaying := m.statusbar.ViewNowPlaying(m.track, m.shuffleOn, m.repeatMode, m.liked)
		modeLine := m.statusbar.ViewModeLine(m.mode, m.cmdInput, m.filterInput, m.tracklist.FilterText(), m.volume, m.deviceName)
		view = lipgloss.JoinVertical(lipgloss.Left, panes, nowPlaying, modeLine)
	}

	// Overlay floating toast
	if m.toast.Visible() {
		view = m.toast.Overlay(view, m.width)
	}

	return view
}
