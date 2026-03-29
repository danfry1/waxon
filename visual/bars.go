package visual

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var barChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// RenderBars renders bars at a fixed count, used for smaller layouts.
func RenderBars(heights []float64, maxHeight int, primary, secondary string) string {
	if len(heights) == 0 {
		return strings.Repeat("\n", maxHeight)
	}

	lines := make([]string, maxHeight)
	for row := range maxHeight {
		rowRatio := 1.0 - float64(row)/float64(maxHeight)
		rowColor := LerpColor(secondary, primary, rowRatio)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(rowColor))

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
		}
		lines[row] = sb.String()
	}
	return strings.Join(lines, "\n")
}

// RenderBarsFullWidth stretches model bars to fill the given width,
// with a color gradient (bright top, dim bottom) and a subtle reflection below.
func RenderBarsFullWidth(modelBars []float64, width, height int, primary, secondary, background string) string {
	if len(modelBars) == 0 || width == 0 {
		return strings.Repeat("\n", height)
	}

	reflectionH := height / 4
	mainH := height - reflectionH

	var lines []string

	// Main bars
	for row := range mainH {
		rowRatio := 1.0 - float64(row)/float64(mainH)
		rowColor := LerpColor(secondary, primary, 0.3+rowRatio*0.7)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(rowColor))

		var sb strings.Builder
		for col := range width {
			barIdx := col * len(modelBars) / width
			h := modelBars[barIdx]
			barH := h * float64(mainH)
			rowFromBottom := mainH - 1 - row

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
		}
		lines = append(lines, sb.String())
	}

	// Reflection (mirrored, fading out)
	for row := range reflectionH {
		// Map this reflection row to the corresponding main bar row (mirrored)
		mirrorRow := row
		fade := 1.0 - float64(row)/float64(reflectionH)
		rowColor := LerpColor(background, secondary, fade*0.3)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(rowColor))

		var sb strings.Builder
		for col := range width {
			barIdx := col * len(modelBars) / width
			h := modelBars[barIdx]
			barH := h * float64(mainH)

			// Mirror: bottom rows of the bar become top rows of reflection
			reflectH := barH * 0.4 // reflection is shorter than the bar
			rowFromTop := float64(mirrorRow)

			if rowFromTop < reflectH-1 {
				sb.WriteString(style.Render("▓"))
			} else if rowFromTop < reflectH {
				sb.WriteString(style.Render("░"))
			} else {
				sb.WriteString(" ")
			}
		}
		lines = append(lines, sb.String())
	}

	return strings.Join(lines, "\n")
}
