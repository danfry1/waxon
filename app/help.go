package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
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
				pair(k.Section1, k.Section2, "library / queue"),
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
			},
		},
		{
			Title: "ACTIONS",
			Bindings: []helpBinding{
				hb(k.Actions, "context actions"),
				hb(k.AddQueue, "add to queue"),
				hb(k.Like, "like / unlike"),
				hb(k.Filter, "filter view"),
				hb(k.Search, "search Spotify"),
				hb(k.Devices, "devices"),
				hb(k.Command, "command mode"),
				hb(k.NowPlaying, "now playing"),
				hb(k.Help, "help"),
				hb(k.Quit, "quit"),
			},
		},
		{
			Title: "NOW PLAYING",
			Bindings: []helpBinding{
				{"l", "synced lyrics"},
				{"V", "vinyl mode"},
				{"f / a / o", "on playing track"},
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
			},
		},
	}
}

// helpKeyColWidth is the key gutter width inside a help column.
const helpKeyColWidth = 11

// splitColumns distributes categories across n columns, filling each column
// greedily until it reaches roughly an even share of the total height. Order is
// preserved so related sections stay adjacent.
func splitColumns(cats []helpCategory, n int) [][]helpCategory {
	if n < 1 {
		n = 1
	}
	total := 0
	for _, c := range cats {
		total += len(c.Bindings) + 2 // title + blank
	}
	target := (total + n - 1) / n
	cols := make([][]helpCategory, 0, n)
	cur := make([]helpCategory, 0, len(cats))
	h := 0
	for _, c := range cats {
		ch := len(c.Bindings) + 2
		if h > 0 && h+ch > target && len(cols) < n-1 {
			cols = append(cols, cur)
			cur, h = make([]helpCategory, 0, len(cats)), 0
		}
		cur = append(cur, c)
		h += ch
	}
	if len(cur) > 0 {
		cols = append(cols, cur)
	}
	return cols
}

// ViewHelp renders the help overlay as a centered floating panel. Columns are
// chosen from the terminal width (3 when it fits, else 2), and content is
// clipped to the terminal height rather than overflowing it.
func ViewHelp(width, height int) string {
	return viewHelp(DefaultKeyMap(), width, height)
}

func viewHelp(k KeyMap, width, height int) string {
	const colW = 30    // preferred column width
	const colWMin = 24 // narrowest column before dropping to fewer columns
	const gap = 2
	ncols := 3
	if width < colWMin*3+gap*2+8 {
		ncols = 2
	}
	// Shrink columns on narrow terminals rather than overflowing.
	cw := colW
	if avail := (width - 8 - gap*(ncols-1)) / ncols; avail < cw {
		cw = max(16, avail)
	}
	overlayW := cw*ncols + gap*(ncols-1) + 4

	titleStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(ColorTextDim)
	catStyle := lipgloss.NewStyle().Foreground(ColorTextDim).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(ColorText)
	descStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

	renderCol := func(cats []helpCategory) string {
		var sb strings.Builder
		for i, cat := range cats {
			sb.WriteString(catStyle.Render(cat.Title) + "\n")
			for _, b := range cat.Bindings {
				key := keyStyle.Render(b.Key)
				pad := max(1, helpKeyColWidth-lipgloss.Width(b.Key))
				desc := b.Desc
				if room := cw - 1 - helpKeyColWidth; lipgloss.Width(desc) > room && room > 1 {
					desc = string([]rune(desc)[:room-1]) + "…"
				}
				sb.WriteString(" " + key + strings.Repeat(" ", pad) + descStyle.Render(desc) + "\n")
			}
			if i < len(cats)-1 {
				sb.WriteString("\n")
			}
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	cols := splitColumns(helpCategories(k), ncols)
	rendered := make([]string, 0, len(cols)*2)
	for i, c := range cols {
		if i > 0 {
			rendered = append(rendered, strings.Repeat(" ", gap))
		}
		rendered = append(rendered, lipgloss.NewStyle().Width(cw).Render(renderCol(c)))
	}
	columns := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	header := titleStyle.Render("Keybindings") + "  " + hintStyle.Render("? / Esc / q to close")
	content := header + "\n\n" + columns

	// Clip to the terminal: border (2) + padding (2) rows are reserved.
	maxLines := height - 4
	if lines := strings.Split(content, "\n"); maxLines > 2 && len(lines) > maxLines {
		content = strings.Join(lines[:maxLines-1], "\n") + "\n" + hintStyle.Render("… (enlarge the terminal to see everything)")
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Width(overlayW).
		Padding(1, 2)

	overlay := border.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#000000")))
}
