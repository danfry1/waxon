package app

import (
	"strings"

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

func helpColumns() ([]helpCategory, []helpCategory) {
	left := []helpCategory{
		{
			Title: "NAVIGATION",
			Bindings: []helpBinding{
				{"j / k", "up / down"},
				{"gg / G", "top / bottom"},
				{"C-u / C-d", "half page"},
				{"h / l", "left / right"},
				{"Tab", "cycle pane"},
				{"1 / 2", "library / queue"},
				{"b", "go back"},
			},
		},
		{
			Title: "GO-TO",
			Bindings: []helpBinding{
				{"gl", "library"},
				{"gq", "queue"},
				{"gc", "current track"},
				{"gr", "recently played"},
			},
		},
		{
			Title: "PLAYBACK",
			Bindings: []helpBinding{
				{"Space", "play / pause"},
				{"Enter", "play selected"},
				{"n / p", "next / prev"},
				{"[ / ]", "seek -5s / +5s"},
			},
		},
	}

	right := []helpCategory{
		{
			Title: "ACTIONS",
			Bindings: []helpBinding{
				{"o", "context actions"},
				{"a", "add to queue"},
				{"/", "filter view"},
				{"s", "search"},
				{"D", "devices"},
				{":", "command mode"},
				{"N", "now playing"},
				{"?", "help"},
				{"q", "quit"},
			},
		},
		{
			Title: "COMMANDS",
			Bindings: []helpBinding{
				{":vol N", "set volume 0-100"},
				{":shuffle", "toggle shuffle"},
				{":repeat X", "off / all / one"},
				{":device", "switch device"},
				{":search Q", "search Spotify"},
				{":recent", "recent tracks"},
				{":q", "quit"},
			},
		},
	}

	return left, right
}

// ViewHelp renders the help overlay as a centered floating panel.
func ViewHelp(width, height int) string {
	colW := min(34, (width-12)/2)
	overlayW := colW*2 + 8
	overlayH := min(height-4, 30)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)
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
				desc := descStyle.Render(b.Desc)
				// Pad key to 12 visible chars for alignment
				pad := 12 - lipgloss.Width(key)
				if pad < 1 {
					pad = 1
				}
				sb.WriteString(" " + key + strings.Repeat(" ", pad) + desc + "\n")
			}
			if i < len(cats)-1 {
				sb.WriteString("\n")
			}
		}
		return sb.String()
	}

	left, right := helpColumns()
	leftCol := lipgloss.NewStyle().Width(colW).Render(renderCol(left))
	rightCol := lipgloss.NewStyle().Width(colW).Render(renderCol(right))

	header := titleStyle.Render("Keybindings") + "  " + hintStyle.Render("? / Esc to close")
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)
	content := header + "\n\n" + columns

	overlay := border.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#000000")))
}
