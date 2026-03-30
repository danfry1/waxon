package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	PlayPause  key.Binding
	Next       key.Binding
	Prev       key.Binding
	VolumeUp   key.Binding
	VolumeDown key.Binding
	Shuffle    key.Binding
	Repeat     key.Binding
	Queue      key.Binding
	Library    key.Binding
	Search     key.Binding
	Devices    key.Binding
	Help       key.Binding
	Quit       key.Binding
	Close      key.Binding
	Select     key.Binding
	Up         key.Binding
	Down       key.Binding
	Back       key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		PlayPause:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		Next:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next")),
		Prev:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev")),
		VolumeUp:   key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "volume up")),
		VolumeDown: key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "volume down")),
		Shuffle:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shuffle")),
		Repeat:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat")),
		Queue:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "queue")),
		Library:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "library")),
		Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Devices:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "devices")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Close:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		Select:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Up:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Back:       key.NewBinding(key.WithKeys("backspace"), key.WithHelp("←", "back")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.Next, k.Prev, k.Queue, k.Library, k.Search, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.PlayPause, k.Next, k.Prev, k.VolumeUp, k.VolumeDown},
		{k.Shuffle, k.Repeat, k.Queue, k.Library, k.Search, k.Devices},
		{k.Up, k.Down, k.Select, k.Close, k.Back, k.Help, k.Quit},
	}
}
