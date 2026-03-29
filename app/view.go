package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	dimColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.15))

	patternStyle := lipgloss.NewStyle().Foreground(dimColor).Background(bg)
	titleStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
	trackStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
	artistStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
	labelStyle := lipgloss.NewStyle().Foreground(primary).Background(bg)
	controlStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
	moodStyle := lipgloss.NewStyle().Foreground(dimColor).Background(bg)
	bgStyle := lipgloss.NewStyle().Background(bg)

	contentWidth := min(m.width-8, 60)
	pad := lipgloss.NewStyle().PaddingLeft(4).Background(bg)

	var content strings.Builder

	if m.track != nil {
		content.WriteString(pad.Render(labelStyle.Render("♫ NOW PLAYING")))
		content.WriteString("\n")
		content.WriteString(pad.Render(trackStyle.Render(m.track.Name)))
		content.WriteString("\n")
		content.WriteString(pad.Render(artistStyle.Render(m.track.Artist)))
	} else {
		content.WriteString(pad.Render(titleStyle.Render("♫ spotui")))
		content.WriteString("\n")
		content.WriteString(pad.Render(artistStyle.Render("waiting for music...")))
		content.WriteString("\n")
		content.WriteString(pad.Render(controlStyle.Render("play something on Spotify to begin")))
	}
	content.WriteString("\n\n")

	barHeights := m.bars[:]
	barsStr := visual.RenderBars(barHeights, barMaxH, md.Primary)
	for _, line := range strings.Split(barsStr, "\n") {
		content.WriteString(pad.Render(line))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	if m.track != nil {
		progress := m.renderProgress(contentWidth, primary, secondary)
		content.WriteString(pad.Render(progress))
		content.WriteString("\n")
		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controls := controlStyle.Render(fmt.Sprintf("⏮  %s  ⏭", playPause))
		content.WriteString(pad.Render(controls))
	}
	content.WriteString("\n")

	contentStr := content.String()
	contentLines := strings.Split(contentStr, "\n")
	contentH := len(contentLines)

	totalH := m.height
	topPatternH := max(0, (totalH-contentH)/2-1)
	bottomPatternH := max(0, totalH-contentH-topPatternH-2)

	topPatterns := visual.RenderPatternRows(md.PatternChar, m.width, topPatternH, m.pattern)
	bottomPatterns := visual.RenderPatternRows(md.PatternChar, m.width, bottomPatternH, m.pattern+topPatternH)

	var full strings.Builder
	for _, row := range topPatterns {
		full.WriteString(patternStyle.Render(row))
		full.WriteString("\n")
	}
	full.WriteString("\n")

	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < m.width {
			line += bgStyle.Render(strings.Repeat(" ", m.width-lineWidth))
		}
		full.WriteString(line)
		full.WriteString("\n")
	}

	for _, row := range bottomPatterns {
		full.WriteString(patternStyle.Render(row))
		full.WriteString("\n")
	}

	if bottomPatternH > 0 {
		moodWord := moodStyle.Render(md.Name)
		moodLine := strings.Repeat(" ", max(0, m.width-lipgloss.Width(moodWord)-4)) + moodWord
		full.WriteString(patternStyle.Render(moodLine))
	}

	if m.showHelp {
		helpStr := m.help.View(m.keys)
		helpStyled := lipgloss.NewStyle().Foreground(secondary).Background(bg).Padding(1, 2).Render(helpStr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpStyled,
			lipgloss.WithWhitespaceBackground(bg))
	}

	return full.String()
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
	filled := int(float64(barWidth) * (float64(pos) / float64(dur)))
	filled = max(0, min(filled, barWidth))
	empty := barWidth - filled
	bar := lipgloss.NewStyle().Foreground(primary).Render(strings.Repeat("━", filled)) +
		lipgloss.NewStyle().Foreground(secondary).Render(strings.Repeat("━", empty))
	posStr := formatDuration(pos)
	durStr := formatDuration(dur)
	return fmt.Sprintf("%s %s / %s", bar, posStr, durStr)
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
