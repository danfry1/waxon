package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	PlayPause key.Binding
	Next      key.Binding
	Prev      key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		PlayPause: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		Next:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next track")),
		Prev:      key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev track")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.Next, k.Prev, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.PlayPause, k.Next, k.Prev},
		{k.Help, k.Quit},
	}
}
