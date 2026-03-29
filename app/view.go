package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/visual"
)

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

	// Subtle ambient glow — steady, no fake pulsing
	var glowIntensity float64
	if m.track != nil && m.track.Playing {
		glowIntensity = 0.20
	} else {
		glowIntensity = 0.06
	}

	// Glow colors: outer edge brighter, inner dimmer
	outerGlow := visual.LerpColor(md.Background, md.Primary, glowIntensity)
	innerGlow := visual.LerpColor(md.Background, md.Primary, glowIntensity*0.4)
	outerStyle := lipgloss.NewStyle().Background(lipgloss.Color(outerGlow))
	innerStyle := lipgloss.NewStyle().Background(lipgloss.Color(innerGlow))

	// Build content
	content := m.buildContent(md, primary, secondary, bg)
	contentLines := strings.Split(content, "\n")
	contentH := len(contentLines)
	innerWidth := m.width - 4 // 2 glow chars per side

	// Vertically center content
	topPad := max(0, (m.height-contentH-4)/2) // -4 for top/bottom glow rows

	// Build full screen
	var full strings.Builder

	// Top glow edge (2 rows)
	full.WriteString(outerStyle.Render(strings.Repeat(" ", m.width)) + "\n")
	full.WriteString(innerStyle.Render(strings.Repeat(" ", m.width)) + "\n")

	// Content area with side glow
	totalContentRows := m.height - 4 // minus top/bottom glow
	for row := range totalContentRows {
		// Left glow
		full.WriteString(outerStyle.Render(" "))
		full.WriteString(innerStyle.Render(" "))

		// Content or empty
		ci := row - topPad
		if ci >= 0 && ci < contentH {
			line := contentLines[ci]
			lineW := lipgloss.Width(line)
			if lineW < innerWidth {
				line += bgStyle.Render(strings.Repeat(" ", innerWidth-lineW))
			}
			full.WriteString(line)
		} else {
			full.WriteString(bgStyle.Render(strings.Repeat(" ", innerWidth)))
		}

		// Right glow
		full.WriteString(innerStyle.Render(" "))
		full.WriteString(outerStyle.Render(" "))
		full.WriteString("\n")
	}

	// Bottom glow edge (2 rows)
	full.WriteString(innerStyle.Render(strings.Repeat(" ", m.width)) + "\n")
	full.WriteString(outerStyle.Render(strings.Repeat(" ", m.width)))

	return full.String()
}

func (m Model) buildContent(md mood.Mood, primary, secondary, bg lipgloss.Color) string {
	bgStyle := lipgloss.NewStyle().Background(bg)
	innerWidth := m.width - 4

	var sections []string
	emptyLine := bgStyle.Render(strings.Repeat(" ", innerWidth))

	// Mood word
	moodColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.35))
	moodStyle := lipgloss.NewStyle().Foreground(moodColor).Background(bg)
	sections = append(sections, centerInner(moodStyle.Render(spacedWord(md.Name)), innerWidth, bgStyle))
	sections = append(sections, emptyLine)

	// Album art
	if m.artworkRendered != "" {
		if m.artworkIsKitty {
			leftPad := max(0, (innerWidth-m.artworkCols)/2)
			padLine := bgStyle.Render(strings.Repeat(" ", leftPad)) + m.artworkRendered
			sections = append(sections, padLine)
			for range m.artworkRows {
				sections = append(sections, emptyLine)
			}
		} else {
			for _, line := range strings.Split(m.artworkRendered, "\n") {
				sections = append(sections, centerInner(line, innerWidth, bgStyle))
			}
		}
		sections = append(sections, emptyLine)
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

	sections = append(sections, emptyLine)

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
