package app

import (
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/source"
)

const (
	numBars     = 20
	barMaxH     = 6
	animFPS     = 30
	pollSeconds = 1.5
)

type animTickMsg time.Time
type pollTickMsg time.Time
type trackUpdateMsg struct{ track *source.Track }
type trackErrorMsg struct{ err error }
type controlDoneMsg struct{}

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
	pattern    int
	width      int
	height     int
	help       help.Model
	showHelp   bool
	keys       KeyMap
	quitting   bool
	lastPoll   time.Time
}

func NewModel(src source.TrackSource) Model {
	m := Model{
		source: src, mood: mood.Idle, targetMood: mood.Idle,
		keys: DefaultKeyMap(), help: help.New(),
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
		return m, nil
	case tea.KeyMsg:
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
		m.handleTrackUpdate(msg.track)
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

func (m *Model) tickAnimation() {
	m.pattern++
	energy := m.mood.Energy
	for i := range numBars {
		m.bars[i], m.barVels[i] = m.barSprings[i].Update(m.bars[i], m.barVels[i], m.barTargets[i])
		if rand.Float64() < energy*0.15 {
			m.barTargets[i] = rand.Float64() * (0.3 + energy*0.7)
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
}

func (m *Model) handleTrackUpdate(track *source.Track) {
	m.track = track
	if track == nil {
		m.startTransitionTo(mood.Idle)
		return
	}
	detected := mood.DetectMood(track.Artist, track.Name, track.Album)
	if detected.Name != m.targetMood.Name {
		m.startTransitionTo(detected)
	}
}

func (m *Model) startTransitionTo(target mood.Mood) {
	if m.mood.Name == target.Name {
		return
	}
	m.targetMood = target
	m.transition = mood.NewTransition(m.mood, target)
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
	return func() tea.Msg { fn(); return controlDoneMsg{} }
}
