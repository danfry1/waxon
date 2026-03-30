package app

import (
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/source"
	"github.com/danielfry/spotui/visual"
)

const (
	numBars     = 40
	barMaxH     = 16
	animFPS     = 30
	pollSeconds = 1.5
)

type animTickMsg time.Time
type pollTickMsg time.Time
type trackUpdateMsg struct{ track *source.Track }
type trackErrorMsg struct{ err error }
type controlDoneMsg struct{}
type artworkMsg struct {
	url    string
	result visual.ArtworkResult
	cols   int
	rows   int
}
type audioFeaturesMsg struct{ features *source.AudioFeatures }

type Model struct {
	source     source.TrackSource
	track      *source.Track
	mood       mood.Mood
	targetMood mood.Mood
	transition *mood.Transition
	bars       [numBars]float64
	barVels    [numBars]float64
	barTargets [numBars]float64
	barSprings [numBars]harmonica.Spring
	beatPhase  float64 // 0.0-1.0, cycles at estimated BPM
	pattern    int
	width      int
	height     int
	artworkURL      string
	artworkRendered string
	artworkIsKitty  bool
	artworkCols     int
	artworkRows     int
	help            help.Model
	showHelp   bool
	keys       KeyMap
	quitting   bool
	lastPoll   time.Time

	effects       Effects
	panel         *Panel
	activePanel   PanelType
	richSource    source.RichSource
	volume        int
	shuffleOn     bool
	repeatMode    source.RepeatMode
	audioFeatures *source.AudioFeatures
}

func NewModel(src source.TrackSource) Model {
	m := Model{
		source: src, mood: mood.Idle, targetMood: mood.Idle,
		keys: DefaultKeyMap(), help: help.New(),
		volume: 50, repeatMode: source.RepeatOff,
	}
	if rich, ok := src.(source.RichSource); ok {
		m.richSource = rich
	}
	for i := range numBars {
		m.barSprings[i] = harmonica.NewSpring(harmonica.FPS(animFPS), 8.0, 0.6)
		m.barTargets[i] = rand.Float64() * 0.3
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg { return animTickMsg(t) }),
		tea.Tick(time.Duration(pollSeconds*float64(time.Second)), func(t time.Time) tea.Msg { return pollTickMsg(t) }),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.effects = NewEffects(msg.Width, msg.Height)
		if m.panel != nil {
			m.panel.Resize(msg.Width, msg.Height)
		}
		return m, nil
	case tea.KeyMsg:
		if m.activePanel != PanelNone {
			return m.updatePanel(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.PlayPause):
			if m.track != nil && m.track.Playing {
				return m, controlCmd(m.source.Pause)
			}
			return m, controlCmd(m.source.Play)
		case key.Matches(msg, m.keys.Next):
			return m, controlCmd(m.source.Next)
		case key.Matches(msg, m.keys.Prev):
			return m, controlCmd(m.source.Previous)
		case msg.Type == tea.KeyLeft:
			return m, m.seekRelative(-5 * time.Second)
		case msg.Type == tea.KeyRight:
			return m, m.seekRelative(5 * time.Second)
		case key.Matches(msg, m.keys.Queue):
			return m.togglePanel(PanelQueue)
		case key.Matches(msg, m.keys.Library):
			return m.togglePanel(PanelLibrary)
		case key.Matches(msg, m.keys.Search):
			return m.togglePanel(PanelSearch)
		case key.Matches(msg, m.keys.Devices):
			return m.togglePanel(PanelDevices)
		case key.Matches(msg, m.keys.VolumeUp):
			return m.adjustVolume(5)
		case key.Matches(msg, m.keys.VolumeDown):
			return m.adjustVolume(-5)
		case key.Matches(msg, m.keys.Shuffle):
			return m.toggleShuffle()
		case key.Matches(msg, m.keys.Repeat):
			return m.cycleRepeat()
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m, m.handleClick(msg.X, msg.Y)
		}
	case animTickMsg:
		m.tickAnimation()
		return m, tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg { return animTickMsg(t) })
	case pollTickMsg:
		return m, tea.Batch(
			fetchTrack(m.source),
			tea.Tick(time.Duration(pollSeconds*float64(time.Second)), func(t time.Time) tea.Msg { return pollTickMsg(t) }),
		)
	case trackUpdateMsg:
		cmd := m.handleTrackUpdate(msg.track)
		return m, cmd
	case artworkMsg:
		if msg.url == m.artworkURL {
			m.artworkRendered = msg.result.Rendered
			m.artworkIsKitty = msg.result.IsKitty
			m.artworkCols = msg.cols
			m.artworkRows = msg.rows
			// Override mood colors with album art's dominant colors
			if msg.result.HasColors {
				m.mood.Primary = msg.result.Primary
				m.mood.Secondary = msg.result.Secondary
				m.mood.Background = msg.result.Background
				m.targetMood.Primary = msg.result.Primary
				m.targetMood.Secondary = msg.result.Secondary
				m.targetMood.Background = msg.result.Background
			}
		}
		return m, nil
	case audioFeaturesMsg:
		m.audioFeatures = msg.features
		if msg.features != nil {
			detected := mood.DetectFromFeatures(msg.features)
			if detected.Name != m.targetMood.Name {
				m.startTransitionTo(detected)
			}
		}
		return m, nil
	case queueLoadedMsg:
		if m.panel != nil && m.panel.Type == PanelQueue {
			items := make([]list.Item, len(msg.tracks))
			for i, t := range msg.tracks {
				items[i] = trackItem{track: t}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case playlistsLoadedMsg:
		if m.panel != nil && m.panel.Type == PanelLibrary {
			m.panel.playlists = msg.playlists
			items := make([]list.Item, len(msg.playlists))
			for i, p := range msg.playlists {
				items[i] = playlistItem{playlist: p}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case playlistTracksMsg:
		if m.panel != nil && m.panel.Type == PanelLibrary && m.panel.inPlaylist {
			items := make([]list.Item, len(msg.tracks))
			for i, t := range msg.tracks {
				items[i] = trackItem{track: t}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case devicesLoadedMsg:
		if m.panel != nil && m.panel.Type == PanelDevices {
			items := make([]list.Item, len(msg.devices))
			for i, d := range msg.devices {
				items[i] = deviceItem{device: d}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case searchResultsMsg:
		if m.panel != nil && m.panel.Type == PanelSearch && msg.results != nil {
			items := make([]list.Item, len(msg.results.Tracks))
			for i, t := range msg.results.Tracks {
				items[i] = trackItem{track: t}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case trackErrorMsg:
		m.track = nil
		m.startTransitionTo(mood.Idle)
		return m, nil
	case controlDoneMsg:
		return m, nil
	}
	return m, nil
}

// estimateBPM returns a plausible tempo based on mood energy.
// Low energy (ambient) ≈ 70 BPM, high energy (electronic) ≈ 140 BPM.
func estimateBPM(energy float64) float64 {
	return 70 + energy*70
}

func (m *Model) tickAnimation() {
	m.pattern++
	playing := m.track != nil && m.track.Playing
	energy := m.mood.Energy

	if playing {
		// Advance beat phase based on estimated BPM
		bpm := estimateBPM(energy)
		beatsPerFrame := bpm / 60.0 / animFPS
		m.beatPhase += beatsPerFrame
		if m.beatPhase >= 1.0 {
			m.beatPhase -= 1.0
		}

		// On the beat (phase near 0): push a wave of energy through the bars
		onBeat := m.beatPhase < 0.08
		// On the off-beat (half beat): subtle secondary pulse
		onOffBeat := m.beatPhase > 0.48 && m.beatPhase < 0.54

		for i := range numBars {
			m.bars[i], m.barVels[i] = m.barSprings[i].Update(m.bars[i], m.barVels[i], m.barTargets[i])

			if onBeat {
				// Beat hit: most bars surge, with spatial variation for natural look
				// Center bars react more, edges react less
				centerDist := math.Abs(float64(i)-float64(numBars)/2) / (float64(numBars) / 2)
				intensity := (1.0 - centerDist*0.5) * (0.5 + energy*0.5)
				m.barTargets[i] = intensity * (0.6 + rand.Float64()*0.4)
			} else if onOffBeat && energy > 0.4 {
				// Off-beat: gentler pulse for higher-energy tracks
				if rand.Float64() < 0.4 {
					m.barTargets[i] = rand.Float64() * energy * 0.5
				}
			} else {
				// Between beats: gentle decay with occasional random variation
				if rand.Float64() < 0.03+energy*0.05 {
					m.barTargets[i] = rand.Float64() * energy * 0.35
				}
			}
		}
	} else {
		// Paused: decay to zero
		for i := range numBars {
			m.bars[i], m.barVels[i] = m.barSprings[i].Update(m.bars[i], m.barVels[i], m.barTargets[i])
			m.barTargets[i] = 0
		}
	}

	if m.transition != nil {
		m.transition.Tick()
		m.mood = m.transition.Current()
		if m.transition.Done() {
			m.mood = m.targetMood
			m.transition = nil
		}
	}
	// Estimate position between polls
	if m.track != nil && m.track.Playing {
		m.track.Position += time.Second / animFPS
		if m.track.Position > m.track.Duration {
			m.track.Position = m.track.Duration
		}
	}

	m.effects.Tick(energy, m.beatPhase, m.mood.Primary, m.mood.Secondary, m.mood.Background, m.artworkCols, m.artworkRows, m.width, m.height)
}

func (m *Model) handleTrackUpdate(track *source.Track) tea.Cmd {
	m.track = track
	if track == nil {
		m.startTransitionTo(mood.Idle)
		m.artworkURL = ""
		m.artworkRendered = ""
		m.audioFeatures = nil
		return nil
	}
	detected := mood.DetectMood(track.Artist, track.Name, track.Album)
	if detected.Name != m.targetMood.Name {
		m.startTransitionTo(detected)
	}

	var cmds []tea.Cmd

	// Fetch audio features if available
	if m.richSource != nil && track.ID != "" {
		rich := m.richSource
		id := track.ID
		cmds = append(cmds, func() tea.Msg {
			features, err := rich.AudioFeatures(id)
			if err != nil {
				return audioFeaturesMsg{nil}
			}
			return audioFeaturesMsg{features}
		})
	}

	// Fetch artwork async if URL changed
	if track.ArtworkURL != "" && track.ArtworkURL != m.artworkURL {
		m.artworkURL = track.ArtworkURL
		m.artworkRendered = ""
		m.artworkIsKitty = false
		// Scale art to terminal: use up to 60% of height, maintain square aspect
		artH := max(12, min(35, (m.height-16)*3/5))
		artW := artH * 2 // 2:1 ratio for square appearance in terminal
		url := track.ArtworkURL
		cmds = append(cmds, func() tea.Msg {
			result := visual.FetchAndRender(url, artW, artH)
			return artworkMsg{url: url, result: result, cols: artW, rows: artH}
		})
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) startTransitionTo(target mood.Mood) {
	if m.mood.Name == target.Name {
		return
	}
	m.targetMood = target
	m.transition = mood.NewTransition(m.mood, target)
}

func (m *Model) handleClick(x, y int) tea.Cmd {
	if m.track == nil {
		return nil
	}

	// Compute layout to find progress bar and controls positions
	artH := 0
	if m.artworkRendered != "" {
		if m.artworkIsKitty {
			artH = m.artworkRows + 1
		} else {
			artH = strings.Count(m.artworkRendered, "\n") + 1
		}
	}

	// Count content lines before progress:
	// mood(1) + empty(1) + [art(artH) + empty(1) if art] + labels(3) + empty(1)
	linesBeforeProgress := 6 // mood + empty + labels(3) + empty
	if artH > 0 {
		linesBeforeProgress += artH + 1 // art lines + empty after art
	}
	totalContentH := linesBeforeProgress + 2 // + progress + controls
	topPad := max(0, (m.height-totalContentH-4)/2)

	progressRow := 2 + topPad + linesBeforeProgress // 2 = top glow
	controlsRow := progressRow + 1

	// Progress bar dimensions
	innerWidth := m.width - 4
	progressWidth := min(m.width-24, 50)
	barWidth := progressWidth - 14
	barStartCol := 2 + (innerWidth-progressWidth)/2
	barEndCol := barStartCol + barWidth

	// Click on progress bar → seek (±1 row tolerance)
	if y >= progressRow-1 && y <= progressRow+1 && x >= barStartCol && x <= barEndCol && barWidth > 0 {
		ratio := float64(x-barStartCol) / float64(barWidth)
		pos := time.Duration(float64(m.track.Duration) * ratio)
		return m.seekTo(pos)
	}

	// Click on controls (±1 row tolerance)
	if y >= controlsRow-1 && y <= controlsRow+1 {
		center := m.width / 2
		if x < center-5 {
			return controlCmd(m.source.Previous)
		} else if x > center+5 {
			return controlCmd(m.source.Next)
		} else {
			if m.track.Playing {
				return controlCmd(m.source.Pause)
			}
			return controlCmd(m.source.Play)
		}
	}

	return nil
}

func (m *Model) seekTo(pos time.Duration) tea.Cmd {
	if m.track == nil {
		return nil
	}
	pos = max(0, min(pos, m.track.Duration))
	m.track.Position = pos
	src := m.source
	return func() tea.Msg {
		if err := src.Seek(pos); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}

func (m *Model) seekRelative(delta time.Duration) tea.Cmd {
	if m.track == nil {
		return nil
	}
	return m.seekTo(m.track.Position + delta)
}

func fetchTrack(src source.TrackSource) tea.Cmd {
	return func() tea.Msg {
		track, err := src.CurrentTrack()
		if err != nil {
			return trackErrorMsg{err}
		}
		return trackUpdateMsg{track}
	}
}

func controlCmd(fn func() error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}

func (m Model) togglePanel(pt PanelType) (tea.Model, tea.Cmd) {
	if m.activePanel == pt {
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	}
	if m.richSource == nil {
		return m, nil
	}
	p := NewPanel(pt, m.width, m.height)
	m.panel = &p
	m.activePanel = pt

	var cmd tea.Cmd
	switch pt {
	case PanelQueue:
		cmd = fetchQueue(m.richSource)
	case PanelLibrary:
		cmd = fetchPlaylists(m.richSource)
	case PanelDevices:
		cmd = fetchDevices(m.richSource)
	}
	return m, cmd
}

func (m Model) updatePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.panel == nil {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, m.keys.Close):
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	case key.Matches(msg, m.keys.Select):
		return m.panelSelect()
	case key.Matches(msg, m.keys.Back):
		if m.panel.Type == PanelLibrary && m.panel.inPlaylist {
			m.panel.inPlaylist = false
			m.panel.playlistID = ""
			m.panel.playlistName = ""
			items := make([]list.Item, len(m.panel.playlists))
			for i, p := range m.panel.playlists {
				items[i] = playlistItem{playlist: p}
			}
			m.panel.SetItems(items)
			m.panel.List.Title = "Library"
			return m, nil
		}
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	}

	// Handle search input
	if m.panel.Type == PanelSearch {
		var cmd tea.Cmd
		m.panel.Search, cmd = m.panel.Search.Update(msg)
		// Trigger search on enter within the search field
		if key.Matches(msg, m.keys.Select) {
			query := m.panel.Search.Value()
			if query != "" && m.richSource != nil {
				return m, doSearch(m.richSource, query)
			}
		}
		if cmd != nil {
			return m, cmd
		}
	}

	// Delegate list navigation
	var cmd tea.Cmd
	m.panel.List, cmd = m.panel.List.Update(msg)
	return m, cmd
}

func (m Model) panelSelect() (tea.Model, tea.Cmd) {
	if m.panel == nil || m.richSource == nil {
		return m, nil
	}
	selected := m.panel.List.SelectedItem()
	if selected == nil {
		// For search panel, trigger search on enter when no item is selected
		if m.panel.Type == PanelSearch {
			query := m.panel.Search.Value()
			if query != "" {
				return m, doSearch(m.richSource, query)
			}
		}
		return m, nil
	}

	switch item := selected.(type) {
	case trackItem:
		// Close panel and play the selected track (by seeking to it via queue)
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	case playlistItem:
		// Drill into playlist
		m.panel.inPlaylist = true
		m.panel.playlistID = item.playlist.ID
		m.panel.playlistName = item.playlist.Name
		m.panel.List.Title = item.playlist.Name
		return m, fetchPlaylistTracks(m.richSource, item.playlist.ID)
	case deviceItem:
		// Transfer playback to selected device
		m.activePanel = PanelNone
		m.panel = nil
		rich := m.richSource
		devID := item.device.ID
		return m, func() tea.Msg {
			if err := rich.TransferPlayback(devID); err != nil {
				return trackErrorMsg{err}
			}
			return controlDoneMsg{}
		}
	}
	return m, nil
}

func (m Model) adjustVolume(delta int) (tea.Model, tea.Cmd) {
	m.volume += delta
	if m.volume < 0 {
		m.volume = 0
	}
	if m.volume > 100 {
		m.volume = 100
	}
	if m.richSource == nil {
		return m, nil
	}
	rich := m.richSource
	vol := m.volume
	return m, func() tea.Msg {
		if err := rich.SetVolume(vol); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}

func (m Model) toggleShuffle() (tea.Model, tea.Cmd) {
	m.shuffleOn = !m.shuffleOn
	if m.richSource == nil {
		return m, nil
	}
	rich := m.richSource
	state := m.shuffleOn
	return m, func() tea.Msg {
		if err := rich.SetShuffle(state); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}

func (m Model) cycleRepeat() (tea.Model, tea.Cmd) {
	switch m.repeatMode {
	case source.RepeatOff:
		m.repeatMode = source.RepeatContext
	case source.RepeatContext:
		m.repeatMode = source.RepeatTrack
	default:
		m.repeatMode = source.RepeatOff
	}
	if m.richSource == nil {
		return m, nil
	}
	rich := m.richSource
	mode := m.repeatMode
	return m, func() tea.Msg {
		if err := rich.SetRepeat(mode); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}
