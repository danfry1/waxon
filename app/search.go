package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

type searchResultsMsg struct {
	results *source.SearchResults
	query   string
}

type searchDebounceMsg struct {
	query string
}

// Search is the floating search overlay model.
type Search struct {
	input     textinput.Model
	results   *source.SearchResults
	cursor    int
	width     int
	height    int
	lastQuery string
}

func NewSearch(width, height int) Search {
	ti := textinput.New()
	ti.Placeholder = "Search tracks, artists, albums..."
	ti.Focus()
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Width = min(60, width-10)

	return Search{
		input:  ti,
		width:  width,
		height: height,
	}
}

func (s Search) Update(msg tea.Msg) (Search, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyDown:
			s.cursor++
			s.clampCursor()
			return s, nil
		case tea.KeyUp:
			s.cursor--
			s.clampCursor()
			return s, nil
		}

		// Forward to text input
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)

		// Debounce search
		query := s.input.Value()
		if query != s.lastQuery && len(query) >= 2 {
			s.lastQuery = query
			return s, tea.Batch(cmd, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
				return searchDebounceMsg{query: query}
			}))
		}
		return s, cmd

	case searchResultsMsg:
		if msg.query == s.input.Value() {
			s.results = msg.results
			s.cursor = 0
		}
		return s, nil
	}

	return s, nil
}

func (s *Search) clampCursor() {
	total := s.totalResults()
	if total == 0 {
		s.cursor = 0
		return
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= total {
		s.cursor = total - 1
	}
}

func (s Search) totalResults() int {
	if s.results == nil {
		return 0
	}
	return s.displayedTracks() + s.displayedArtists() + s.displayedAlbums()
}

// Display caps for each section (must match the View rendering limits).
const (
	maxDisplayTracks  = 8
	maxDisplayArtists = 4
	maxDisplayAlbums  = 4
)

func (s Search) displayedTracks() int  { return min(maxDisplayTracks, len(s.results.Tracks)) }
func (s Search) displayedArtists() int { return min(maxDisplayArtists, len(s.results.Artists)) }
func (s Search) displayedAlbums() int  { return min(maxDisplayAlbums, len(s.results.Albums)) }

// SelectedTrack returns the selected track if the cursor is on a track row.
func (s Search) SelectedTrack() *source.Track {
	if s.results == nil || s.cursor < 0 || s.cursor >= s.displayedTracks() {
		return nil
	}
	return &s.results.Tracks[s.cursor]
}

// SelectedArtist returns the selected artist if the cursor is on an artist row.
func (s Search) SelectedArtist() *source.SearchArtist {
	if s.results == nil {
		return nil
	}
	artistIdx := s.cursor - s.displayedTracks()
	if artistIdx < 0 || artistIdx >= s.displayedArtists() {
		return nil
	}
	return &s.results.Artists[artistIdx]
}

// SelectedAlbum returns the selected album if the cursor is on an album row.
func (s Search) SelectedAlbum() *source.SearchAlbum {
	if s.results == nil {
		return nil
	}
	albumIdx := s.cursor - s.displayedTracks() - s.displayedArtists()
	if albumIdx < 0 || albumIdx >= s.displayedAlbums() {
		return nil
	}
	return &s.results.Albums[albumIdx]
}

func (s Search) View() string {
	overlayW := min(64, s.width-8)
	overlayH := min(28, s.height-4)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Background(ColorBg).
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2)

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

	var content string
	content += titleStyle.Render("  Search") + "  " + hintStyle.Render("Esc to close") + "\n\n"
	content += s.input.View() + "\n"

	// Divider below input
	divider := lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", overlayW-4))
	content += divider + "\n"

	if s.results != nil && s.totalResults() > 0 {
		idx := 0

		sectionStyle := lipgloss.NewStyle().Foreground(ColorTextDim).Bold(true)

		if len(s.results.Tracks) > 0 {
			content += "\n" + sectionStyle.Render(" TRACKS") + "\n"
			for i, t := range s.results.Tracks {
				selected := idx == s.cursor
				line := s.renderTrackRow(t, selected, overlayW-6)
				content += line + "\n"
				idx++
				if i >= maxDisplayTracks-1 {
					break
				}
			}
		}

		if len(s.results.Artists) > 0 {
			content += "\n" + sectionStyle.Render(" ARTISTS") + "\n"
			for i, a := range s.results.Artists {
				selected := idx == s.cursor
				line := s.renderResultRow(a.Name, "", selected, overlayW-6)
				content += line + "\n"
				idx++
				if i >= maxDisplayArtists-1 {
					break
				}
			}
		}

		if len(s.results.Albums) > 0 {
			content += "\n" + sectionStyle.Render(" ALBUMS") + "\n"
			for i, a := range s.results.Albums {
				selected := idx == s.cursor
				line := s.renderResultRow(a.Name, a.Artist, selected, overlayW-6)
				content += line + "\n"
				idx++
				if i >= maxDisplayAlbums-1 {
					break
				}
			}
		}
	} else if s.input.Value() != "" {
		content += "\n" + StyleDimText.Render("  Searching...")
	} else {
		content += "\n"
		content += StyleDimText.Render("  Type to search Spotify") + "\n\n"
		content += StyleDimText.Render("  Results show tracks, artists, and albums.") + "\n\n"
		content += lipgloss.NewStyle().Foreground(ColorBorder).Render("  j/k navigate   Enter select   Esc close") + "\n"
	}

	overlay := border.Render(content)
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#000000")))
}

// renderTrackRow renders a single track result row.
// The entire line is styled uniformly to avoid ANSI background bleed.
func (s Search) renderTrackRow(t source.Track, selected bool, maxW int) string {
	dur := fmtDur(t.Duration)
	// Reserve space: 4 prefix + 2 gap + artist + 2 gap + duration
	artistW := min(len(t.Artist), 20)
	nameW := maxW - 4 - artistW - len(dur) - 4
	if nameW < 10 {
		nameW = 10
	}

	name := truncate(t.Name, nameW)
	artist := truncate(t.Artist, artistW)

	// Build plain text line, then style the whole thing
	prefix := "   "
	if selected {
		prefix = " ▸ "
	}
	// Pad name to fixed width for alignment
	namePad := nameW - lipgloss.Width(name)
	if namePad < 0 {
		namePad = 0
	}
	line := prefix + name + strings.Repeat(" ", namePad) + "  " + artist + "  " + dur

	if selected {
		return lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(truncate(line, maxW))
	}
	return lipgloss.NewStyle().Foreground(ColorTextSec).Render(truncate(line, maxW))
}

// renderResultRow renders a single artist/album result row.
func (s Search) renderResultRow(name, subtitle string, selected bool, maxW int) string {
	prefix := "   "
	if selected {
		prefix = " ▸ "
	}
	line := prefix + name
	if subtitle != "" {
		line += "  " + subtitle
	}

	if selected {
		return lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(truncate(line, maxW))
	}
	return lipgloss.NewStyle().Foreground(ColorTextSec).Render(truncate(line, maxW))
}
