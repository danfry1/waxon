package visual

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var barChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func RenderBars(heights []float64, maxHeight int, color string) string {
	if len(heights) == 0 { return strings.Repeat("\n", maxHeight) }
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	lines := make([]string, maxHeight)
	for row := range maxHeight {
		var sb strings.Builder
		for _, h := range heights {
			barH := h * float64(maxHeight)
			rowFromBottom := maxHeight - 1 - row
			if float64(rowFromBottom) < barH-1 {
				sb.WriteString(style.Render("█"))
			} else if float64(rowFromBottom) < barH {
				frac := barH - math.Floor(barH)
				idx := int(frac * float64(len(barChars)-1))
				idx = max(0, min(idx, len(barChars)-1))
				sb.WriteString(style.Render(barChars[idx]))
			} else {
				sb.WriteString(" ")
			}
			sb.WriteString(" ")
		}
		lines[row] = sb.String()
	}
	return strings.Join(lines, "\n")
}
