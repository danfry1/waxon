package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

// SidebarSection identifies which section of the sidebar.
type SidebarSection int

const (
	SectionLibrary SidebarSection = iota
	SectionQueue
)

// sidebarItem wraps a playlist for the bubbles/list.
type sidebarItem struct {
	playlist source.Playlist
	icon     string // tiny rendered art icon (4x1 half-blocks)
}

func (s sidebarItem) Title() string {
	if s.icon != "" {
		return s.icon + " " + s.playlist.Name
	}
	return "♫ " + s.playlist.Name
}

func (s sidebarItem) Description() string {
	if s.playlist.TrackCount == 0 {
		return ""
	}
	return fmt.Sprintf("%d tracks", s.playlist.TrackCount)
}

func (s sidebarItem) FilterValue() string {
	return s.playlist.Name
}

// queueItem wraps a track for the queue view in the sidebar.
type queueItem struct {
	track source.Track
}

func (q queueItem) Title() string       { return "♫ " + q.track.Name }
func (q queueItem) Description() string { return q.track.Artist }
func (q queueItem) FilterValue() string { return q.track.Name }

// Sidebar is the left pane model.
type Sidebar struct {
	list     list.Model
	section  SidebarSection
	allItems []list.Item // unfiltered items for restoring after filter
	width    int
	height   int
}

func NewSidebar(width, height int) Sidebar {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(ColorAccent).
		BorderForeground(ColorAccent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(ColorTextSec).
		BorderForeground(ColorAccent)

	l := list.New(nil, delegate, width-2, height-2)
	l.Title = "LIBRARY"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = StyleSectionHeader

	return Sidebar{
		list:    l,
		section: SectionLibrary,
		width:   width,
		height:  height,
	}
}

func (s *Sidebar) SetPlaylists(playlists []source.Playlist) {
	items := make([]list.Item, len(playlists))
	for i, pl := range playlists {
		items[i] = sidebarItem{playlist: pl}
	}
	s.allItems = items
	s.list.SetItems(items)
}

// SetPlaylistIcons updates the library playlist items with rendered art icons.
// Merges into allItems (the library set) rather than the live list, so icons
// are preserved correctly even if the user switched to the queue view.
func (s *Sidebar) SetPlaylistIcons(icons map[string]string) {
	if s.allItems == nil {
		return
	}
	updated := make([]list.Item, len(s.allItems))
	for i, item := range s.allItems {
		if si, ok := item.(sidebarItem); ok {
			if icon, found := icons[si.playlist.ID]; found {
				si.icon = icon
			}
			updated[i] = si
		} else {
			updated[i] = item
		}
	}
	s.allItems = updated
	// Only update the live list if we're currently showing the library
	if s.section == SectionLibrary {
		s.list.SetItems(updated)
	}
}

func (s *Sidebar) SetQueueTracks(tracks []source.Track) {
	// Spotify pads the queue with the current track on repeat.
	// Strip trailing consecutive duplicates, keeping at most one.
	tracks = stripTrailingDupes(tracks)

	if len(tracks) == 0 {
		s.list.SetItems(nil)
		s.list.Title = "QUEUE (empty)"
		return
	}

	items := make([]list.Item, len(tracks))
	for i, t := range tracks {
		items[i] = queueItem{track: t}
	}
	// Don't overwrite allItems — that holds the library playlists for
	// restoring when switching back from the queue section.
	s.list.SetItems(items)
	s.list.Title = "QUEUE"
}

// stripTrailingDupes removes trailing consecutive duplicates of the last
// track, keeping at most one. Spotify fills the queue response with the
// current track on repeat as padding — these won't actually play.
func stripTrailingDupes(tracks []source.Track) []source.Track {
	if len(tracks) < 2 {
		return tracks
	}
	lastID := tracks[len(tracks)-1].ID
	// Walk backwards to find where the trailing run starts.
	cut := len(tracks)
	for cut > 0 && tracks[cut-1].ID == lastID {
		cut--
	}
	// Keep at most one of the trailing track (it may be the real "next up").
	if cut < len(tracks) {
		return tracks[:cut+1]
	}
	return tracks
}

func (s *Sidebar) Resize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width-2, height-2)
}

func (s *Sidebar) SelectedPlaylist() *source.Playlist {
	item := s.list.SelectedItem()
	if item == nil {
		return nil
	}
	if si, ok := item.(sidebarItem); ok {
		return &si.playlist
	}
	if qi, ok := item.(queueItem); ok {
		return &source.Playlist{
			ID:   qi.track.ID,
			URI:  qi.track.URI,
			Name: qi.track.Name,
		}
	}
	return nil
}

func (s *Sidebar) Section() SidebarSection {
	return s.section
}

func (s *Sidebar) SetSection(sec SidebarSection) {
	s.section = sec
	if sec == SectionLibrary {
		s.list.Title = "LIBRARY"
		// Restore the library items (with icons) that were saved in allItems.
		if s.allItems != nil {
			s.list.SetItems(s.allItems)
		}
	} else {
		s.list.Title = "QUEUE"
	}
	s.list.Select(0)
}

// SetFilter filters the sidebar playlist list by name.
func (s *Sidebar) SetFilter(query string) {
	if query == "" {
		s.ClearFilter()
		return
	}
	q := strings.ToLower(query)
	var filtered []list.Item
	for _, item := range s.allItems {
		if si, ok := item.(sidebarItem); ok {
			if strings.Contains(strings.ToLower(si.playlist.Name), q) {
				filtered = append(filtered, item)
			}
		}
	}
	s.list.SetItems(filtered)
}

// ClearFilter restores the full unfiltered playlist list.
func (s *Sidebar) ClearFilter() {
	if s.allItems != nil {
		s.list.SetItems(s.allItems)
	}
}

// SetCursorFromClick maps a Y coordinate (relative to the pane top)
// to a list item and selects it.
func (s *Sidebar) SetCursorFromClick(y int) {
	// Layout: 1 (border) + 1 (title) + 1 (blank) = 3 rows before items.
	// Each item is 3 rows (2 content + 1 spacing) with the default delegate.
	const headerRows = 3
	const itemHeight = 3 // defaultHeight(2) + defaultSpacing(1)

	row := y - headerRows
	if row < 0 {
		return
	}
	itemIdx := row / itemHeight

	// Convert to global index accounting for pagination
	globalIdx := s.list.Paginator.Page*s.list.Paginator.PerPage + itemIdx
	if globalIdx < 0 || globalIdx >= len(s.list.Items()) {
		return
	}
	s.list.Select(globalIdx)
}

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s Sidebar) View(active bool) string {
	border := PaneBorder(active)
	return border.
		Width(s.width - 2).
		Height(s.height - 2).
		Render(s.list.View())
}
