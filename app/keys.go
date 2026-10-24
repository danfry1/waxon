package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
	VolUp     key.Binding
	VolDown   key.Binding
	Shuffle   key.Binding
	Repeat    key.Binding

	// Actions
	Visual   key.Binding
	AddQueue key.Binding
	Like     key.Binding
	Actions  key.Binding
	// LikePlaying and ActionsPlaying act on the playing track from anywhere
	// (Shift: the playing track; lowercase: the selection).
	LikePlaying    key.Binding
	ActionsPlaying key.Binding
	Mute           key.Binding
	Devices  key.Binding
	Mode     key.Binding // play in waxon ⇄ control the Spotify app

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
		VolUp:     key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "volume up")),
		VolDown:   key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "volume down")),
		Shuffle:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "shuffle")),
		Repeat:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat")),

		Visual:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "visual select")),
		AddQueue: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add to queue")),
		Like:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "favourite")),
		Actions:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "actions")),

		LikePlaying:    key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "like the playing track")),
		ActionsPlaying: key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "actions for the playing track")),
		Mute:           key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mute")),
		Devices:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "devices")),
		Mode:     key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "play in waxon ⇄ control the Spotify app")),

		Back: key.NewBinding(key.WithKeys("backspace", "b"), key.WithHelp("⌫/b", "back")),

		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Search:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "search")),
		Command:    key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		NowPlaying: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "now playing")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Escape:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close/cancel")),

	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.PlayPause, k.Filter, k.Search, k.Command, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom, k.HalfUp, k.HalfDown},
		{k.FocusLeft, k.FocusRight, k.CyclePane, k.Back},
		{k.Enter, k.PlayPause, k.Next, k.Prev, k.SeekFwd, k.SeekBack},
		{k.AddQueue, k.Like, k.Actions, k.Devices, k.Filter, k.Search, k.Command, k.Help, k.NowPlaying, k.Quit},
	}
}

// GAction identifies the action from a g-prefix key combo.
type GAction int

const (
	GActionNone     GAction = iota
	GActionTop              // gg — go to top
	GActionLibrary          // gl — focus library
	GActionQueue            // gq — focus queue
	GActionCurrent          // gc — jump to currently playing track
	GActionGoAlbum          // ga — album of the selected (or playing) track
	GActionGoArtist         // gA — its artist
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
	{"a", GActionGoAlbum, "album of the track"},
	{"A", GActionGoArtist, "artist of the track"},
}

// GTracker tracks g-prefix two-key motions (see gMotions).
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

// maxCount caps a typed count prefix.
const maxCount = 9999

// countDigit reports whether msg extends a count prefix: 1-9, or 0 once a
// count has started. A digit the user has bound to another action (other
// than the section keys a count replaces) keeps that action.
func (m Model) countDigit(msg tea.KeyMsg) (int, bool) {
	s := msg.String()
	if len(s) != 1 || s[0] < '0' || s[0] > '9' || (s[0] == '0' && m.count == 0) {
		return 0, false
	}
	k := m.keys
	for _, b := range k.keyBindingFields() {
		if key.Matches(msg, *b) {
			return 0, false
		}
	}
	return int(s[0] - '0'), true
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
		"vol_up": &k.VolUp, "vol_down": &k.VolDown, "shuffle": &k.Shuffle, "repeat": &k.Repeat,
		"visual": &k.Visual, "add_queue": &k.AddQueue, "like": &k.Like, "actions": &k.Actions, "devices": &k.Devices, "mode": &k.Mode,
		"like_playing": &k.LikePlaying, "actions_playing": &k.ActionsPlaying, "mute": &k.Mute,
		"back": &k.Back, "filter": &k.Filter, "search": &k.Search, "command": &k.Command,
		"help": &k.Help, "now_playing": &k.NowPlaying, "quit": &k.Quit,
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
// (action name → comma-separated keys) applied. Every override that can be
// applied is; the error lists the rest (unknown actions, empty key lists)
// and keys an override shares with another action, which would shadow one
// of them, so one typo doesn't throw away the whole keymap.
func KeyMapFromOverrides(overrides map[string]string) (KeyMap, error) {
	k := DefaultKeyMap()
	fields := k.keyBindingFields()
	var problems []string
	actions := make([]string, 0, len(overrides))
	for action := range overrides {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	for _, action := range actions {
		b, ok := fields[action]
		if !ok {
			if now, gone := retiredActions[action]; gone {
				problems = append(problems, fmt.Sprintf("%q is gone: %s", action, now))
			} else {
				problems = append(problems, fmt.Sprintf("unknown action %q", action))
			}
			continue
		}
		var keys []string
		for _, part := range strings.Split(overrides[action], ",") {
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
			problems = append(problems, fmt.Sprintf("%q has no keys", action))
			continue
		}
		desc := b.Help().Desc
		*b = key.NewBinding(key.WithKeys(keys...), key.WithHelp(displayKey(keys[0]), desc))
	}
	problems = append(problems, keyClashes(fields, overrides)...)
	if len(problems) > 0 {
		return k, errors.New(strings.Join(problems, "; "))
	}
	return k, nil
}

// retiredActions are action names that no longer exist, with what does
// the job now, so an old config says what to change instead of failing.
var retiredActions = map[string]string{
	"section1": "gl goes to the Library",
	"section2": "gq goes to the Queue",
}

// keyClashes lists keys that an overridden action shares with another
// action: whichever is matched first wins, and the other stops working.
func keyClashes(fields map[string]*key.Binding, overrides map[string]string) []string {
	owners := map[string][]string{}
	for action, b := range fields {
		for _, kk := range b.Keys() {
			owners[kk] = append(owners[kk], action)
		}
	}
	var clashes []string
	for kk, actions := range owners {
		if len(actions) < 2 {
			continue
		}
		sort.Strings(actions)
		for _, a := range actions {
			if _, ok := overrides[a]; ok {
				clashes = append(clashes, fmt.Sprintf("%q is both %s", displayKey(kk), strings.Join(actions, " and ")))
				break
			}
		}
	}
	sort.Strings(clashes)
	return clashes
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
