package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	// want is the playlist (URI) the list is waiting for, so a slower
	// answer for one the user has since left can't replace it; loadingOther
	// while something that isn't a playlist loads, "" when nothing was asked.
	want string

	table table.Model
	// colTitles replaces the column headers for a view that isn't a track
	// list (an artist's releases, saved albums); nil is the default.
	colTitles []string
	// rowIcons are the covers of album, artist and show rows, by image URL,
	// drawn into their (otherwise empty) leading cells.
	rowIcons    map[string]string
	tracks      []source.Track
	filtered    []source.Track // subset visible after filter; nil means no filter active
	filteredIdx []int          // index into tracks for each filtered row
	filterText  string         // current active filter query
	searchKeys  []string       // lowercased "name\x00artist\x00album" per track, parallel to tracks
	allRows     []table.Row    // full row set; the table sees only a window of these
	cursor      int            // selected index into displayTracks (global, not windowed)
	offset      int            // data index of the first visible row
	title       string
	subtitle    string // optional second line (e.g. genres, artist name)
	headerInfo  string // optional third line (e.g. "30 tracks · 1h 42m")
	notice      string // shown in place of an empty table (e.g. why there are no tracks)
	contextURI  string // playlist/album URI for play context
	artBlock    string // rendered playlist/album art
	loading     bool   // true while fetching tracks
	loadingMore bool   // a further page is being fetched (set per render)
	spinFrame   int    // current spinner animation frame
	width       int
	height      int
	nowPlaying  string // ID of currently playing track
	visual      int    // visual-mode anchor (display index); -1 when not selecting
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
		visual: -1,
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

// loadingOther is TrackList.want while a view that isn't a playlist loads.
const loadingOther = "\x00other"

// SetLoadingFor shows the loading state for the playlist uri.
func (tl *TrackList) SetLoadingFor(title, uri string) {
	tl.SetLoading(title)
	tl.want = uri
}

// Expect records that the playlist uri was asked for (without showing the
// loading state, for refreshes of the list on screen).
func (tl *TrackList) Expect(uri string) { tl.want = uri }

// Wants reports whether a loaded playlist page for uri may replace the list.
func (tl *TrackList) Wants(uri string) bool { return tl.want == "" || tl.want == uri }

func (tl *TrackList) SetLoading(title string) {
	tl.want = loadingOther
	tl.loading = true
	tl.visual = -1
	tl.title = title
	tl.tracks = nil
	tl.searchKeys = nil
	tl.notice = ""
	// The last view's header lines don't belong to the one loading.
	tl.subtitle, tl.headerInfo = "", ""
	tl.table.SetHeight(tl.tableHeight())
	// A new view starts unfiltered; a filter left on would otherwise also
	// pull every page of the next playlist.
	tl.filterText = ""
	tl.filtered, tl.filteredIdx = nil, nil
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

// SetNotice sets a message shown instead of the (empty) track table.
func (tl *TrackList) SetNotice(s string) {
	tl.notice = s
}

// HasNotice reports whether the list is showing a notice instead of tracks.
func (tl *TrackList) HasNotice() bool {
	return tl.notice != "" && len(tl.tracks) == 0
}

func (tl *TrackList) SetHeaderInfo(info string) {
	tl.headerInfo = info
	tl.table.SetHeight(tl.tableHeight())
	tl.syncViewport()
}

func (tl *TrackList) SetTracks(tracks []source.Track, title, contextURI string) {
	tl.loading = false
	tl.tracks = tracks
	tl.searchKeys = buildSearchKeys(tracks)
	tl.title = title
	tl.visual = -1 // a reloaded list invalidates the selection
	tl.subtitle = ""
	tl.headerInfo = ""
	tl.notice = ""
	tl.contextURI = contextURI
	if tl.colTitles != nil {
		tl.colTitles = nil // a new view starts with the track headers
		tl.table.SetColumns(tl.columns())
	}

	// Re-apply active filter when tracks are refreshed
	if tl.filterText != "" {
		tl.applyFilter(tl.filterText)
	} else {
		tl.filtered, tl.filteredIdx = nil, nil
	}

	tl.rebuildRows()
}

// AppendTracks adds more tracks to the existing list without resetting
// the cursor position. Used for lazy-loading additional pages.
func (tl *TrackList) AppendTracks(tracks []source.Track) {
	tl.tracks = append(tl.tracks, tracks...)
	tl.searchKeys = append(tl.searchKeys, buildSearchKeys(tracks)...)
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
	heading := ""
	for i, t := range display {
		if t.IsSeparator && t.Name != "" {
			heading = t.Name
		}
		if t.IsSeparator {
			if t.Name == "" {
				// Blank spacer row for breathing room
				rows[i] = table.Row{" ", "", "", "", ""}
			} else {
				// Section divider: ─── ALBUMS ─────────────
				label := " " + strings.ToUpper(t.Name) + " "
				if t.Artist != "" {
					label = " " + strings.ToUpper(t.Name) + " · " + t.Artist + " "
				}
				colW := tl.columnWidth(2)
				rightPad := colW - 4 - ansi.StringWidth(label) // 4 for left dashes
				if rightPad < 3 {
					rightPad = 3
				}
				divider := strings.Repeat("─", 4) + label + strings.Repeat("─", rightPad)
				rows[i] = table.Row{" ", "", divider, "", ""}
			}
		} else if t.IsAlbumRow {
			// Album row — no prefix, album name, year in duration col
			rows[i] = table.Row{
				" ",
				"",
				truncate(t.Name, tl.columnWidth(2)),
				truncate(t.Album, tl.columnWidth(3)),
				t.Artist,
			}
		} else {
			trackNum++
			prefix := " "
			// In the queue only Now playing is marked, not queued copies of
			// the same track.
			if t.ID == tl.nowPlaying && (tl.contextURI != queueViewURI || heading == "Now playing") {
				prefix = "▶"
			}
			who := t.Artist
			if t.IsEpisode && tl.contextURI == "spotify:show:"+t.ShowID {
				who = t.Album // in the show's own list, the release date
			}
			rows[i] = table.Row{
				prefix,
				strconv.Itoa(trackNum),
				truncate(t.Name, tl.columnWidth(2)),
				truncate(who, tl.columnWidth(3)),
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
	window := tl.allRows[tl.offset:end]
	if tl.visual >= 0 {
		// Mark the selected range in the marker column. Only the visible
		// window is touched, so moving the selection is cheap on long lists.
		lo, hi := tl.VisualRange()
		marked := make([]table.Row, len(window))
		for i, row := range window {
			marked[i] = row
			if d := tl.offset + i; d >= lo && d <= hi && row[0] != "▶" && tl.selectable(d) {
				r := append(table.Row(nil), row...)
				r[0] = "┃"
				marked[i] = r
			}
		}
		window = marked
	}
	tl.table.SetRows(window)
	tl.table.SetCursor(tl.cursor - tl.offset)
}

// StartVisual anchors a visual selection at the cursor.
func (tl *TrackList) StartVisual() {
	tl.visual = tl.cursor
	tl.syncViewport()
}

// EndVisual drops the visual selection.
func (tl *TrackList) EndVisual() {
	tl.visual = -1
	tl.syncViewport()
}

// Visual reports whether a visual selection is active.
func (tl *TrackList) Visual() bool { return tl.visual >= 0 }

// VisualRange returns the selected display rows, inclusive.
func (tl *TrackList) VisualRange() (lo, hi int) {
	return min(tl.visual, tl.cursor), max(tl.visual, tl.cursor)
}

// selectable reports whether display row d is a playable track (not a
// separator or discography album row).
func (tl *TrackList) selectable(d int) bool {
	display := tl.displayTracks()
	return d >= 0 && d < len(display) && !display[d].IsSeparator && !display[d].IsAlbumRow
}

// VisualSelection returns the selected tracks in list order, and each one's
// index into the full track list (its playlist position, stable under a
// filter). With no visual selection it is just the cursor row.
func (tl *TrackList) VisualSelection() ([]source.Track, []int) {
	lo, hi := tl.cursor, tl.cursor
	if tl.visual >= 0 {
		lo, hi = tl.VisualRange()
	}
	display := tl.displayTracks()
	var tracks []source.Track
	var idx []int
	for d := lo; d <= hi && d < len(display); d++ {
		if !tl.selectable(d) {
			continue
		}
		tracks = append(tracks, display[d])
		if tl.filtered != nil {
			idx = append(idx, tl.filteredIdx[d])
		} else {
			idx = append(idx, d)
		}
	}
	return tracks, idx
}

// MoveUp moves the selection up by n rows.
func (tl *TrackList) MoveUp(n int) {
	tl.cursor -= n
	tl.skipSeparators(-1)
	tl.syncViewport()
}

// skipSeparators moves the cursor off section lines and blank rows, which
// there's nothing to do on: on in the direction it was moving (dir), or
// back the other way at the end of the list.
func (tl *TrackList) skipSeparators(dir int) {
	display := tl.displayTracks()
	n := len(display)
	if n == 0 {
		return
	}
	tl.cursor = clampInt(tl.cursor, 0, n-1)
	for _, d := range []int{dir, -dir} {
		for i := tl.cursor; i >= 0 && i < n; i += d {
			if !display[i].IsSeparator {
				tl.cursor = i
				return
			}
		}
	}
}

// MoveDown moves the selection down by n rows.
func (tl *TrackList) MoveDown(n int) {
	tl.cursor += n
	tl.skipSeparators(1)
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

// GotoRow moves the selection to display row i (clamped).
func (tl *TrackList) GotoRow(i int) {
	tl.cursor = i
	tl.syncViewport()
}

// GotoBottom moves the selection to the last row.
func (tl *TrackList) GotoBottom() {
	tl.cursor = len(tl.allRows) - 1
	tl.syncViewport()
}

// Cursor returns the selected index into the display tracks.
func (tl *TrackList) Cursor() int { return tl.cursor }

// SetFilter applies a case-insensitive filter matching track name, artist
// or album.
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
	var idx []int
	for i, t := range tl.tracks {
		if t.IsSeparator {
			continue
		}
		if strings.Contains(tl.searchKeys[i], q) {
			result = append(result, t)
			idx = append(idx, i)
		}
	}
	tl.filtered = result
	tl.filteredIdx = idx
}

// Filtering reports whether a filter query is active on the list.
func (tl *TrackList) Filtering() bool { return tl.filterText != "" }

// buildSearchKeys lowercases each track's filterable fields once, so typing a
// filter over a long playlist doesn't re-lowercase every track per keystroke.
// Fields are joined with NUL so a query can't match across a field boundary.
func buildSearchKeys(tracks []source.Track) []string {
	keys := make([]string, len(tracks))
	for i, t := range tracks {
		keys[i] = strings.ToLower(t.Name + "\x00" + t.Artist + "\x00" + t.Album)
	}
	return keys
}

// RemoveAt deletes the track at index i (into the full list) and keeps the
// cursor on the same row where possible.
func (tl *TrackList) RemoveAt(i int) {
	if i < 0 || i >= len(tl.tracks) {
		return
	}
	tl.tracks = append(tl.tracks[:i], tl.tracks[i+1:]...)
	tl.visual = -1
	tl.searchKeys = append(tl.searchKeys[:i], tl.searchKeys[i+1:]...)
	if tl.filterText != "" {
		tl.applyFilter(tl.filterText)
	}
	tl.rebuildRows()
}

// SelectedIndex returns the selected row's index into the full track list
// (its position in the playlist), or -1 when nothing is selected. Unlike the
// cursor, this is stable under an active filter.
func (tl *TrackList) SelectedIndex() int {
	if tl.cursor < 0 || tl.cursor >= len(tl.displayTracks()) {
		return -1
	}
	if tl.filtered != nil {
		if tl.cursor < len(tl.filteredIdx) {
			return tl.filteredIdx[tl.cursor]
		}
		return -1
	}
	return tl.cursor
}

// ClearFilter removes the active filter and restores the full track list.
func (tl *TrackList) ClearFilter() {
	tl.filterText = ""
	tl.filtered, tl.filteredIdx = nil, nil
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

// selectedOccurrence is the selected track's ID and which of the rows with
// that ID it is (0 for the first), so a refresh can find it again.
func (tl *TrackList) selectedOccurrence() (id string, nth int) {
	display := tl.displayTracks()
	if tl.cursor < 0 || tl.cursor >= len(display) || display[tl.cursor].IsSeparator {
		return "", 0
	}
	id = display[tl.cursor].ID
	for _, t := range display[:tl.cursor] {
		if !t.IsSeparator && t.ID == id {
			nth++
		}
	}
	return id, nth
}

// gotoOccurrence selects the nth row with track id; false when there's no
// such row.
func (tl *TrackList) gotoOccurrence(id string, nth int) bool {
	if id == "" {
		return false
	}
	for i, t := range tl.displayTracks() {
		if !t.IsSeparator && t.ID == id {
			if nth == 0 {
				tl.GotoRow(i)
				return true
			}
			nth--
		}
	}
	return false
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

// columns is trackColumns for this pane, with the view's own headers.
func (tl *TrackList) columns() []table.Column {
	cols := trackColumns(tl.width)
	for i, title := range tl.colTitles {
		if i < len(cols) {
			cols[i].Title = title
		}
	}
	return cols
}

// SetColumnTitles names the columns for the view being shown (until the
// next SetTracks); a title per column, marker first.
func (tl *TrackList) SetColumnTitles(titles ...string) {
	tl.colTitles = titles
	tl.table.SetColumns(tl.columns())
}

// SetRowIcon sets the cover drawn on rows whose image is url.
func (tl *TrackList) SetRowIcon(url, icon string) {
	if tl.rowIcons == nil {
		tl.rowIcons = map[string]string{}
	}
	tl.rowIcons[url] = icon
}

// RowIconURLs are the images of the rows shown that have no cover yet.
func (tl *TrackList) RowIconURLs() []string {
	var urls []string
	seen := map[string]bool{}
	for _, t := range tl.tracks {
		if u := t.ArtworkURL; t.IsAlbumRow && u != "" && !seen[u] && tl.rowIcons[u] == "" {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	return urls
}

// rowIconCol is where a row's cover starts: past the marker column, in the
// number column that album rows leave empty.
const rowIconCol = colMarkerW + tableCellPadding

// withRowIcons draws the visible rows' covers into the rendered table.
// They're laid in after rendering because the table measures cells without
// knowing about colour codes, and would cut a coloured cell short.
func (tl TrackList) withRowIcons(table string) string {
	if len(tl.rowIcons) == 0 || tl.columnWidth(1) == 0 {
		return table // no number column to draw them in (narrow pane)
	}
	lines := strings.Split(table, "\n")
	display := tl.displayTracks()
	first := len(lines) - tl.table.Height() // the rows area follows the header
	for i := range tl.table.Rows() {
		li, di := first+i, tl.offset+i
		if li < 0 || li >= len(lines) || di >= len(display) {
			continue
		}
		t := display[di]
		icon := tl.rowIcons[t.ArtworkURL]
		if !t.IsAlbumRow || icon == "" {
			continue
		}
		w := ansi.StringWidth(icon)
		lines[li] = ansi.Truncate(lines[li], rowIconCol, "") + icon + ansi.TruncateLeft(lines[li], rowIconCol+w, "")
	}
	return strings.Join(lines, "\n")
}

func (tl *TrackList) Resize(width, height int) {
	tl.width = width
	tl.height = height
	tl.table.SetHeight(tl.tableHeight())
	tl.table.SetColumns(tl.columns())
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
		// The header has exactly one row for this line (tableHeight counts on
		// it), so it must not wrap: drop the label, then fall back to a bare
		// spinner, as the pane narrows.
		avail := tl.width - 3 // border + leading space
		if tl.artBlock != "" {
			avail -= HeaderArtW + 1
		}
		info := tl.headerInfo
		if tl.loadingMore {
			spin := spinnerFrames[tl.spinFrame]
			switch full := info + " · " + spin + " loading more"; {
			case ansi.StringWidth(full) <= avail:
				info = full
			default:
				info += " " + spin
			}
		}
		header += "\n " + StyleDimText.Render(ansi.Truncate(info, max(0, avail), "…"))
	}

	// Show playlist art alongside title if available
	if tl.artBlock != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Center, tl.artBlock, " ", header)
	}

	var content string
	if tl.loading {
		content = header + "\n" + tl.skeletonRows()
	} else if tl.HasNotice() {
		content = header + "\n\n" + StyleDimText.PaddingLeft(2).Width(max(1, tl.width-4)).Render(tl.notice)
	} else {
		content = header + "\n" + tl.withRowIcons(tl.table.View())
	}

	return border.
		Width(tl.width - 2).
		Height(tl.height - 2).
		Render(content)
}

// skeletonRows draws placeholder rows shaped like the track table while the
// first page loads, so the pane holds its layout instead of jumping.
func (tl TrackList) skeletonRows() string {
	n := clampInt(tl.capacity(), 1, 8)
	cols := trackColumns(tl.width)
	dim := lipgloss.NewStyle().Foreground(ColorBorder)
	var b strings.Builder
	b.WriteString("\n")
	for i := range n {
		// Vary the bar lengths so the placeholder reads as rows of text.
		title := max(4, cols[2].Width*(5+(i*3)%4)/9)
		artist := max(3, cols[3].Width*(4+(i*5)%3)/8)
		line := strings.Repeat(" ", colMarkerW+cols[1].Width+tableCellPadding*2) +
			dim.Render(strings.Repeat("▁", title)) +
			strings.Repeat(" ", max(1, cols[2].Width-title+tableCellPadding))
		if cols[3].Width > 0 {
			line += dim.Render(strings.Repeat("▁", artist))
		}
		b.WriteString(ansi.Truncate(line, max(0, tl.width-4), "") + "\n")
	}
	return b.String()
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
	notice     string
	contextURI string
	want       string // what the list was waiting for (TrackList.want)
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
		notice:     tl.notice,
		contextURI: tl.contextURI,
		want:       tl.want,
		artBlock:   tl.artBlock,
		cursor:     tl.cursor,
		offset:     tl.offset,
		focusPane:  pane,
	}
}

// RestoreState restores the tracklist from a previously saved NavState.
func (tl *TrackList) RestoreState(s NavState) {
	tl.loading = false
	tl.visual = -1
	tl.tracks = s.tracks
	tl.searchKeys = buildSearchKeys(s.tracks)
	tl.title = s.title
	tl.subtitle = s.subtitle
	tl.headerInfo = s.headerInfo
	tl.notice = s.notice
	tl.contextURI = s.contextURI
	// Back on this view, it waits for what it waited for before, so a
	// slower page for the one opened meanwhile can't replace it (and the
	// empty start view still takes the startup load).
	tl.want = s.want
	tl.artBlock = s.artBlock
	tl.filtered, tl.filteredIdx = nil, nil
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
	if allEpisodes(tracks) {
		trackWord = pluralize(count, "episode", "episodes")
	}
	if h > 0 {
		return fmt.Sprintf("%d %s · %dh %dm", count, trackWord, h, m)
	}
	return fmt.Sprintf("%d %s · %dm", count, trackWord, m)
}

// partialListInfo is FormatPartialTrackListInfo with the right noun for the
// loaded rows (a show's list counts episodes).
func partialListInfo(tracks []source.Track, loaded, total int) string {
	if allEpisodes(tracks) {
		return fmt.Sprintf("%d of %d episodes", loaded, total)
	}
	return FormatPartialTrackListInfo(loaded, total)
}

// allEpisodes reports whether every playable row is a podcast episode.
func allEpisodes(tracks []source.Track) bool {
	n := 0
	for _, t := range tracks {
		if t.IsSeparator || t.IsAlbumRow {
			continue
		}
		if !t.IsEpisode {
			return false
		}
		n++
	}
	return n > 0
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
// formatArtistInfo summarises an artist page: its top tracks when Spotify
// provides them, otherwise the discography ("12 albums · 8 singles").
func formatArtistInfo(page *source.ArtistPage) string {
	if len(page.Tracks) > 0 {
		return FormatTrackListInfo(page.Tracks)
	}
	var albums, singles int
	for _, a := range page.Albums {
		if a.Type == "Album" {
			albums++
		} else {
			singles++
		}
	}
	var parts []string
	if albums > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", albums, pluralize(albums, "album", "albums")))
	}
	if singles > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", singles, pluralize(singles, "single", "singles")))
	}
	return strings.Join(parts, " · ")
}

// The Play and Shuffle rows at the top of an artist page.
const (
	playArtistRowID    = "artist:play"
	shuffleArtistRowID = "artist:shuffle"
)

func buildArtistTrackList(page *source.ArtistPage) []source.Track {
	var tracks []source.Track
	if page.ID != "" {
		// Spotify won't list an artist's top tracks to most apps, so a page
		// could have nothing to play: these play the artist on Spotify.
		uri := "spotify:artist:" + page.ID
		tracks = append(tracks,
			source.Track{ID: playArtistRowID, URI: uri, Name: "▶  Play " + page.Name, IsAlbumRow: true},
			source.Track{ID: shuffleArtistRowID, URI: uri, Name: "⇄  Shuffle", IsAlbumRow: true},
		)
	}
	tracks = append(tracks, page.Tracks...)

	var albums, singles []source.ArtistAlbum
	for _, a := range page.Albums {
		if a.Type == "Album" {
			albums = append(albums, a)
		} else {
			singles = append(singles, a)
		}
	}

	// A blank row separates sections, but not above the first one.
	gap := func() {
		if len(tracks) > 0 {
			tracks = append(tracks, source.Track{IsSeparator: true})
		}
	}

	if len(albums) > 0 {
		gap()
		tracks = append(tracks, source.Track{
			Name:        "Albums",
			Artist:      strconv.Itoa(len(albums)),
			IsSeparator: true,
		})
		for _, album := range albums {
			tracks = append(tracks, source.Track{
				Name:       album.Name,
				Artist:     album.Year,
				AlbumID:    album.ID,
				ArtworkURL: album.ImageURL,
				IsAlbumRow: true,
			})
		}
	}

	if len(singles) > 0 {
		gap()
		tracks = append(tracks, source.Track{
			Name:        "Singles & EPs",
			Artist:      strconv.Itoa(len(singles)),
			IsSeparator: true,
		})
		for _, single := range singles {
			tracks = append(tracks, source.Track{
				Name:       single.Name,
				Artist:     single.Year,
				AlbumID:    single.ID,
				ArtworkURL: single.ImageURL,
				IsAlbumRow: true,
			})
		}
	}

	return tracks
}
