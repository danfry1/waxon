package visual

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var barChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// BarChars is the exported version for use by the view compositor.
var BarChars = barChars

// RenderBars renders bars at a fixed count (for tests and small layouts).
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

// RenderBarsFullWidth smoothly interpolates model bars to fill the entire width,
// with vertical color gradient, sparkle particles, and a subtle reflection.
func RenderBarsFullWidth(modelBars []float64, width, height int, primary, secondary, background string) string {
	if len(modelBars) == 0 || width == 0 {
		return strings.Repeat("\n", height)
	}

	// Smoothly interpolate model bars to fill width
	smoothBars := InterpolateBars(modelBars, width)

	reflectionH := max(2, height/5)
	mainH := height - reflectionH

	// Pre-compute styled characters per row (vertical gradient)
	type rowCache struct {
		block    string
		partials [8]string
	}
	rowCaches := make([]rowCache, mainH)
	for row := range mainH {
		rowRatio := 1.0 - float64(row)/float64(mainH)
		rowColor := LerpColor(secondary, primary, 0.2+rowRatio*0.8)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(rowColor))
		rowCaches[row].block = style.Render("█")
		for i, c := range barChars {
			rowCaches[row].partials[i] = style.Render(c)
		}
	}

	var lines []string

	// Main bars
	for row := range mainH {
		cache := rowCaches[row]
		var sb strings.Builder
		for col := range width {
			h := smoothBars[col]
			barH := h * float64(mainH)
			rowFromBottom := mainH - 1 - row

			if float64(rowFromBottom) < barH-1 {
				sb.WriteString(cache.block)
			} else if float64(rowFromBottom) < barH {
				frac := barH - math.Floor(barH)
				idx := int(frac * float64(len(barChars)-1))
				idx = max(0, min(idx, len(barChars)-1))
				sb.WriteString(cache.partials[idx])
			} else {
				sb.WriteString(" ")
			}
		}
		lines = append(lines, sb.String())
	}

	// Reflection — subtle ▁ characters that fade out
	for row := range reflectionH {
		fade := 1.0 - float64(row)/float64(reflectionH)
		reflectColor := LerpColor(background, secondary, fade*0.15)
		reflectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(reflectColor))
		styledReflect := reflectStyle.Render("▁")

		var sb strings.Builder
		for col := range width {
			h := smoothBars[col]
			reflectH := h * float64(mainH) * 0.25 * fade
			if float64(row) < reflectH {
				sb.WriteString(styledReflect)
			} else {
				sb.WriteString(" ")
			}
		}
		lines = append(lines, sb.String())
	}

	return strings.Join(lines, "\n")
}

// RenderBarsWithGlow renders bars with background tinting on tall bars to create a bloom effect.
func RenderBarsWithGlow(heights []float64, maxHeight int, primary, secondary, background string) string {
	if len(heights) == 0 {
		return strings.Repeat("\n", maxHeight)
	}
	lines := make([]string, maxHeight)
	for row := range maxHeight {
		rowRatio := 1.0 - float64(row)/float64(maxHeight)
		rowColor := LerpColor(secondary, primary, 0.2+rowRatio*0.8)

		var sb strings.Builder
		for _, h := range heights {
			barH := h * float64(maxHeight)
			rowFromBottom := maxHeight - 1 - row

			if float64(rowFromBottom) < barH-1 {
				bgTint := background
				if h > 0.6 {
					glowT := (h - 0.6) / 0.4 * 0.15
					bgTint = LerpColor(background, rowColor, glowT)
				}
				glowStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(rowColor)).
					Background(lipgloss.Color(bgTint))
				sb.WriteString(glowStyle.Render("█"))
			} else if float64(rowFromBottom) < barH {
				frac := barH - math.Floor(barH)
				idx := int(frac * float64(len(barChars)-1))
				idx = max(0, min(idx, len(barChars)-1))
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(rowColor))
				sb.WriteString(style.Render(barChars[idx]))
			} else {
				glowBg := background
				if h > 0.5 && float64(rowFromBottom) < barH+3 {
					proximity := 1.0 - (float64(rowFromBottom)-barH)/3.0
					glowBg = LerpColor(background, secondary, proximity*0.05)
				}
				bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(glowBg))
				sb.WriteString(bgStyle.Render(" "))
			}
		}
		lines[row] = sb.String()
	}
	return strings.Join(lines, "\n")
}

// interpolateBars uses cosine interpolation to smoothly expand model bars
// into a higher-resolution output, creating fluid waveforms instead of blocky steps.
func InterpolateBars(modelBars []float64, outputWidth int) []float64 {
	if len(modelBars) == 0 || outputWidth == 0 {
		return make([]float64, outputWidth)
	}
	if len(modelBars) == 1 {
		result := make([]float64, outputWidth)
		for i := range result {
			result[i] = modelBars[0]
		}
		return result
	}

	result := make([]float64, outputWidth)
	for i := range outputWidth {
		pos := float64(i) * float64(len(modelBars)-1) / float64(outputWidth-1)
		lo := int(pos)
		hi := min(lo+1, len(modelBars)-1)
		frac := pos - float64(lo)

		// Cosine interpolation for smooth curves
		t := (1 - math.Cos(frac*math.Pi)) / 2
		result[i] = modelBars[lo]*(1-t) + modelBars[hi]*t
	}
	return result
}
