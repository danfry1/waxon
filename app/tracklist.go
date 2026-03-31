package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/waxon/source"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// TrackList is the right pane model showing tracks in a table.
type TrackList struct {
	table      table.Model
	tracks     []source.Track
	filtered   []source.Track // subset visible after filter; nil means no filter active
	filterText string         // current active filter query
	title      string
	subtitle   string // optional second line (e.g. genres, artist name)
	headerInfo string // optional third line (e.g. "30 tracks · 1h 42m")
	contextURI string // playlist/album URI for play context
	artBlock   string // rendered playlist/album art
	loading    bool   // true while fetching tracks
	spinFrame  int    // current spinner animation frame
	width      int
	height     int
	nowPlaying string // ID of currently playing track
}

func trackColumns(width int) []table.Column {
	return []table.Column{
		{Title: " ", Width: 2},
		{Title: "#", Width: 4},
		{Title: "Title", Width: max(10, width/3)},
		{Title: "Artist", Width: max(10, width/4)},
		{Title: "Duration", Width: 8},
	}
}

func NewTrackList(width, height int) TrackList {
	columns := trackColumns(width)

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(height-4),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		BorderBottom(true).
		Foreground(ColorTextDim).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(ColorAccent).
		Background(ColorSurface).
		Bold(true)
	t.SetStyles(s)

	return TrackList{
		table:  t,
		title:  "Tracks",
		width:  width,
		height: height,
	}
}

func (tl *TrackList) SetLoading(title string) {
	tl.loading = true
	tl.title = title
	tl.tracks = nil
	tl.filtered = nil
	tl.table.SetRows(nil)
}

func (tl *TrackList) TickSpinner() {
	tl.spinFrame = (tl.spinFrame + 1) % len(spinnerFrames)
}

func (tl *TrackList) SetSubtitle(s string) {
	tl.subtitle = s
}

func (tl *TrackList) SetHeaderInfo(info string) {
	tl.headerInfo = info
	tl.table.SetHeight(tl.tableHeight())
}

func (tl *TrackList) SetTracks(tracks []source.Track, title, contextURI string) {
	tl.loading = false
	tl.tracks = tracks
	tl.title = title
	tl.subtitle = ""
	tl.headerInfo = ""
	tl.contextURI = contextURI

	// Re-apply active filter when tracks are refreshed
	if tl.filterText != "" {
		tl.applyFilter(tl.filterText)
	} else {
		tl.filtered = nil
	}

	tl.rebuildRows()
}

// AppendTracks adds more tracks to the existing list without resetting
// the cursor position. Used for lazy-loading additional pages.
func (tl *TrackList) AppendTracks(tracks []source.Track) {
	tl.tracks = append(tl.tracks, tracks...)
	if tl.filterText != "" {
		tl.applyFilter(tl.filterText)
	}
	tl.rebuildRows()
}

// displayTracks returns the tracks currently visible (filtered or all).
func (tl *TrackList) displayTracks() []source.Track {
	if tl.filtered != nil {
		return tl.filtered
	}
	return tl.tracks
}

// rebuildRows regenerates the table rows from the display tracks.
func (tl *TrackList) rebuildRows() {
	display := tl.displayTracks()
	rows := make([]table.Row, len(display))
	trackNum := 0
	for i, t := range display {
		if t.IsSeparator {
			if t.Name == "" {
				// Blank spacer row for breathing room
				rows[i] = table.Row{" ", "", "", "", ""}
			} else {
				// Section divider: ─── ALBUMS ─────────────
				label := " " + strings.ToUpper(t.Name) + " "
				colW := tl.columnWidth(2)
				rightPad := colW - 4 - len(label) // 4 for left dashes
				if rightPad < 3 {
					rightPad = 3
				}
				divider := strings.Repeat("─", 4) + label + strings.Repeat("─", rightPad)
				rows[i] = table.Row{
					" ",
					"",
					divider,
					t.Artist,
					"",
				}
			}
		} else if t.IsAlbumRow {
			// Album row — no prefix, album name, year in duration col
			rows[i] = table.Row{
				" ",
				"",
				truncate(t.Name, tl.columnWidth(2)),
				"",
				t.Artist,
			}
		} else {
			trackNum++
			prefix := " "
			if t.ID == tl.nowPlaying {
				prefix = "▶"
			}
			rows[i] = table.Row{
				prefix,
				strconv.Itoa(trackNum),
				truncate(t.Name, tl.columnWidth(2)),
				truncate(t.Artist, tl.columnWidth(3)),
				fmtDur(t.Duration),
			}
		}
	}
	tl.table.SetRows(rows)
}

// SetFilter applies a case-insensitive filter matching track name or artist.
// An empty query clears the filter.
func (tl *TrackList) SetFilter(query string) {
	if query == "" {
		tl.ClearFilter()
		return
	}
	tl.filterText = query
	tl.applyFilter(query)
	tl.rebuildRows()
}

func (tl *TrackList) applyFilter(query string) {
	q := strings.ToLower(query)
	var result []source.Track
	for _, t := range tl.tracks {
		if t.IsSeparator || t.IsAlbumRow {
			continue
		}
		if strings.Contains(strings.ToLower(t.Name), q) ||
			strings.Contains(strings.ToLower(t.Artist), q) {
			result = append(result, t)
		}
	}
	tl.filtered = result
}

// ClearFilter removes the active filter and restores the full track list.
func (tl *TrackList) ClearFilter() {
	tl.filterText = ""
	tl.filtered = nil
	tl.rebuildRows()
}

// FilterText returns the current active filter query (empty if none).
func (tl *TrackList) FilterText() string {
	return tl.filterText
}

func (tl *TrackList) ContextURI() string {
	return tl.contextURI
}

func (tl *TrackList) SetArt(artBlock string) {
	tl.artBlock = artBlock
	tl.table.SetHeight(tl.tableHeight())
}

func (tl *TrackList) SetNowPlaying(trackID string) {
	if tl.nowPlaying == trackID {
		return
	}
	tl.nowPlaying = trackID
	// Refresh rows to update the ▶ indicator
	if len(tl.tracks) > 0 {
		tl.rebuildRows()
	}
}

func (tl *TrackList) SelectedTrack() *source.Track {
	display := tl.displayTracks()
	idx := tl.table.Cursor()
	if idx < 0 || idx >= len(display) {
		return nil
	}
	return &display[idx]
}

// JumpToTrack moves the cursor to the track with the given ID.
// Returns true if the track was found, false otherwise.
func (tl *TrackList) JumpToTrack(trackID string) bool {
	display := tl.displayTracks()
	for i, t := range display {
		if t.ID == trackID {
			tl.table.SetCursor(i)
			return true
		}
	}
	return false
}

func (tl *TrackList) tableHeight() int {
	extra := 4 // border + header
	if tl.artBlock != "" {
		extra += HeaderArtH
	}
	if tl.subtitle != "" {
		extra += 1
	}
	if tl.headerInfo != "" {
		extra += 1
	}
	return max(3, tl.height-extra)
}

func (tl *TrackList) Resize(width, height int) {
	tl.width = width
	tl.height = height
	tl.table.SetHeight(tl.tableHeight())
	tl.table.SetColumns(trackColumns(width))
	// Re-render rows with new widths
	if len(tl.tracks) > 0 {
		tl.rebuildRows()
	}
}

func (tl TrackList) Update(msg tea.Msg) (TrackList, tea.Cmd) {
	var cmd tea.Cmd
	tl.table, cmd = tl.table.Update(msg)
	return tl, cmd
}

func (tl TrackList) View(active bool) string {
	border := PaneBorder(active)
	titleStyle := StyleSectionHeader
	if active {
		titleStyle = titleStyle.Foreground(CurrentAccent())
	}
	titleText := tl.title
	if tl.loading {
		spinStyle := lipgloss.NewStyle().Foreground(CurrentAccent())
		spin := spinStyle.Render(spinnerFrames[tl.spinFrame])
		titleText = spin + " " + tl.title
	}
	header := titleStyle.Render(titleText)

	// Render subtitle as styled genre tags
	if tl.subtitle != "" {
		accentColor := CurrentAccent()
		tagStyle := lipgloss.NewStyle().
			Foreground(accentColor).
			Background(ColorSurface).
			Padding(0, 1)
		var tags string
		for i, genre := range strings.Split(tl.subtitle, ", ") {
			if i > 0 {
				tags += " "
			}
			tags += tagStyle.Render(genre)
		}
		header += "\n " + tags
	}

	// Render header info line (track count, duration, etc.)
	if tl.headerInfo != "" {
		header += "\n " + StyleDimText.Render(tl.headerInfo)
	}

	// Show playlist art alongside title if available
	if tl.artBlock != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Center, tl.artBlock, " ", header)
	}

	var content string
	if tl.loading {
		loadingMsg := StyleDimText.Render("\n  Loading...")
		content = header + "\n" + loadingMsg
	} else {
		content = header + "\n" + tl.table.View()
	}

	return border.
		Width(tl.width - 2).
		Height(tl.height - 2).
		Render(content)
}

// headerRows returns the number of rows above the table data area,
// including the pane border, header content, and the table's column
// header + border.
func (tl TrackList) headerRows() int {
	rows := 1 // pane border top

	if tl.artBlock != "" {
		// Art is joined horizontally with title/subtitle/headerInfo,
		// so the combined height equals the art height.
		rows += HeaderArtH
	} else {
		rows++ // title line
		if tl.subtitle != "" {
			rows++
		}
		if tl.headerInfo != "" {
			rows++
		}
	}

	rows++ // table column headers
	rows++ // table header bottom border
	return rows
}

// SetCursorFromClick maps a Y coordinate (relative to the pane top)
// to a table data row and moves the cursor there.
func (tl *TrackList) SetCursorFromClick(y int) {
	row := y - tl.headerRows()
	if row < 0 {
		return
	}
	display := tl.displayTracks()
	if row >= len(display) {
		return
	}
	tl.table.SetCursor(row)
}

func (tl TrackList) columnWidth(idx int) int {
	cols := tl.table.Columns()
	if idx < len(cols) {
		return cols[idx].Width
	}
	return 20
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	runes := []rune(s)
	if maxW <= 3 {
		// No room for ellipsis — just trim to fit
		for len(runes) > 0 && lipgloss.Width(string(runes)) > maxW {
			runes = runes[:len(runes)-1]
		}
		return string(runes)
	}
	// Trim runes until the display width fits within maxW-3 (leaving room for "...")
	for len(runes) > 0 && lipgloss.Width(string(runes))+3 > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

// NavState captures the tracklist state for browser-like back navigation.
type NavState struct {
	tracks     []source.Track
	title      string
	subtitle   string
	headerInfo string
	contextURI string
	artBlock   string
	cursor     int
	focusPane  Pane
}

// GetState snapshots the current tracklist state (including cursor position).
func (tl *TrackList) GetState(pane Pane) NavState {
	return NavState{
		tracks:     tl.tracks,
		title:      tl.title,
		subtitle:   tl.subtitle,
		headerInfo: tl.headerInfo,
		contextURI: tl.contextURI,
		artBlock:   tl.artBlock,
		cursor:     tl.table.Cursor(),
		focusPane:  pane,
	}
}

// RestoreState restores the tracklist from a previously saved NavState.
func (tl *TrackList) RestoreState(s NavState) {
	tl.loading = false
	tl.tracks = s.tracks
	tl.title = s.title
	tl.subtitle = s.subtitle
	tl.headerInfo = s.headerInfo
	tl.contextURI = s.contextURI
	tl.artBlock = s.artBlock
	tl.filtered = nil
	tl.filterText = ""
	tl.rebuildRows()
	tl.table.SetHeight(tl.tableHeight())
	if s.cursor >= 0 && s.cursor < len(s.tracks) {
		tl.table.SetCursor(s.cursor)
	}
}

// FormatTrackListInfo returns a summary string like "30 tracks · 1h 42m".
func FormatTrackListInfo(tracks []source.Track) string {
	count := 0
	var total time.Duration
	for _, t := range tracks {
		if t.IsSeparator || t.IsAlbumRow {
			continue
		}
		count++
		total += t.Duration
	}
	if count == 0 {
		return ""
	}
	h := int(total.Hours())
	m := int(total.Minutes()) % 60
	trackWord := pluralize(count, "track", "tracks")
	if h > 0 {
		return fmt.Sprintf("%d %s · %dh %dm", count, trackWord, h, m)
	}
	return fmt.Sprintf("%d %s · %dm", count, trackWord, m)
}

// FormatPartialTrackListInfo returns "N of M tracks" for partially loaded playlists.
func FormatPartialTrackListInfo(loaded, total int) string {
	return fmt.Sprintf("%d of %d tracks", loaded, total)
}

// FormatAlbumInfo returns "Artist · Year · N tracks".
func FormatAlbumInfo(artist, year string, tracks []source.Track) string {
	count := 0
	for _, t := range tracks {
		if !t.IsSeparator && !t.IsAlbumRow {
			count++
		}
	}
	parts := []string{}
	if artist != "" {
		parts = append(parts, artist)
	}
	if year != "" {
		parts = append(parts, year)
	}
	parts = append(parts, fmt.Sprintf("%d %s", count, pluralize(count, "track", "tracks")))
	return strings.Join(parts, " · ")
}

// buildArtistTrackList constructs a combined track list from an artist page,
// including top tracks, album rows, and section separators.
func buildArtistTrackList(page *source.ArtistPage) []source.Track {
	tracks := append([]source.Track{}, page.Tracks...)

	var albums, singles []source.ArtistAlbum
	for _, a := range page.Albums {
		if a.Type == "Album" {
			albums = append(albums, a)
		} else {
			singles = append(singles, a)
		}
	}

	if len(albums) > 0 {
		tracks = append(tracks, source.Track{IsSeparator: true})
		tracks = append(tracks, source.Track{
			Name:        "Albums",
			Artist:      fmt.Sprintf("%d %s", len(albums), pluralize(len(albums), "album", "albums")),
			IsSeparator: true,
		})
		for _, album := range albums {
			tracks = append(tracks, source.Track{
				Name:       album.Name,
				Artist:     album.Year,
				AlbumID:    album.ID,
				IsAlbumRow: true,
			})
		}
	}

	if len(singles) > 0 {
		tracks = append(tracks, source.Track{IsSeparator: true})
		tracks = append(tracks, source.Track{
			Name:        "Singles & EPs",
			Artist:      fmt.Sprintf("%d %s", len(singles), pluralize(len(singles), "single", "singles")),
			IsSeparator: true,
		})
		for _, single := range singles {
			tracks = append(tracks, source.Track{
				Name:       single.Name,
				Artist:     single.Year,
				AlbumID:    single.ID,
				IsAlbumRow: true,
			})
		}
	}

	return tracks
}
