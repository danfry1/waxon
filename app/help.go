package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// helpCategory groups keybindings by purpose.
type helpCategory struct {
	Title    string
	Bindings []helpBinding
}

type helpBinding struct {
	Key  string
	Desc string
}

// keyLabel returns a short display label for a binding's primary key, using
// the same conventions as the help text ("Space", "C-u", "⌫").
func keyLabel(b key.Binding) string {
	if len(b.Keys()) == 0 {
		return ""
	}
	switch k := b.Keys()[0]; k {
	case " ":
		return "Space"
	case "enter":
		return "Enter"
	case "tab":
		return "Tab"
	case "esc":
		return "Esc"
	case "backspace":
		return "⌫"
	default:
		return strings.Replace(k, "ctrl+", "C-", 1)
	}
}

// hb builds a help row from a binding's primary key so the overlay can never
// drift from what is actually bound; desc is the (short) help text.
func hb(b key.Binding, desc string) helpBinding {
	return helpBinding{keyLabel(b), desc}
}

// pair merges two bindings into one "a / b" row.
func pair(a, b key.Binding, desc string) helpBinding {
	return helpBinding{keyLabel(a) + " / " + keyLabel(b), desc}
}

// helpCategories returns every help section, generated from the live KeyMap,
// the g-motion table, and the command table. Sections that aren't key.Bindings
// (g-prefix, Now Playing, mouse) are listed explicitly here.
func helpCategories(k KeyMap) []helpCategory {
	gotos := make([]helpBinding, 0, len(gMotions))
	for _, g := range gMotions {
		gotos = append(gotos, helpBinding{"g" + g.Key, g.Desc})
	}

	return []helpCategory{
		{
			Title: "NAVIGATION",
			Bindings: []helpBinding{
				pair(k.Down, k.Up, "down / up"),
				{"gg / " + keyLabel(k.Bottom), "top / bottom"},
				pair(k.HalfUp, k.HalfDown, "half page"),
				pair(k.FocusLeft, k.FocusRight, "focus left / right"),
				hb(k.CyclePane, "cycle pane"),
				{"5j / 12G / 6]", "count"},
				{keyLabel(k.Back) + " / b", "go back"},
			},
		},
		{Title: "GO-TO (g prefix)", Bindings: gotos},
		{
			Title: "PLAYBACK",
			Bindings: []helpBinding{
				hb(k.PlayPause, "play / pause"),
				hb(k.Enter, "play selected"),
				pair(k.Next, k.Prev, "next / previous"),
				pair(k.SeekBack, k.SeekFwd, "seek -5s / +5s"),
				pair(k.VolDown, k.VolUp, "volume -5 / +5"),
				hb(k.Mute, "mute / unmute"),
				hb(k.Shuffle, "toggle shuffle"),
				hb(k.Repeat, "repeat off/all/one"),
			},
		},
		{
			Title: "ACTIONS",
			Bindings: []helpBinding{
				hb(k.Actions, "actions (selection)"),
				hb(k.ActionsPlaying, "actions (playing track)"),
				hb(k.AddQueue, "add to queue"),
				hb(k.Like, "like / unlike selection"),
				hb(k.LikePlaying, "like / unlike playing"),
				hb(k.Filter, "filter view"),
				hb(k.Search, "search Spotify"),
				hb(k.Devices, "devices"),
				hb(k.Mode, "play in waxon ⇄ control the Spotify app"),
				hb(k.Command, "command mode"),
				hb(k.NowPlaying, "now playing"),
				hb(k.Help, "help"),
				hb(k.Quit, "quit"),
			},
		},
		{
			Title: "VISUAL",
			Bindings: []helpBinding{
				hb(k.Visual, "select a range"),
				{keyLabel(k.AddQueue) + " / " + keyLabel(k.Like), "queue / like all"},
				{keyLabel(k.Actions), "add all to playlist"},
				{"d", "remove from playlist"},
				{"Esc", "cancel"},
			},
		},
		{
			// These keys mean something else in the main view (l focuses
			// the track list), so say where they work.
			Title: "NOW PLAYING (" + keyLabel(k.NowPlaying) + " first)",
			Bindings: []helpBinding{
				{"l", "synced lyrics"},
				{"v", "vinyl mode"},
				{"f / a / o", "on playing track"},
				{"ga / gA", "its album / artist"},
				{"N / Esc", "close"},
			},
		},
		{Title: "COMMANDS", Bindings: commandHelp},
		{
			Title: "MOUSE",
			Bindings: []helpBinding{
				{"click", "select row"},
				{"2×click", "open / play"},
				{"wheel", "scroll pane"},
				{"click bar", "seek"},
			},
		},
	}
}

// essentials is the help's first page: the keys a newcomer needs, from the
// live KeyMap like the rest.
func essentials(k KeyMap) []helpCategory {
	return []helpCategory{
		{Title: "MOVE", Bindings: []helpBinding{
			pair(k.Down, k.Up, "down / up"),
			{"gg / " + keyLabel(k.Bottom), "top / bottom"},
			pair(k.FocusLeft, k.FocusRight, "left / right pane"),
			{keyLabel(k.Back) + " / b", "back"},
		}},
		{Title: "PLAY", Bindings: []helpBinding{
			hb(k.Enter, "play the selection"),
			hb(k.PlayPause, "play / pause"),
			pair(k.Next, k.Prev, "next / previous track"),
			pair(k.VolDown, k.VolUp, "volume down / up"),
		}},
		{Title: "FIND", Bindings: []helpBinding{
			hb(k.Search, "search Spotify"),
			hb(k.Filter, "filter this list"),
			hb(k.Command, "a command, like :vol 50"),
		}},
		{Title: "GO TO", Bindings: []helpBinding{
			{"gq", "the queue"},
			{"ga / gA", "album / artist of the selection"},
			{"gl", "the library"},
		}},
		{Title: "DO", Bindings: []helpBinding{
			hb(k.Actions, "actions for the selection"),
			hb(k.AddQueue, "add to the queue"),
			hb(k.Like, "like / unlike"),
			hb(k.NowPlaying, "Now Playing (l there: lyrics)"),
			hb(k.Devices, "devices"),
			hb(k.Mode, "play in waxon ⇄ the Spotify app"),
		}},
		{Title: "WAXON", Bindings: []helpBinding{
			hb(k.Help, "help: Tab for every key"),
			hb(k.Quit, "quit"),
		}},
	}
}

// helpPage is one page of the help overlay; cols is the most columns it
// spreads over.
type helpPage struct {
	name string
	cats []helpCategory
	cols int
}

// helpPages are the help overlay's pages: the essentials first, then every
// key, then every command.
func helpPages(k KeyMap) []helpPage {
	var keys, cmds []helpCategory
	for _, c := range helpCategories(k) {
		if c.Title == "COMMANDS" {
			cmds = append(cmds, c)
		} else {
			keys = append(keys, c)
		}
	}
	return []helpPage{
		{"Essentials", essentials(k), 2},
		{"All keys", keys, 3},
		{"Commands", cmds, 1},
	}
}

// helpGutter is a column's key gutter: its widest key, plus room.
func helpGutter(cats []helpCategory) int {
	w := 0
	for _, c := range cats {
		for _, b := range c.Bindings {
			w = max(w, lipgloss.Width(b.Key))
		}
	}
	return w + 2
}

// helpColWidth is the width a column needs to show every row in full.
func helpColWidth(cats []helpCategory) int {
	g, w := helpGutter(cats), 0
	for _, c := range cats {
		w = max(w, lipgloss.Width(c.Title))
		for _, b := range c.Bindings {
			w = max(w, 1+g+lipgloss.Width(b.Desc))
		}
	}
	return w
}

// splitColumns distributes categories across n columns in order, choosing
// the split whose tallest column is shortest, so the columns come out even.
func splitColumns(cats []helpCategory, n int) [][]helpCategory {
	n = max(1, min(n, len(cats)))
	heights := make([]int, len(cats))
	for i, c := range cats {
		heights[i] = len(c.Bindings) + 2 // title + blank
	}
	// best[i][j]: the shortest tallest column for cats[i:] in j columns.
	type cell struct{ tallest, cut int }
	best := make([][]cell, len(cats)+1)
	for i := range best {
		best[i] = make([]cell, n+1)
	}
	for i := len(cats) - 1; i >= 0; i-- {
		h := 0
		for _, x := range heights[i:] {
			h += x
		}
		best[i][1] = cell{h, len(cats)}
		for j := 2; j <= n; j++ {
			best[i][j] = cell{h, len(cats)}
			h := 0
			for cut := i + 1; cut < len(cats); cut++ {
				h += heights[cut-1]
				if t := max(h, best[cut][j-1].tallest); t < best[i][j].tallest {
					best[i][j] = cell{t, cut}
				}
			}
		}
	}
	cols := make([][]helpCategory, 0, n)
	for i, j := 0, n; i < len(cats); j-- {
		cut := best[i][j].cut
		cols = append(cols, cats[i:cut])
		i = cut
	}
	return cols
}

// ViewHelp renders the help overlay's first page as a centered floating
// panel.
func ViewHelp(width, height int) string {
	return viewHelp(DefaultKeyMap(), width, height, 0, 0, "")
}

// helpBody is a help page's columns as lines, and the overlay's width. It
// uses as many columns (up to the page's) as fit with every row in full,
// and truncates descriptions only when one column doesn't fit.
func helpBody(k KeyMap, width, page int) (lines []string, overlayW int) {
	const gap = 3
	pages := helpPages(k)
	pg := pages[max(0, min(page, len(pages)-1))]
	avail := max(16, width-8) // the overlay's border and padding

	var cols [][]helpCategory
	var widths []int
	for n := pg.cols; n >= 1; n-- {
		cols = splitColumns(pg.cats, n)
		widths = widths[:0]
		total := gap * (len(cols) - 1)
		for _, c := range cols {
			w := helpColWidth(c)
			widths = append(widths, w)
			total += w
		}
		if total <= avail || n == 1 {
			break
		}
	}
	if len(cols) == 1 {
		widths[0] = min(widths[0], avail)
	}

	catStyle := lipgloss.NewStyle().Foreground(ColorTextDim).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(ColorText)
	descStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

	renderCol := func(cats []helpCategory, cw int) string {
		g := helpGutter(cats)
		var sb strings.Builder
		for i, cat := range cats {
			sb.WriteString(catStyle.Render(cat.Title) + "\n")
			for _, b := range cat.Bindings {
				desc := b.Desc
				if room := cw - 1 - g; lipgloss.Width(desc) > room && room > 1 {
					desc = string([]rune(desc)[:room-1]) + "…"
				}
				pad := max(1, g-lipgloss.Width(b.Key))
				sb.WriteString(" " + keyStyle.Render(b.Key) + strings.Repeat(" ", pad) + descStyle.Render(desc) + "\n")
			}
			if i < len(cats)-1 {
				sb.WriteString("\n")
			}
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	rendered := make([]string, 0, len(cols)*2)
	overlayW = gap * (len(cols) - 1)
	for i, c := range cols {
		if i > 0 {
			rendered = append(rendered, strings.Repeat(" ", gap))
		}
		rendered = append(rendered, lipgloss.NewStyle().Width(widths[i]).Render(renderCol(c, widths[i])))
		overlayW += widths[i]
	}
	columns := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	return strings.Split(columns, "\n"), overlayW
}

// helpRows is how many body rows fit on screen: border (2), padding (2),
// the page tabs and the blank line under them, and the footer and the blank
// line above it.
func helpRows(height int) int { return max(1, height-8) }

// helpMaxScroll is the furthest a help page scrolls.
func helpMaxScroll(k KeyMap, width, height, page int) int {
	lines, _ := helpBody(k, width, page)
	return max(0, len(lines)-helpRows(height))
}

// viewHelp draws the help page over base (the screen it was opened from),
// or on its own backdrop when base is "". The panel is sized to the largest
// page, so moving between pages doesn't resize it.
func viewHelp(k KeyMap, width, height, scroll, page int, base string) string {
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorTextDim)
	lines, _ := helpBody(k, width, page)

	pages := helpPages(k)
	overlayW, tallest := 0, 0
	for p := range pages {
		l, w := helpBody(k, width, p)
		overlayW, tallest = max(overlayW, w), max(tallest, len(l))
	}
	for len(lines) < min(tallest, helpRows(height)) {
		lines = append(lines, "")
	}
	tabs := make([]string, len(pages))
	for i, p := range pages {
		if i == page {
			tabs[i] = accent.Render(p.name)
		} else {
			tabs[i] = dim.Render(p.name)
		}
	}
	header := strings.Join(tabs, dim.Render("  ·  "))
	footer := "Tab next page · ? / Esc / q to close"

	content := header + "\n\n"
	if rows := helpRows(height); len(lines) > rows {
		scroll = max(0, min(scroll, len(lines)-rows))
		content += strings.Join(lines[scroll:scroll+rows], "\n")
		more := "↓ j/k to scroll"
		if scroll == len(lines)-rows {
			more = "↑ j/k to scroll"
		}
		footer = more + " · " + footer
	} else {
		content += strings.Join(lines, "\n")
	}
	overlayW = max(overlayW, lipgloss.Width(header))
	if fw := lipgloss.Width(footer); fw > overlayW {
		footer = string([]rune(footer)[:max(1, overlayW-1)]) + "…"
	}
	content += "\n\n" + dim.Render(footer)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Width(overlayW+4).
		Padding(1, 2)

	overlay := border.Render(content)
	if base != "" {
		return overlayCentered(base, overlay, width, height)
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay,
		overlayBackdrop())
}

// overlayCentered draws box over the middle of base, replacing the cells
// under it and keeping the rest of each line.
func overlayCentered(base, box string, width, height int) string {
	lines := strings.Split(base, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	boxLines := strings.Split(box, "\n")
	boxW := lipgloss.Width(boxLines[0])
	top := max(0, (height-len(boxLines))/2)
	left := max(0, (width-boxW)/2)
	for i, bl := range boxLines {
		row := top + i
		if row >= len(lines) {
			break
		}
		l := ansi.Truncate(lines[row], left, "")
		if w := lipgloss.Width(l); w < left {
			l += strings.Repeat(" ", left-w)
		}
		lines[row] = l + "\033[0m" + bl + "\033[0m" + stripLeft(lines[row], left+boxW)
	}
	return strings.Join(lines, "\n")
}
