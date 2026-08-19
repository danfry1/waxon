package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

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
	Like     key.Binding
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
		Like:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "favourite")),
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
		{k.AddQueue, k.Like, k.Actions, k.Devices, k.Filter, k.Search, k.Command, k.Help, k.NowPlaying, k.Quit},
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

// gMotion is a two-key g-prefix motion: the second key, the action it
// resolves to, and its help text. This table is the single source of truth
// for both GTracker and the help overlay.
type gMotion struct {
	Key    string
	Action GAction
	Desc   string
}

// gMotions lists every supported g-prefix motion in help-display order.
var gMotions = []gMotion{
	{"g", GActionTop, "top"},
	{"l", GActionLibrary, "library"},
	{"q", GActionQueue, "queue"},
	{"c", GActionCurrent, "jump to playing"},
	{"r", GActionRecent, "recently played"},
}

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
		}
		return GActionNone
	}
	// We have a pending "g" — resolve the second key.
	t.pending = false
	for _, m := range gMotions {
		if m.Key == k {
			return m.Action
		}
	}
	return GActionNone
}

// Pending returns whether the tracker is waiting for a second key after "g".
func (t *GTracker) Pending() bool {
	return t.pending
}

// Reset clears any pending state.
func (t *GTracker) Reset() {
	t.pending = false
}

// keyBindingFields maps config action names to KeyMap fields. Names are what
// users write in config.json under "keys". The g-prefix table and Esc (which
// closes every overlay/mode) are fixed and not listed here.
func (k *KeyMap) keyBindingFields() map[string]*key.Binding {
	return map[string]*key.Binding{
		"up": &k.Up, "down": &k.Down, "bottom": &k.Bottom,
		"half_up": &k.HalfUp, "half_down": &k.HalfDown,
		"focus_left": &k.FocusLeft, "focus_right": &k.FocusRight, "cycle_pane": &k.CyclePane,
		"enter": &k.Enter, "play_pause": &k.PlayPause, "next": &k.Next, "prev": &k.Prev,
		"seek_fwd": &k.SeekFwd, "seek_back": &k.SeekBack,
		"add_queue": &k.AddQueue, "like": &k.Like, "actions": &k.Actions, "devices": &k.Devices,
		"back": &k.Back, "filter": &k.Filter, "search": &k.Search, "command": &k.Command,
		"help": &k.Help, "now_playing": &k.NowPlaying, "quit": &k.Quit,
		"section1": &k.Section1, "section2": &k.Section2,
	}
}

// KeyActionNames returns the configurable action names, sorted.
func KeyActionNames() []string {
	k := DefaultKeyMap()
	fields := k.keyBindingFields()
	names := make([]string, 0, len(fields))
	for n := range fields {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// KeyMapFromOverrides returns the default KeyMap with the given overrides
// applied. Each value is a comma-separated list of keys as Bubbletea names
// them ("j", "down", "ctrl+d", "space", "enter", "esc", "tab", "backspace").
// Unknown action names or empty key lists are errors so typos don't silently
// leave a default in place.
func KeyMapFromOverrides(overrides map[string]string) (KeyMap, error) {
	k := DefaultKeyMap()
	fields := k.keyBindingFields()
	for action, spec := range overrides {
		b, ok := fields[action]
		if !ok {
			return k, fmt.Errorf("unknown key action %q (available: %s)", action, strings.Join(KeyActionNames(), ", "))
		}
		var keys []string
		for _, part := range strings.Split(spec, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.EqualFold(part, "space") {
				part = " "
			}
			keys = append(keys, part)
		}
		if len(keys) == 0 {
			return k, fmt.Errorf("key action %q has no keys", action)
		}
		desc := b.Help().Desc
		*b = key.NewBinding(key.WithKeys(keys...), key.WithHelp(displayKey(keys[0]), desc))
	}
	return k, nil
}

// displayKey renders a Bubbletea key name the way the help text shows it.
func displayKey(k string) string {
	switch k {
	case " ":
		return "space"
	case "ctrl+u":
		return "C-u"
	case "ctrl+d":
		return "C-d"
	}
	return strings.Replace(k, "ctrl+", "C-", 1)
}
