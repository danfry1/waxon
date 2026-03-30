package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/source"
)

type PanelType int

const (
	PanelNone PanelType = iota
	PanelQueue
	PanelLibrary
	PanelSearch
	PanelDevices
)

type trackItem struct {
	track source.Track
}

func (t trackItem) Title() string       { return t.track.Name }
func (t trackItem) Description() string { return fmt.Sprintf("%s · %s", t.track.Artist, formatDur(t.track.Duration)) }
func (t trackItem) FilterValue() string { return t.track.Name + " " + t.track.Artist }

type playlistItem struct {
	playlist source.Playlist
}

func (p playlistItem) Title() string       { return p.playlist.Name }
func (p playlistItem) Description() string { return fmt.Sprintf("%d tracks", p.playlist.TrackCount) }
func (p playlistItem) FilterValue() string { return p.playlist.Name }

type deviceItem struct {
	device source.Device
}

func (d deviceItem) Title() string {
	if d.device.IsActive {
		return "▶ " + d.device.Name
	}
	return "  " + d.device.Name
}
func (d deviceItem) Description() string { return d.device.Type }
func (d deviceItem) FilterValue() string { return d.device.Name }

type Panel struct {
	Type      PanelType
	List      list.Model
	Search    textinput.Model
	Width     int
	Height    int

	playlists    []source.Playlist
	inPlaylist   bool
	playlistID   string
	playlistName string
}

func NewPanel(panelType PanelType, width, height int) Panel {
	panelW := width / 2
	panelH := height - 4

	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, panelW-4, panelH-4)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	p := Panel{
		Type:   panelType,
		List:   l,
		Width:  panelW,
		Height: panelH,
	}

	switch panelType {
	case PanelQueue:
		l.Title = "Up Next"
	case PanelLibrary:
		l.Title = "Library"
	case PanelDevices:
		l.Title = "Devices"
	case PanelSearch:
		ti := textinput.New()
		ti.Placeholder = "Search tracks, artists, albums..."
		ti.Focus()
		ti.Width = panelW - 8
		p.Search = ti
		l.Title = "Search"
	}

	p.List = l
	return p
}

func (p *Panel) SetItems(items []list.Item) {
	p.List.SetItems(items)
}

func (p *Panel) Resize(width, height int) {
	p.Width = width / 2
	p.Height = height - 4
	p.List.SetSize(p.Width-4, p.Height-4)
}

func (p Panel) View(primary, secondary, background string) string {
	panelBg := lipgloss.Color(background)
	borderColor := lipgloss.Color(primary)

	style := lipgloss.NewStyle().
		Width(p.Width - 2).
		Height(p.Height).
		Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2)

	content := p.List.View()
	if p.Type == PanelSearch {
		searchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(primary)).
			Background(panelBg)
		content = searchStyle.Render(p.Search.View()) + "\n\n" + content
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(secondary)).
		Background(panelBg)
	hints := p.hintText()

	return style.Render(content + "\n" + hintStyle.Render(hints))
}

func (p Panel) hintText() string {
	switch p.Type {
	case PanelQueue:
		return "↑↓ navigate · enter play · q close"
	case PanelLibrary:
		if p.inPlaylist {
			return "↑↓ navigate · enter play · backspace back · l close"
		}
		return "↑↓ navigate · enter open · l close"
	case PanelSearch:
		return "type to search · ↑↓ navigate · enter play · esc close"
	case PanelDevices:
		return "↑↓ navigate · enter select · d close"
	}
	return ""
}

func formatDur(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// Panel-related messages
type queueLoadedMsg struct{ tracks []source.Track }
type playlistsLoadedMsg struct{ playlists []source.Playlist }
type playlistTracksMsg struct{ tracks []source.Track }
type devicesLoadedMsg struct{ devices []source.Device }
type searchResultsMsg struct{ results *source.SearchResults }

func fetchQueue(src source.RichSource) tea.Cmd {
	return func() tea.Msg {
		tracks, err := src.Queue()
		if err != nil {
			return trackErrorMsg{err}
		}
		return queueLoadedMsg{tracks}
	}
}

func fetchPlaylists(src source.RichSource) tea.Cmd {
	return func() tea.Msg {
		playlists, err := src.Playlists()
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistsLoadedMsg{playlists}
	}
}

func fetchPlaylistTracks(src source.RichSource, id string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := src.PlaylistTracks(id)
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistTracksMsg{tracks}
	}
}

func fetchDevices(src source.RichSource) tea.Cmd {
	return func() tea.Msg {
		devices, err := src.Devices()
		if err != nil {
			return trackErrorMsg{err}
		}
		return devicesLoadedMsg{devices}
	}
}

func doSearch(src source.RichSource, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := src.Search(query)
		if err != nil {
			return trackErrorMsg{err}
		}
		return searchResultsMsg{results}
	}
}
