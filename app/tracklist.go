package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// TrackList is the right pane model showing tracks in a table.
//
// Scrolling is owned by waxon rather than delegated to the bubbles table:
// the table is fed only the currently visible window of rows (never more
// than its viewport height), so it never scrolls internally. As a result
// tl.offset is always the exact data index of the first visible row, which
// is what makes click-to-row mapping reliable even when the list is scrolled.
type TrackList struct {
	table      table.Model
	tracks     []source.Track
	filtered   []source.Track // subset visible after filter; nil means no filter active
	filterText string         // current active filter query
	allRows    []table.Row    // full row set; the table sees only a window of these
	cursor     int            // selected index into displayTracks (global, not windowed)
	offset     int            // data index of the first visible row
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

// Column layout breakpoints, measured on the usable width inside the pane
// border. Below each threshold a column is dropped (a zero width hides it)
// so the table never overflows the pane.
const (
	colsFullMinWidth   = 64 // marker, #, title, artist, duration
	colsNoNumMinWidth  = 44 // marker, title, artist, duration
	colsNoDurMinWidth  = 30 // marker, title, artist
	tableCellPadding   = 2  // bubbles table pads each visible cell by 1 each side
	colMarkerW, colNum = 2, 4
	colDurW            = 8
)

// trackColumns returns a column set that fits within a pane of the given
// total width, dropping the track number, then duration, then artist as the
// pane narrows. Title always gets the most space (4/7 of what's left).
func trackColumns(width int) []table.Column {
	avail := width - 2 // pane border
	cols := []table.Column{
		{Title: " ", Width: colMarkerW},
		{Title: "#", Width: colNum},
		{Title: "Title"},
		{Title: "Artist"},
		{Title: "Duration", Width: colDurW},
	}
	visible := 5
	switch {
	case avail >= colsFullMinWidth:
	case avail >= colsNoNumMinWidth:
		cols[1].Width = 0
		visible = 4
	case avail >= colsNoDurMinWidth:
		cols[1].Width, cols[4].Width = 0, 0
		visible = 3
	default:
		cols[1].Width, cols[3].Width, cols[4].Width = 0, 0, 0
		visible = 2
	}
	rem := avail - cols[0].Width - cols[1].Width - cols[4].Width - visible*tableCellPadding
	if cols[3].Width == 0 && visible == 2 {
		cols[2].Width = max(4, rem)
		return cols
	}
	cols[2].Width = max(6, rem*4/7)
	cols[3].Width = max(4, rem-cols[2].Width)
	return cols
}

func NewTrackList(width, height int) TrackList {
	columns := trackColumns(width)

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(height-4),
	)

	t.SetStyles(trackTableStyles())

	tl := TrackList{
		table:  t,
		title:  "Tracks",
		width:  width,
		height: height,
	}
	// Recompute the viewport height now that the bordered header style is
	// applied, so the visible-row capacity is stable from the start (the
	// header border adds a row that WithHeight alone doesn't account for).
	tl.table.SetHeight(tl.tableHeight())
	return tl
}

// trackTableStyles builds the table styles from the live palette.
func trackTableStyles() table.Styles {
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
	return s
}

// Restyle re-applies palette-derived styles after a theme change.
func (tl *TrackList) Restyle() {
	tl.table.SetStyles(trackTableStyles())
	tl.rebuildRows()
}

func (tl *TrackList) SetLoading(title string) {
	tl.loading = true
	tl.title = title
	tl.tracks = nil
	tl.filtered = nil
	tl.allRows = nil
	tl.cursor = 0
	tl.offset = 0
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
	tl.syncViewport()
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
	tl.allRows = rows
	tl.syncViewport()
}

// capacity returns how many data rows the table viewport can display.
func (tl *TrackList) capacity() int {
	if c := tl.table.Height(); c > 0 {
		return c
	}
	return 1
}

// syncViewport clamps the cursor, recomputes the scroll offset so the cursor
// stays visible, and feeds the bubbles table only the visible window of rows.
// Because the window never exceeds the viewport height, the table never
// scrolls internally — so tl.offset is always the exact first visible row.
func (tl *TrackList) syncViewport() {
	n := len(tl.allRows)
	if n == 0 {
		tl.cursor = 0
		tl.offset = 0
		tl.table.SetRows(nil)
		return
	}
	tl.cursor = clampInt(tl.cursor, 0, n-1)

	visible := tl.capacity()
	// Scroll just enough to keep the cursor within the visible window.
	switch {
	case tl.cursor < tl.offset:
		tl.offset = tl.cursor
	case tl.cursor >= tl.offset+visible:
		tl.offset = tl.cursor - visible + 1
	}
	tl.offset = clampInt(tl.offset, 0, max(0, n-visible))

	end := min(tl.offset+visible, n)
	tl.table.SetRows(tl.allRows[tl.offset:end])
	tl.table.SetCursor(tl.cursor - tl.offset)
}

// MoveUp moves the selection up by n rows.
func (tl *TrackList) MoveUp(n int) {
	tl.cursor -= n
	tl.syncViewport()
}

// MoveDown moves the selection down by n rows.
func (tl *TrackList) MoveDown(n int) {
	tl.cursor += n
	tl.syncViewport()
}

// HalfPageUp moves the selection up by half a viewport.
func (tl *TrackList) HalfPageUp() { tl.MoveUp(max(1, tl.capacity()/2)) }

// HalfPageDown moves the selection down by half a viewport.
func (tl *TrackList) HalfPageDown() { tl.MoveDown(max(1, tl.capacity()/2)) }

// GotoTop moves the selection to the first row.
func (tl *TrackList) GotoTop() {
	tl.cursor = 0
	tl.syncViewport()
}

// GotoBottom moves the selection to the last row.
func (tl *TrackList) GotoBottom() {
	tl.cursor = len(tl.allRows) - 1
	tl.syncViewport()
}

// Cursor returns the selected index into the display tracks.
func (tl *TrackList) Cursor() int { return tl.cursor }

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
	tl.syncViewport()
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
	idx := tl.cursor
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
			tl.cursor = i
			tl.syncViewport()
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
	// Re-render rows with new widths (also re-windows via syncViewport).
	if len(tl.tracks) > 0 {
		tl.rebuildRows()
	} else {
		tl.syncViewport()
	}
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

// SetCursorFromClick maps a Y coordinate (relative to the pane top) to a
// data row and moves the cursor there. The clicked row is offset by the
// current scroll position, so clicks land correctly on a scrolled list.
// Returns true if the click landed on a real row.
func (tl *TrackList) SetCursorFromClick(y int) bool {
	row := y - tl.headerRows()
	if row < 0 || row >= tl.capacity() {
		return false
	}
	target := tl.offset + row
	if target >= len(tl.displayTracks()) {
		return false
	}
	tl.cursor = target
	tl.syncViewport()
	return true
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
	offset     int
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
		cursor:     tl.cursor,
		offset:     tl.offset,
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
	tl.table.SetHeight(tl.tableHeight())
	tl.cursor = s.cursor
	tl.offset = s.offset
	tl.rebuildRows()
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
