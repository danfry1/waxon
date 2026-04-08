package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ActionType identifies a context action.
type ActionType int

const (
	ActionPlay ActionType = iota
	ActionQueue
	ActionLike
	ActionGoArtist
	ActionGoAlbum
	ActionOpenSpotify
	ActionCopyURI
	ActionPlayPlaylist
	ActionOpenPlaylistSpotify
	ActionLoadTracks
)

// ActionItem is a single entry in the actions popup.
type ActionItem struct {
	Type  ActionType
	Label string
	Icon  string
}

// ActionsPopup is a floating overlay listing context actions for the selected item.
type ActionsPopup struct {
	items    []ActionItem
	cursor   int
	title    string
	uri      string // track or playlist URI for copy action
	artistID string // first artist's Spotify ID for "Go to Artist"
	albumID  string // album's Spotify ID for "Go to Album"
	width    int
	height   int
}

// NewTrackActions returns an ActionsPopup configured for a track.
func NewTrackActions(trackName, artistName, uri, artistID, albumID string, width, height int) ActionsPopup {
	title := trackName
	if artistName != "" {
		title = fmt.Sprintf("%s — %s", trackName, artistName)
	}
	return ActionsPopup{
		items: []ActionItem{
			{Type: ActionPlay, Label: "Play", Icon: "▶"},
			{Type: ActionQueue, Label: "Add to Queue", Icon: "♫"},
			{Type: ActionGoArtist, Label: "Go to Artist", Icon: "→"},
			{Type: ActionGoAlbum, Label: "Go to Album", Icon: "→"},
			{Type: ActionOpenSpotify, Label: "Open in Spotify", Icon: "◎"},
			{Type: ActionCopyURI, Label: "Copy Track URI", Icon: "⎘"},
		},
		title:    title,
		uri:      uri,
		artistID: artistID,
		albumID:  albumID,
		width:    width,
		height:   height,
	}
}

// NewPlaylistActions returns an ActionsPopup configured for a playlist.
func NewPlaylistActions(playlistName, uri string, width, height int) ActionsPopup {
	return ActionsPopup{
		items: []ActionItem{
			{Type: ActionPlayPlaylist, Label: "Play Playlist", Icon: "▶"},
			{Type: ActionLoadTracks, Label: "Load Tracks", Icon: "♫"},
			{Type: ActionOpenPlaylistSpotify, Label: "Open in Spotify", Icon: "◎"},
		},
		title:  playlistName,
		uri:    uri,
		width:  width,
		height: height,
	}
}

// MoveDown moves the cursor down in the actions list.
func (a *ActionsPopup) MoveDown() {
	a.cursor++
	if a.cursor >= len(a.items) {
		a.cursor = len(a.items) - 1
	}
}

// MoveUp moves the cursor up in the actions list.
func (a *ActionsPopup) MoveUp() {
	a.cursor--
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// Selected returns the currently highlighted action.
func (a ActionsPopup) Selected() ActionItem {
	if a.cursor >= 0 && a.cursor < len(a.items) {
		return a.items[a.cursor]
	}
	return ActionItem{}
}

// URI returns the URI stored in the popup (for clipboard copy).
func (a ActionsPopup) URI() string {
	return a.uri
}

// ArtistID returns the artist ID stored in the popup.
func (a ActionsPopup) ArtistID() string {
	return a.artistID
}

// AlbumID returns the album ID stored in the popup.
func (a ActionsPopup) AlbumID() string {
	return a.albumID
}

// View renders the actions popup as a centered floating overlay.
func (a ActionsPopup) View() string {
	overlayW := min(50, a.width-8)
	overlayH := min(len(a.items)+6, a.height-4)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Background(ColorBg).
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(ColorTextDim)

	content := titleStyle.Render("  Actions") + "\n"
	content += subtitleStyle.Render("  "+truncate(a.title, overlayW-6)) + "\n\n"

	for i, item := range a.items {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(ColorTextSec)
		if i == a.cursor {
			prefix = "> "
			style = StyleActiveItem
		}
		line := fmt.Sprintf("%s%s  %s", prefix, item.Icon, item.Label)
		content += style.Render(line) + "\n"
	}

	content += "\n" + subtitleStyle.Render("  j/k navigate  Enter select  Esc close")

	overlay := border.Render(content)
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#000000")))
}
