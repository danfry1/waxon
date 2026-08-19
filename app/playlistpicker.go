package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

// PlaylistPicker is a floating overlay for choosing one of the user's
// editable playlists (e.g. "Add to playlist…"). Typing filters the list;
// j/k (or ctrl+n/ctrl+p, arrows) move; Enter picks; Esc closes.
type PlaylistPicker struct {
	input     textinput.Model
	all       []source.Playlist // editable playlists only
	filtered  []source.Playlist
	cursor    int
	title     string
	subtitle  string
	width     int
	height    int
	trackID   string // subject of the action
	trackName string
}

// pickerMaxRows is the most playlists shown at once.
const pickerMaxRows = 12

// NewPlaylistPicker builds a picker over the editable playlists in pls.
func NewPlaylistPicker(pls []source.Playlist, trackID, trackName string, width, height int) PlaylistPicker {
	ti := textinput.New()
	ti.Placeholder = "Type to filter…"
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Width = min(44, width-12)
	ti.Focus()

	editable := make([]source.Playlist, 0, len(pls))
	for _, pl := range pls {
		if pl.Editable && pl.ID != likedPlaylistID {
			editable = append(editable, pl)
		}
	}
	p := PlaylistPicker{
		input:     ti,
		all:       editable,
		title:     "Add to playlist",
		subtitle:  trackName,
		width:     width,
		height:    height,
		trackID:   trackID,
		trackName: trackName,
	}
	p.applyFilter()
	return p
}

func (p *PlaylistPicker) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	p.filtered = p.filtered[:0]
	for _, pl := range p.all {
		if q == "" || strings.Contains(strings.ToLower(pl.Name), q) {
			p.filtered = append(p.filtered, pl)
		}
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = max(0, len(p.filtered)-1)
	}
}

// Update handles keys: navigation is intercepted, everything else edits the
// filter. Enter/Esc are handled by the caller.
func (p PlaylistPicker) Update(msg tea.Msg) (PlaylistPicker, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyDown, tea.KeyCtrlN, tea.KeyCtrlJ:
			p.MoveDown()
			return p, nil
		case tea.KeyUp, tea.KeyCtrlP, tea.KeyCtrlK:
			p.MoveUp()
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.applyFilter()
	return p, cmd
}

// MoveDown moves the cursor down.
func (p *PlaylistPicker) MoveDown() {
	if p.cursor < len(p.filtered)-1 {
		p.cursor++
	}
}

// MoveUp moves the cursor up.
func (p *PlaylistPicker) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// Selected returns the highlighted playlist, or nil when the list is empty.
func (p PlaylistPicker) Selected() *source.Playlist {
	if p.cursor >= 0 && p.cursor < len(p.filtered) {
		pl := p.filtered[p.cursor]
		return &pl
	}
	return nil
}

// TrackID returns the track the picker is acting on.
func (p PlaylistPicker) TrackID() string { return p.trackID }

// TrackName returns the track's display name.
func (p PlaylistPicker) TrackName() string { return p.trackName }

// View renders the picker as a centred overlay.
func (p PlaylistPicker) View() string {
	overlayW := min(56, p.width-8)
	rows := min(len(p.filtered), pickerMaxRows)
	overlayH := min(rows+9, p.height-4)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Background(ColorBg).
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2)
	titleStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorTextDim)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("  "+p.title) + "\n")
	if p.subtitle != "" {
		sb.WriteString(dim.Render("  "+truncate(p.subtitle, overlayW-8)) + "\n")
	}
	sb.WriteString("\n" + p.input.View() + "\n\n")

	switch {
	case len(p.all) == 0:
		sb.WriteString(dim.Render("  No editable playlists — create one with :playlist new NAME") + "\n")
	case len(p.filtered) == 0:
		sb.WriteString(dim.Render("  No playlists match") + "\n")
	default:
		// Keep the cursor within the visible window.
		start := 0
		if p.cursor >= pickerMaxRows {
			start = p.cursor - pickerMaxRows + 1
		}
		for i := start; i < len(p.filtered) && i < start+pickerMaxRows; i++ {
			pl := p.filtered[i]
			prefix, style := "  ", lipgloss.NewStyle().Foreground(ColorTextSec)
			if i == p.cursor {
				prefix, style = "> ", StyleActiveItem
			}
			count := ""
			if pl.TrackCount > 0 {
				count = dim.Render(fmt.Sprintf("  %d %s", pl.TrackCount, pluralize(pl.TrackCount, "track", "tracks")))
			}
			sb.WriteString(style.Render(prefix+truncate(pl.Name, overlayW-18)) + count + "\n")
		}
		if len(p.filtered) > pickerMaxRows {
			sb.WriteString(dim.Render("  …") + "\n")
		}
	}
	sb.WriteString("\n" + dim.Render(truncate("  type to filter · ↑/↓ move · Enter add · Esc", overlayW-4)))

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, border.Render(sb.String()), overlayBackdrop())
}
