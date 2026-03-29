package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/visual"
)

const frameDepth = 3 // how many chars deep the edge bars go

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "starting..."
	}

	md := m.mood
	primary := lipgloss.Color(md.Primary)
	secondary := lipgloss.Color(md.Secondary)
	bg := lipgloss.Color(md.Background)
	bgStyle := lipgloss.NewStyle().Background(bg)

	// Help overlay
	if m.showHelp {
		helpStr := m.help.View(m.keys)
		helpStyled := lipgloss.NewStyle().
			Foreground(secondary).Background(bg).
			Padding(2, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.3))).
			Render(helpStr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpStyled,
			lipgloss.WithWhitespaceBackground(bg))
	}

	// Interpolate bars for all edges
	visibleBars := min(numBars, m.width)
	hBars := visual.InterpolateBars(m.bars[:visibleBars], m.width)       // horizontal (top/bottom)
	vBars := visual.InterpolateBars(m.bars[:visibleBars], m.height)      // vertical (left/right)

	// Pre-compute frame bar styles (gradient by depth)
	frameStyles := make([]lipgloss.Style, frameDepth)
	for d := range frameDepth {
		fade := 1.0 - float64(d)*0.3 // outer edge brightest, inner edge dimmest
		color := visual.LerpColor(md.Background, md.Primary, fade*0.7)
		frameStyles[d] = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg)
	}

	// Build content sections (centered, no frame)
	content := m.buildContent(md, primary, secondary, bg)
	contentLines := strings.Split(content, "\n")

	// Vertically center content
	contentH := len(contentLines)
	topPad := max(0, (m.height-contentH)/2)

	// Pad content to full height
	bgLine := bgStyle.Render(strings.Repeat(" ", max(0, m.width-frameDepth*2)))
	allContentLines := make([]string, m.height)
	for i := range m.height {
		ci := i - topPad
		if ci >= 0 && ci < contentH {
			allContentLines[i] = contentLines[ci]
		} else {
			allContentLines[i] = bgLine
		}
	}

	// Compose full screen: frame bars + content
	var full strings.Builder
	for row := range m.height {
		// Top/bottom edge bars
		isTopFrame := row < frameDepth
		isBottomFrame := row >= m.height-frameDepth

		if isTopFrame {
			// Top edge: bars hanging down
			full.WriteString(m.renderHorizontalFrame(hBars, row, frameDepth, true, frameStyles, bg))
		} else if isBottomFrame {
			// Bottom edge: bars growing up
			fromBottom := m.height - 1 - row
			full.WriteString(m.renderHorizontalFrame(hBars, fromBottom, frameDepth, false, frameStyles, bg))
		} else {
			// Content row with left/right edge bars
			leftEdge := m.renderVerticalEdge(vBars[row], frameDepth, true, frameStyles, bg)
			rightEdge := m.renderVerticalEdge(vBars[row], frameDepth, false, frameStyles, bg)

			// Pad content line to fill middle
			cl := allContentLines[row]
			clWidth := lipgloss.Width(cl)
			innerWidth := m.width - frameDepth*2
			if clWidth < innerWidth {
				cl += bgStyle.Render(strings.Repeat(" ", innerWidth-clWidth))
			}

			full.WriteString(leftEdge)
			full.WriteString(cl)
			full.WriteString(rightEdge)
		}
		if row < m.height-1 {
			full.WriteString("\n")
		}
	}

	return full.String()
}

// renderHorizontalFrame renders one row of the top or bottom frame bars.
func (m Model) renderHorizontalFrame(bars []float64, depthFromEdge, maxDepth int, inverted bool, styles []lipgloss.Style, bg lipgloss.Color) string {
	bgStyle := lipgloss.NewStyle().Background(bg)
	var sb strings.Builder

	for col := range m.width {
		h := bars[col]
		barDepth := h * float64(maxDepth)
		d := float64(depthFromEdge)

		styleIdx := min(depthFromEdge, maxDepth-1)
		style := styles[styleIdx]

		if d < barDepth-1 {
			sb.WriteString(style.Render("█"))
		} else if d < barDepth {
			frac := barDepth - math.Floor(barDepth)
			chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
			if inverted {
				chars = []string{"▔", "▀", "▀", "▀", "█", "█", "█", "█"}
			}
			idx := int(frac * float64(len(chars)-1))
			idx = max(0, min(idx, len(chars)-1))
			sb.WriteString(style.Render(chars[idx]))
		} else {
			sb.WriteString(bgStyle.Render(" "))
		}
	}
	return sb.String()
}

// renderVerticalEdge renders the left or right edge bars for a single row.
func (m Model) renderVerticalEdge(barVal float64, maxDepth int, isLeft bool, styles []lipgloss.Style, bg lipgloss.Color) string {
	bgStyle := lipgloss.NewStyle().Background(bg)
	barCols := barVal * float64(maxDepth)

	var sb strings.Builder
	for d := range maxDepth {
		col := d
		if !isLeft {
			col = maxDepth - 1 - d
		}

		styleIdx := col
		if !isLeft {
			styleIdx = d
		}
		styleIdx = min(styleIdx, maxDepth-1)
		style := styles[styleIdx]

		distFromEdge := float64(d)
		if !isLeft {
			distFromEdge = float64(d)
		}

		if distFromEdge < barCols-1 {
			sb.WriteString(style.Render("█"))
		} else if distFromEdge < barCols {
			if isLeft {
				sb.WriteString(style.Render("▐"))
			} else {
				sb.WriteString(style.Render("▌"))
			}
		} else {
			sb.WriteString(bgStyle.Render(" "))
		}
	}
	return sb.String()
}

// buildContent builds the centered content area (mood word, art, track info, progress).
func (m Model) buildContent(md mood.Mood, primary, secondary, bg lipgloss.Color) string {
	bgStyle := lipgloss.NewStyle().Background(bg)
	innerWidth := m.width - frameDepth*2

	var sections []string

	// Mood word
	moodColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.35))
	moodStyle := lipgloss.NewStyle().Foreground(moodColor).Background(bg)
	sections = append(sections, centerInner(moodStyle.Render(spacedWord(md.Name)), innerWidth, bgStyle))
	sections = append(sections, bgStyle.Render(strings.Repeat(" ", innerWidth)))

	// Album art
	if m.artworkRendered != "" {
		if m.artworkIsKitty {
			leftPad := max(0, (innerWidth-m.artworkCols)/2)
			padLine := bgStyle.Render(strings.Repeat(" ", leftPad)) + m.artworkRendered
			sections = append(sections, padLine)
			for range m.artworkRows {
				sections = append(sections, bgStyle.Render(strings.Repeat(" ", innerWidth)))
			}
		} else {
			for _, line := range strings.Split(m.artworkRendered, "\n") {
				sections = append(sections, centerInner(line, innerWidth, bgStyle))
			}
		}
		sections = append(sections, bgStyle.Render(strings.Repeat(" ", innerWidth)))
	}

	// Track info
	if m.track != nil {
		labelColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.4))
		labelStyle := lipgloss.NewStyle().Foreground(labelColor).Background(bg)
		trackStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		artistStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)

		sections = append(sections, centerInner(labelStyle.Render("N O W   P L A Y I N G"), innerWidth, bgStyle))
		sections = append(sections, centerInner(trackStyle.Render(m.track.Name), innerWidth, bgStyle))
		sections = append(sections, centerInner(artistStyle.Render(m.track.Artist), innerWidth, bgStyle))
	} else {
		titleStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		subStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
		sections = append(sections, centerInner(titleStyle.Render("♫  s p o t u i"), innerWidth, bgStyle))
		sections = append(sections, centerInner(subStyle.Render("waiting for music..."), innerWidth, bgStyle))
	}

	sections = append(sections, bgStyle.Render(strings.Repeat(" ", innerWidth)))

	// Progress
	if m.track != nil {
		progressWidth := min(innerWidth-20, 50)
		progressStr := m.renderProgress(progressWidth, primary, secondary)
		sections = append(sections, centerInner(progressStr, innerWidth, bgStyle))

		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controlStr := fmt.Sprintf("⏮      %s      ⏭", playPause)
		controlStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
		sections = append(sections, centerInner(controlStyle.Render(controlStr), innerWidth, bgStyle))
	}

	return strings.Join(sections, "\n")
}

func (m Model) renderProgress(width int, primary, secondary lipgloss.Color) string {
	if m.track == nil {
		return ""
	}
	pos := m.track.Position
	dur := m.track.Duration
	if dur == 0 {
		return ""
	}

	barWidth := width - 14
	if barWidth < 10 {
		barWidth = 10
	}
	ratio := float64(pos) / float64(dur)
	filled := int(float64(barWidth) * ratio)
	filled = max(0, min(filled, barWidth-1))

	var bar strings.Builder
	filledStyle := lipgloss.NewStyle().Foreground(primary)
	dotStyle := lipgloss.NewStyle().Foreground(primary).Bold(true)
	emptyStyle := lipgloss.NewStyle().Foreground(secondary)

	bar.WriteString(filledStyle.Render(strings.Repeat("━", filled)))
	bar.WriteString(dotStyle.Render("●"))
	remaining := barWidth - filled - 1
	if remaining > 0 {
		bar.WriteString(emptyStyle.Render(strings.Repeat("━", remaining)))
	}

	posStr := formatDuration(pos)
	durStr := formatDuration(dur)
	timeStyle := lipgloss.NewStyle().Foreground(secondary)

	return fmt.Sprintf("%s %s", bar.String(), timeStyle.Render(fmt.Sprintf("%s / %s", posStr, durStr)))
}

func centerInner(rendered string, innerWidth int, bgStyle lipgloss.Style) string {
	w := lipgloss.Width(rendered)
	if w >= innerWidth {
		return rendered
	}
	leftPad := (innerWidth - w) / 2
	rightPad := innerWidth - w - leftPad
	return bgStyle.Render(strings.Repeat(" ", leftPad)) + rendered + bgStyle.Render(strings.Repeat(" ", rightPad))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func spacedWord(s string) string {
	runes := []rune(s)
	spaced := make([]rune, 0, len(runes)*2-1)
	for i, r := range runes {
		if i > 0 {
			spaced = append(spaced, ' ')
		}
		spaced = append(spaced, r)
	}
	return string(spaced)
}
