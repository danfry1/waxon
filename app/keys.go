package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	HalfUp   key.Binding
	HalfDown key.Binding

	// Pane
	FocusLeft  key.Binding
	FocusRight key.Binding
	CyclePane  key.Binding

	// Playback
	Enter     key.Binding
	PlayPause key.Binding
	Next      key.Binding
	Prev      key.Binding
	SeekFwd   key.Binding
	SeekBack  key.Binding

	// Actions
	AddQueue key.Binding
	Actions  key.Binding
	Devices  key.Binding

	// Navigation history
	Back key.Binding

	// Modes
	Filter     key.Binding
	Search     key.Binding
	Command    key.Binding
	Help       key.Binding
	NowPlaying key.Binding
	Quit       key.Binding
	Escape     key.Binding

	// Sections
	Section1 key.Binding
	Section2 key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		HalfUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("C-u", "half page up")),
		HalfDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("C-d", "half page down")),

		FocusLeft:  key.NewBinding(key.WithKeys("h", "H"), key.WithHelp("h", "focus left")),
		FocusRight: key.NewBinding(key.WithKeys("l", "L"), key.WithHelp("l", "focus right")),
		CyclePane:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cycle pane")),

		Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play")),
		PlayPause: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		Next:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next track")),
		Prev:      key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev track")),
		SeekFwd:   key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "seek +5s")),
		SeekBack:  key.NewBinding(key.WithKeys("["), key.WithHelp("[", "seek -5s")),

		AddQueue: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add to queue")),
		Actions:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "actions")),
		Devices:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "devices")),

		Back: key.NewBinding(key.WithKeys("backspace", "b"), key.WithHelp("⌫/b", "back")),

		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Search:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "search")),
		Command:    key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		NowPlaying: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "now playing")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Escape:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close/cancel")),

		Section1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "library")),
		Section2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "queue")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.PlayPause, k.Filter, k.Search, k.Command, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom, k.HalfUp, k.HalfDown},
		{k.FocusLeft, k.FocusRight, k.CyclePane, k.Section1, k.Section2, k.Back},
		{k.Enter, k.PlayPause, k.Next, k.Prev, k.SeekFwd, k.SeekBack},
		{k.AddQueue, k.Actions, k.Devices, k.Filter, k.Search, k.Command, k.Help, k.NowPlaying, k.Quit},
	}
}

// GAction identifies the action from a g-prefix key combo.
type GAction int

const (
	GActionNone    GAction = iota
	GActionTop             // gg — go to top
	GActionLibrary         // gl — focus library
	GActionQueue           // gq — focus queue
	GActionCurrent         // gc — jump to currently playing track
	GActionRecent          // gr — load recently played
)

// GTracker tracks g-prefix two-key motions (gg, gl, gq, gc, gr).
type GTracker struct {
	pending bool
}

// Feed processes a key press. Returns the resolved GAction.
// When "g" is pressed the first time it returns GActionNone (pending).
// A second key resolves the action. Any unrecognised second key resets and
// returns GActionNone.
func (t *GTracker) Feed(k string) GAction {
	if !t.pending {
		if k == "g" {
			t.pending = true
			return GActionNone
		}
		return GActionNone
	}
	// We have a pending "g" — resolve the second key.
	t.pending = false
	switch k {
	case "g":
		return GActionTop
	case "l":
		return GActionLibrary
	case "q":
		return GActionQueue
	case "c":
		return GActionCurrent
	case "r":
		return GActionRecent
	default:
		return GActionNone
	}
}

// Pending returns whether the tracker is waiting for a second key after "g".
func (t *GTracker) Pending() bool {
	return t.pending
}

// Reset clears any pending state.
func (t *GTracker) Reset() {
	t.pending = false
}
